package main

import (
	"context"
	"io"
	"keyantong/config"
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

func main() {
	// Load global configuration
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
	fileLogger := log.New(logFile, "", log.LstdFlags)

	// Load accounts from file or environment variables
	accounts, err := config.LoadAccounts()
	if err != nil {
		log.Fatalf("Failed to load accounts: %v", err)
	}

	log.Printf("加载 %d 个账户", len(accounts))
	log.Printf("检查间隔 %s，重试间隔 %s，时区 %s",
		cfg.CheckInterval, cfg.RetryInterval, cfg.Location)

	// Create store factory for per-account state management
	storeFactory := store.NewFileStoreFactory(cfg.DataDir)

	// Create signers for each account
	var signers []signer.Signer
	for i, acc := range accounts {
		// Initialize service with configured API endpoints
		svc, err := service.NewService(
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

		// Generate unique account ID (hash of email) for consistent state file naming
		accountID := store.GenerateAccountID(acc.Email)

		// Create account-specific state store
		stateStore := storeFactory.CreateStore(accountID)

		// Build signer
		s := signer.NewAccountSigner(svc, stateStore, cfg, fileLogger)
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
func runChecks(signers []signer.Signer) {
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
