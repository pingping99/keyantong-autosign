package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"keyantong/service"
)

const (
	defaultWindowStart = "08:00"
	defaultWindowEnd   = "08:10"
	defaultTZ          = "Asia/Shanghai"
	defaultDataDir     = "./data"
	dateLayout         = "2006-01-02"
)

var defaultCheckInterval = 30 * time.Minute

// AppConfig contains runtime configuration sourced from environment variables.
type AppConfig struct {
	Email         string
	Password      string
	DataDir       string
	CheckInterval time.Duration
	WindowStart   time.Duration
	WindowEnd     time.Duration
	Location      *time.Location
}

// SignState tracks the last date the script successfully signed in.
type SignState struct {
	LastSignDate string `json:"last_sign_date"`
}

func main() {
	cfg := mustLoadConfig()

	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		log.Fatalf("Failed to create data directory %q: %v", cfg.DataDir, err)
	}

	logFilePath := filepath.Join(cfg.DataDir, "sign.log")
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o664)
	if err != nil {
		log.Fatalf("Failed to open log file %q: %v", logFilePath, err)
	}
	defer logFile.Close()
	log.SetOutput(io.MultiWriter(os.Stdout, logFile))
	log.SetFlags(log.LstdFlags)

	statePath := filepath.Join(cfg.DataDir, "state.json")
	state, err := loadState(statePath)
	if err != nil {
		log.Printf("Unable to load state: %v", err)
		state = &SignState{}
	}

	svc, err := service.NewService(cfg.Email, cfg.Password)
	if err != nil {
		log.Fatalf("Failed to create service: %v", err)
	}

	log.Printf("签到窗口 %s-%s，间隔 %s，时区 %s，状态路径 %s",
		formatWindow(cfg.WindowStart), formatWindow(cfg.WindowEnd), cfg.CheckInterval, cfg.Location, statePath)

	attemptSign(svc, cfg, state, statePath)

	ticker := time.NewTicker(cfg.CheckInterval)
	defer ticker.Stop()

	for range ticker.C {
		attemptSign(svc, cfg, state, statePath)
	}
}

func mustLoadConfig() AppConfig {
	email := os.Getenv("ABLESCI_EMAIL")
	password := os.Getenv("ABLESCI_PASSWORD")
	if email == "" || password == "" {
		log.Fatal("Environment variables ABLESCI_EMAIL and ABLESCI_PASSWORD must be set")
	}

	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = defaultDataDir
	}

	interval := parseDurationWithDefault(os.Getenv("CHECK_INTERVAL"), defaultCheckInterval)
	windowStart := parseTimeWindow(os.Getenv("SIGN_WINDOW_START"), defaultWindowStart)
	windowEnd := parseTimeWindow(os.Getenv("SIGN_WINDOW_END"), defaultWindowEnd)

	locName := os.Getenv("TZ")
	if locName == "" {
		locName = defaultTZ
	}
	loc, err := time.LoadLocation(locName)
	if err != nil {
		log.Printf("Failed to load timezone %q, falling back to Local: %v", locName, err)
		loc = time.Local
	}

	return AppConfig{
		Email:         email,
		Password:      password,
		DataDir:       dataDir,
		CheckInterval: interval,
		WindowStart:   windowStart,
		WindowEnd:     windowEnd,
		Location:      loc,
	}
}

func attemptSign(svc *service.Service, cfg AppConfig, state *SignState, statePath string) {
	now := time.Now().In(cfg.Location)
	today := now.Format(dateLayout)
	windowRange := fmt.Sprintf("%s-%s", formatWindow(cfg.WindowStart), formatWindow(cfg.WindowEnd))

	if !isWithinWindow(now, cfg.WindowStart, cfg.WindowEnd) {
		log.Printf("当前时间 %s 不在签到窗口 %s，等待下一次检查", now.Format("15:04"), windowRange)
		return
	}

	if state.LastSignDate == today {
		log.Printf("今天 (%s) 已完成签到，跳过", today)
		return
	}

	log.Printf("签到窗口开启，开始签到... (当前时间 %s)", now.Format("15:04"))
	if err := svc.AutoSign(); err != nil {
		log.Printf("签到失败: %v", err)
		return
	}

	state.LastSignDate = today
	if err := saveState(statePath, state); err != nil {
		log.Printf("保存签到状态失败: %v", err)
	}
}

func parseDurationWithDefault(raw string, fallback time.Duration) time.Duration {
	if raw == "" {
		return fallback
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		log.Printf("invalid duration %q, using default %s: %v", raw, fallback, err)
		return fallback
	}
	return d
}

func parseTimeWindow(raw, fallback string) time.Duration {
	input := raw
	if input == "" {
		input = fallback
	}
	parsed, err := time.Parse("15:04", input)
	if err != nil {
		log.Printf("invalid clock %q, fallback to %s: %v", input, fallback, err)
		parsed, _ = time.Parse("15:04", fallback)
	}
	return time.Duration(parsed.Hour())*time.Hour + time.Duration(parsed.Minute())*time.Minute
}

func isWithinWindow(now time.Time, start, end time.Duration) bool {
	current := time.Duration(now.Hour())*time.Hour + time.Duration(now.Minute())*time.Minute
	if start <= end {
		return current >= start && current <= end
	}
	return current >= start || current <= end
}

func formatWindow(d time.Duration) string {
	h := int(d / time.Hour)
	m := int((d % time.Hour) / time.Minute)
	return fmt.Sprintf("%02d:%02d", h, m)
}

func loadState(path string) (*SignState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return &SignState{}, err
	}
	var state SignState
	if err := json.Unmarshal(data, &state); err != nil {
		return &SignState{}, err
	}
	return &state, nil
}

func saveState(path string, state *SignState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
