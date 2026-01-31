package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"keyantong/domain"
	"log"
	"os"
	"path/filepath"
	"time"
)

const (
	DefaultDynamicWindowStart = "08:00"
	DefaultDynamicWindowEnd   = "18:00"
	DefaultDynamicWindowSpan  = 45 * time.Minute
	DefaultTZ                 = "Asia/Shanghai"
	DefaultDataDir            = "./data"
	DateLayout                = "2006-01-02"
	TimeLayout                = "15:04"
	DefaultRetryInterval      = 10 * time.Minute
	DefaultForceSignOnStart   = true
)

var DefaultCheckInterval = 30 * time.Minute

// AppConfig contains runtime configuration sourced from environment variables.
type AppConfig struct {
	Accounts           []domain.Account
	DataDir            string
	CheckInterval      time.Duration
	DynamicWindowStart time.Duration
	DynamicWindowEnd   time.Duration
	DynamicWindowSpan  time.Duration
	Location           *time.Location
	RetryInterval      time.Duration
	ForceSignOnStart   bool
}

// Load reads configuration from environment variables and files.
func Load() (*AppConfig, error) {
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = DefaultDataDir
	}

	interval := parseDurationWithDefault(os.Getenv("CHECK_INTERVAL"), DefaultCheckInterval)
	retryInterval := parseDurationWithDefault(os.Getenv("RETRY_INTERVAL"), DefaultRetryInterval)
	dynamicWindowStart := parseTimeWindow(os.Getenv("DYNAMIC_WINDOW_START"), DefaultDynamicWindowStart)
	dynamicWindowEnd := parseTimeWindow(os.Getenv("DYNAMIC_WINDOW_END"), DefaultDynamicWindowEnd)
	dynamicWindowSpan := parseDurationWithDefault(os.Getenv("DYNAMIC_WINDOW_SPAN"), DefaultDynamicWindowSpan)
	forceSignOnStart := parseBoolWithDefault(os.Getenv("FORCE_SIGN_ON_START"), DefaultForceSignOnStart)

	locName := os.Getenv("TZ")
	if locName == "" {
		locName = DefaultTZ
	}
	loc, err := time.LoadLocation(locName)
	if err != nil {
		log.Printf("Failed to load timezone %q, falling back to Local: %v", locName, err)
		loc = time.Local
	}

	accounts, err := loadAccounts(filepath.Join(dataDir, "accounts.json"))
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("failed to load accounts.json: %w", err)
		}
		email := os.Getenv("ABLESCI_EMAIL")
		password := os.Getenv("ABLESCI_PASSWORD")
		if email == "" || password == "" {
			return nil, errors.New("environment variables ABLESCI_EMAIL and ABLESCI_PASSWORD must be set when accounts.json is absent")
		}
		accounts = []domain.Account{{Email: email, Password: password}}
	}

	if len(accounts) == 0 {
		return nil, errors.New("no accounts found in configuration")
	}

	// Set account IDs
	for i := range accounts {
		accounts[i].SetID()
	}

	return &AppConfig{
		Accounts:           accounts,
		DataDir:            dataDir,
		CheckInterval:      interval,
		DynamicWindowStart: dynamicWindowStart,
		DynamicWindowEnd:   dynamicWindowEnd,
		DynamicWindowSpan:  dynamicWindowSpan,
		Location:           loc,
		RetryInterval:      retryInterval,
		ForceSignOnStart:   forceSignOnStart,
	}, nil
}

// loadAccounts reads accounts from JSON file.
func loadAccounts(path string) ([]domain.Account, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var payload struct {
		Accounts []domain.Account `json:"accounts"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse %q: %w", path, err)
	}

	filtered := make([]domain.Account, 0, len(payload.Accounts))
	for _, acc := range payload.Accounts {
		if acc.Email == "" || acc.Password == "" {
			continue
		}
		filtered = append(filtered, acc)
	}

	return filtered, nil
}

// parseDurationWithDefault parses duration string or returns default.
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

// parseTimeWindow parses time string (HH:MM) to duration since midnight.
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

// parseBoolWithDefault parses boolean string or returns default.
func parseBoolWithDefault(raw string, fallback bool) bool {
	if raw == "" {
		return fallback
	}
	switch raw {
	case "true", "1", "yes", "on":
		return true
	case "false", "0", "no", "off":
		return false
	default:
		log.Printf("invalid boolean %q, using default %t", raw, fallback)
		return fallback
	}
}
