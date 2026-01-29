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

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Setup logging
	logFilePath := filepath.Join(cfg.DataDir, "sign.log")
	firstRun := isFirstRun(logFilePath)
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o664)
	if err != nil {
		log.Fatalf("Failed to open log file %q: %v", logFilePath, err)
	}
	defer logFile.Close()
	log.SetOutput(io.MultiWriter(os.Stdout, logFile))
	log.SetFlags(log.LstdFlags)

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

	log.Printf("签到窗口 %s-%s，间隔 %s，时区 %s，账号数量 %d",
		scheduler.FormatWindow(cfg.WindowStart), scheduler.FormatWindow(cfg.WindowEnd),
		cfg.CheckInterval, cfg.Location, len(signers))

	// Force sign on first run
	if firstRun {
		log.Printf("首次运行检测：%s 为空，立即执行登录并签到", logFilePath)
		now := time.Now()
		for _, s := range signers {
			if err := s.ForceSign(now); err != nil {
				log.Printf("强制签到失败: %v", err)
			}
		}
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
		s := signer.NewAccountSigner(account, svc, store, cfg)
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

// isFirstRun checks if this is the first run by checking log file.
func isFirstRun(logPath string) bool {
	info, err := os.Stat(logPath)
	if err != nil {
		return os.IsNotExist(err)
	}
	return info.Size() == 0
}
