package main

import (
	"io"
	"keyantong/config"
	"keyantong/scheduler"
	"keyantong/service"
	"keyantong/signer"
	"keyantong/store"
	"log"
	"os"
	"path/filepath"
	"time"
)

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

	// Build signers for each account
	signers := buildSigners(cfg, stateStore)
	if len(signers) == 0 {
		log.Fatal("No valid accounts available, exiting")
	}

	log.Printf("动态签到范围 %s-%s，窗口时长 %s，检查间隔 %s，重试间隔 %s，时区 %s，账号数量 %d",
		scheduler.FormatWindow(cfg.DynamicWindowStart), scheduler.FormatWindow(cfg.DynamicWindowEnd),
		cfg.DynamicWindowSpan, cfg.CheckInterval, cfg.RetryInterval, cfg.Location, len(signers))

	// Force sign on startup if configured
	if cfg.ForceSignOnStart {
		log.Printf("程序启动，立即执行登录并签到（无视时间窗口）")
		now := time.Now()
		for _, s := range signers {
			if err := s.ForceSign(now); err != nil {
				log.Printf("启动签到失败: %v", err)
			}
		}
	} else {
		log.Printf("程序启动，已禁用强制签到，等待窗口内自动签到")
	}

	// Run initial checks
	runChecks(signers)

	// Start periodic checks
	ticker := time.NewTicker(cfg.CheckInterval)
	defer ticker.Stop()

	for range ticker.C {
		runChecks(signers)
	}
}

// buildSigners creates signer instances for each account.
func buildSigners(cfg *config.AppConfig, store store.StateStore) []signer.Signer {
	signers := make([]signer.Signer, 0, len(cfg.Accounts))
	for _, account := range cfg.Accounts {
		svc, err := service.NewService(account.Email, account.Password)
		if err != nil {
			log.Printf("[%s] 初始化 service 失败: %v", account.Email, err)
			continue
		}
		s := signer.NewAccountSigner(account, svc, store, cfg, fileLogger)
		signers = append(signers, s)
	}
	return signers
}

// runChecks executes sign attempts for all signers.
func runChecks(signers []signer.Signer) {
	now := time.Now()
	for _, s := range signers {
		if err := s.AttemptSign(now); err != nil {
			log.Printf("签到失败: %v", err)
		}
	}
}
