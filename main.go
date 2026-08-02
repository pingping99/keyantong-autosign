package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"keyantong/config"
	"keyantong/core"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

const maxLogSize = 5 * 1024 * 1024

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		log.Fatalf("创建数据目录失败: %v", err)
	}

	logFilePath := filepath.Join(cfg.DataDir, "sign.log")
	rotateLogFile(logFilePath)
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		log.Fatalf("打开日志文件失败: %v", err)
	}
	defer logFile.Close()
	log.SetOutput(io.MultiWriter(os.Stdout, logFile))
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	fileLogger := log.New(logFile, "", log.LstdFlags)

	if err := core.MigrateSingleAccountState(cfg.DataDir, cfg.Email); err != nil {
		log.Fatalf("迁移旧状态失败: %v", err)
	}
	store, err := core.NewFileStore(cfg.DataDir)
	if err != nil {
		log.Fatalf("初始化状态存储失败: %v", err)
	}
	service, err := core.NewService(
		cfg.Email,
		cfg.Password,
		cfg.APIBaseURL,
		cfg.APILoginPath,
		cfg.APISignPath,
	)
	if err != nil {
		log.Fatalf("初始化签到服务失败: %v", err)
	}
	signer := core.NewAccountSigner(service, store, cfg, fileLogger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	healthServer := startHealthServer(cfg.HealthCheckHost, cfg.HealthCheckPort)
	defer shutdownHealthServer(healthServer)

	log.Printf("单账户签到服务已启动，账户: %s，时区: %s", maskEmail(cfg.Email), cfg.Location)
	if cfg.ForceSignOnStart {
		if err := signer.SignOnStartup(ctx, time.Now()); err != nil && !core.IsAlreadySignedError(err) {
			core.Notify(fmt.Sprintf("启动签到失败: %v", err))
		}
	} else {
		runCheck(ctx, signer, cfg)
	}
	smartScheduleLoop(ctx, signer, cfg)
	log.Printf("收到退出信号，正在关闭")
}

func smartScheduleLoop(ctx context.Context, signer core.Signer, cfg *config.AppConfig) {
	for {
		waitDuration := calculateNextCheckInterval(cfg)
		log.Printf("下次检查将在 %v 后", waitDuration.Round(time.Second))
		timer := time.NewTimer(waitDuration)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			runCheck(ctx, signer, cfg)
		}
	}
}

func calculateNextCheckInterval(cfg *config.AppConfig) time.Duration {
	now := time.Now().In(cfg.Location)
	if core.IsWithinHourRange(now, cfg.Location, cfg.EarlyHourThreshold, cfg.LateHourThreshold) {
		return cfg.CheckInterval
	}
	return core.TimeUntilNextWorkingTime(now, cfg.Location, cfg.EarlyHourThreshold) +
		core.CalculateJitter(5*time.Minute)
}

func runCheck(ctx context.Context, signer core.Signer, cfg *config.AppConfig) {
	now := time.Now().In(cfg.Location)
	if !core.IsWithinHourRange(now, cfg.Location, cfg.EarlyHourThreshold, cfg.LateHourThreshold) {
		return
	}
	if err := signer.AttemptSign(ctx, now); err != nil && !core.IsAlreadySignedError(err) {
		core.Notify(fmt.Sprintf("签到失败: %v", err))
	}
}

func startHealthServer(host string, port int) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(writer http.ResponseWriter, _ *http.Request) {
		info := core.GetHealth()
		writer.Header().Set("Content-Type", "application/json")
		if info.Status == "failed" {
			writer.WriteHeader(http.StatusServiceUnavailable)
		} else {
			writer.WriteHeader(http.StatusOK)
		}
		if err := json.NewEncoder(writer).Encode(info); err != nil {
			log.Printf("写入健康检查响应失败: %v", err)
		}
	})

	address := net.JoinHostPort(host, strconv.Itoa(port))
	server := &http.Server{
		Addr:              address,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       30 * time.Second,
	}
	go func() {
		log.Printf("健康检查监听 %s", address)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("健康检查服务异常: %v", err)
		}
	}()
	return server
}

func shutdownHealthServer(server *http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("关闭健康检查服务失败: %v", err)
	}
}

func rotateLogFile(path string) {
	info, err := os.Stat(path)
	if err != nil || info.Size() < maxLogSize {
		return
	}
	backupPath := path + ".old"
	_ = os.Remove(backupPath)
	if err := os.Rename(path, backupPath); err != nil {
		log.Printf("日志轮转失败: %v", err)
	}
}

func maskEmail(email string) string {
	for index, character := range email {
		if character == '@' {
			if index <= 2 {
				return "***" + email[index:]
			}
			return email[:2] + "***" + email[index:]
		}
	}
	return "***"
}
