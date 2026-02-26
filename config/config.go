package config

import (
	"errors"
	"log"
	"os"
	"time"
)

const (
	DefaultTZ                 = "Asia/Shanghai"
	DefaultDataDir            = "./data"
	DateLayout                = "2006-01-02"
	TimeLayout                = "15:04:05"
	DefaultRetryInterval      = 10 * time.Minute
	DefaultForceSignOnStart   = true
)

var DefaultCheckInterval = 30 * time.Minute

// AppConfig contains runtime configuration sourced from environment variables.
type AppConfig struct {
	Email            string
	Password         string
	DataDir          string
	CheckInterval    time.Duration
	Location         *time.Location
	RetryInterval    time.Duration
	ForceSignOnStart bool
}

// Load reads configuration from environment variables.
func Load() (*AppConfig, error) {
	email := os.Getenv("ABLESCI_EMAIL")
	password := os.Getenv("ABLESCI_PASSWORD")
	if email == "" || password == "" {
		return nil, errors.New("environment variables ABLESCI_EMAIL and ABLESCI_PASSWORD must be set")
	}

	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = DefaultDataDir
	}

	interval := parseDurationWithDefault(os.Getenv("CHECK_INTERVAL"), DefaultCheckInterval)
	retryInterval := parseDurationWithDefault(os.Getenv("RETRY_INTERVAL"), DefaultRetryInterval)
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

	return &AppConfig{
		Email:            email,
		Password:         password,
		DataDir:          dataDir,
		CheckInterval:    interval,
		Location:         loc,
		RetryInterval:    retryInterval,
		ForceSignOnStart: forceSignOnStart,
	}, nil
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
