package main

import (
	"context"
	"io"
	"keyantong/config"
	"keyantong/scheduler"
	"keyantong/service"
	"keyantong/signer"
	"keyantong/store"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

const maxLogSize = 5 * 1024 * 1024 // 5MB

// fileLogger is used for logging only to file (not to stdout)
var fileLogger *log.Logger

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
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
	fileLogger = log.New(logFile, "", log.LstdFlags)

	// Initialize state store
	stateStore, err := store.NewFileStore(cfg.DataDir)
	if err != nil {
		log.Fatalf("Failed to create state store: %v", err)
	}

	// Initialize service
	svc, err := service.NewService(cfg.Email, cfg.Password)
	if err != nil {
		log.Fatalf("Failed to initialize service: %v", err)
	}

	// Build signer
	s := signer.NewAccountSigner(svc, stateStore, cfg, fileLogger)

	log.Printf("智能签到范围 %s-%s，检查间隔 %s，重试间隔 %s，时区 %s",
		scheduler.FormatWindow(cfg.DynamicWindowStart), scheduler.FormatWindow(cfg.DynamicWindowEnd),
		cfg.CheckInterval, cfg.RetryInterval, cfg.Location)

	// Force sign on startup if configured
	forceSignDone := false
	if cfg.ForceSignOnStart {
		log.Printf("程序启动，立即执行登录并签到（无视时间窗口）")
		now := time.Now()
		if err := s.ForceSign(now); err != nil {
			log.Printf("启动签到失败: %v", err)
		} else {
			forceSignDone = true
		}
	} else {
		log.Printf("程序启动，已禁用强制签到，等待窗口内自动签到")
	}

	// Run initial check only if force sign was not successful
	if !forceSignDone {
		runCheck(s)
	}

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
			runCheck(s)
		}
	}
}

// runCheck executes sign attempt for the signer.
func runCheck(s signer.Signer) {
	now := time.Now()
	if err := s.AttemptSign(now); err != nil {
		log.Printf("签到失败: %v", err)
	}
}

// rotateLogFile renames the log file if it exceeds maxLogSize.
func rotateLogFile(path string) {
	info, err := os.Stat(path)
	if err != nil {
		return // File doesn't exist yet, nothing to rotate
	}
	if info.Size() < maxLogSize {
		return
	}
	backupPath := path + ".old"
	// Remove previous backup if exists
	os.Remove(backupPath)
	if err := os.Rename(path, backupPath); err != nil {
		log.Printf("日志轮转失败: %v", err)
	}
}
