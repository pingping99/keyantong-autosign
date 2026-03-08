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
	// Load global configuration (priority: ENV > config.yml > defaults)
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Ensure data directory exists
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		log.Fatalf("Failed to create data directory %q: %v", cfg.DataDir, err)
	}

	// Setup logging
	logFilePath := filepath.Join(cfg.DataDir, "sign.log")
	rotateLogFile(logFilePath)
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o664)
	if err != nil {
		log.Fatalf("Failed to open log file %q: %v", logFilePath, err)
	}
	defer logFile.Close()
	log.SetOutput(io.MultiWriter(os.Stdout, logFile))
	log.SetFlags(log.LstdFlags)

	// Setup file-only logger (for quiet logs outside sign window)
	fileLogger := log.New(logFile, "", log.LstdFlags)

	// Load accounts (priority: ENV > config.yml > data/accounts.json)
	accounts, err := config.LoadAccounts(cfg.DataDir)
	if err != nil {
		log.Fatalf("Failed to load accounts: %v", err)
	}

	log.Printf("加载 %d 个账户", len(accounts))
	log.Printf("检查间隔 %s，重试间隔 %s，时区 %s",
		cfg.CheckInterval, cfg.RetryInterval, cfg.Location)

	// Create store factory for per-account state management
	storeFactory := core.NewFileStoreFactory(cfg.DataDir)

	// Create signers for each account
	var signers []core.Signer
	for i, acc := range accounts {
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

	// Force sign on startup if configured
	if cfg.ForceSignOnStart {
		log.Printf("程序启动，立即执行登录并签到（无视时间窗口）")
		now := time.Now().In(cfg.Location)
		for i, s := range signers {
			if err := s.ForceSign(now); err != nil {
				log.Printf("账户 %d 启动签到失败: %v", i+1, err)
			}
		}
	} else {
		log.Printf("程序启动，已禁用强制签到，等待窗口内自动签到")
	}

	// Run initial check
	runChecks(signers)

	// Setup graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Start periodic checks
	ticker := time.NewTicker(cfg.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("收到退出信号，正在优雅关闭...")
			return
		case <-ticker.C:
			runChecks(signers)
		}
	}
}

// runChecks executes sign attempt for all signers.
func runChecks(signers []core.Signer) {
	now := time.Now()
	for i, s := range signers {
		if err := s.AttemptSign(now); err != nil {
			log.Printf("账户 %d 签到失败: %v", i+1, err)
		}
	}
}

// rotateLogFile renames the log file if it exceeds maxLogSize.
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
