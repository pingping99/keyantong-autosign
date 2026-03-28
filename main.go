package main

import (
	"context"
	"io"
	"keyantong/config"
	"keyantong/core"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

const maxLogSize = 5 * 1024 * 1024 // 5MB

func main() {
	// 加载全局配置（优先级: ENV > config.yml > 默认值）
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 确保数据目录存在
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		log.Fatalf("创建数据目录 %q 失败: %v", cfg.DataDir, err)
	}

	// 设置日志
	logFilePath := filepath.Join(cfg.DataDir, "sign.log")
	rotateLogFile(logFilePath)
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o664)
	if err != nil {
		log.Fatalf("打开日志文件 %q 失败: %v", logFilePath, err)
	}
	defer logFile.Close()
	log.SetOutput(io.MultiWriter(os.Stdout, logFile))
	log.SetFlags(log.LstdFlags)

	// 创建文件日志器（用于非工作时间的静默日志）
	fileLogger := log.New(logFile, "", log.LstdFlags)

	// 加载账户（优先级: ENV > config.yml > data/accounts.json）
	accounts, err := config.LoadAccounts(cfg.DataDir)
	if err != nil {
		log.Fatalf("加载账户失败: %v", err)
	}

	log.Printf("加载 %d 个账户", len(accounts))
	log.Printf("检查间隔 %s，重试间隔 %s，时区 %s",
		cfg.CheckInterval, cfg.RetryInterval, cfg.Location)

	// 创建状态存储工厂
	storeFactory := core.NewFileStoreFactory(cfg.DataDir)

	// 为每个账户创建签到器
	var signers []core.Signer
	for i, acc := range accounts {
		// 迁移旧版状态文件（如果存在）
		if err := core.MigrateAccountState(cfg.DataDir, acc.Email); err != nil {
			log.Printf("账户 %d 状态迁移失败（可忽略）: %v", i+1, err)
		}

		svc, err := core.NewService(
			acc.Email,
			acc.Password,
			cfg.APIBaseURL,
			cfg.APILoginPath,
			cfg.APISignPath,
		)
		if err != nil {
			log.Printf("账户 %d (%s) 初始化失败: %v", i+1, acc.Email, err)
			continue
		}

		accountID := core.GenerateAccountID(acc.Email)
		stateStore := storeFactory.CreateStore(accountID)

		s := core.NewAccountSigner(svc, stateStore, cfg, fileLogger)
		signers = append(signers, s)
		log.Printf("✓ 账户 %d 已加载: %s", i+1, acc.Email)
	}

	if len(signers) == 0 {
		log.Fatalf("没有可用的账户")
	}

	// 设置优雅关闭
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// 启动时签到（如果配置启用）
	if cfg.ForceSignOnStart {
		log.Printf("程序启动，执行启动签到")
		now := time.Now().In(cfg.Location)
		for i, s := range signers {
			if err := s.SignOnStartup(ctx, now); err != nil {
				log.Printf("账户 %d 启动签到失败: %v", i+1, err)
			}
		}
	} else {
		log.Printf("程序启动，已禁用启动签到，等待窗口内自动签到")
	}

	// 运行首次检查
	runChecks(ctx, signers, cfg)

	// 启动智能调度循环
	smartScheduleLoop(ctx, signers, cfg)

	log.Printf("收到退出信号，正在优雅关闭...")
}

// smartScheduleLoop 智能调度循环
// 工作时间内使用 CheckInterval，工作时间外计算到下一个工作时间的等待
func smartScheduleLoop(ctx context.Context, signers []core.Signer, cfg *config.AppConfig) {
	for {
		// 计算下次检查的等待时间
		waitDuration := calculateNextCheckInterval(cfg)

		log.Printf("下次检查将在 %v 后", waitDuration.Round(time.Second))

		// 等待，支持取消
		timer := time.NewTimer(waitDuration)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			runChecks(ctx, signers, cfg)
		}
	}
}

// calculateNextCheckInterval 计算下次检查的等待时间
func calculateNextCheckInterval(cfg *config.AppConfig) time.Duration {
	now := time.Now().In(cfg.Location)
	currentHour := now.Hour()

	// 在工作时间内，使用正常检查间隔
	if currentHour >= cfg.EarlyHourThreshold && currentHour < cfg.LateHourThreshold {
		return cfg.CheckInterval
	}

	// 工作时间外，计算到下一个工作时间开始的等待
	waitUntilWorkStart := core.TimeUntilNextWorkingTime(now, cfg.Location, cfg.EarlyHourThreshold)

	// 添加一点随机抖动（1-5分钟）避免精确整点
	jitter := core.CalculateJitter(300) // 最多5分钟
	if jitter < time.Minute {
		jitter = time.Minute // 最少1分钟
	}

	return waitUntilWorkStart + jitter
}

// runChecks 对所有账户执行签到尝试
func runChecks(ctx context.Context, signers []core.Signer, cfg *config.AppConfig) {
	now := time.Now().In(cfg.Location)

	// 检查是否在工作时间内
	if !core.IsWithinHourRange(now, cfg.Location, cfg.EarlyHourThreshold, cfg.LateHourThreshold) {
		// 工作时间外，静默跳过
		return
	}

	for i, s := range signers {
		// 检查 context 是否已取消
		select {
		case <-ctx.Done():
			return
		default:
		}

		if err := s.AttemptSign(ctx, now); err != nil {
			// 只记录非预期错误（已签到不算错误）
			if !core.IsAlreadySignedError(err) {
				log.Printf("账户 %d 签到失败: %v", i+1, err)
			}
		}
	}
}

// rotateLogFile 如果日志文件超过 maxLogSize 则轮转
func rotateLogFile(path string) {
	info, err := os.Stat(path)
	if err != nil {
		return
	}
	if info.Size() < maxLogSize {
		return
	}
	backupPath := path + ".old"
	os.Remove(backupPath)
	if err := os.Rename(path, backupPath); err != nil {
		log.Printf("日志轮转失败: %v", err)
	}
}
