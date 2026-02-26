package config

import (
	"fmt"
	"log"
	"os"
	"time"
)

const (
	DefaultTZ                 = "Asia/Shanghai"
	DefaultDataDir            = "./data"
	DefaultRetryInterval      = 10 * time.Minute
	DefaultForceSignOnStart   = true
	DefaultEarlyHourThreshold = 8  // Earliest hour to sign in (00:00-07:59 is too early)
	DefaultLateHourThreshold  = 22 // Always execute if haven't signed by this hour
	DefaultAPIBaseURL         = "https://www.ablesci.com"
	DefaultAPILoginPath       = "/site/login"
	DefaultAPISignPath        = "/user/sign"
)

var DefaultCheckInterval = 30 * time.Minute

// AppConfig contains runtime configuration sourced from environment variables.
// Note: Email and Password are account-level data, not global configuration.
// Load accounts separately using LoadAccounts().
type AppConfig struct {
	DataDir            string
	CheckInterval      time.Duration
	Location           *time.Location
	RetryInterval      time.Duration
	ForceSignOnStart   bool
	EarlyHourThreshold int    // Hour before which to skip signing (e.g., 8 = skip 00:00-07:59)
	LateHourThreshold  int    // Hour at/after which to force sign immediately if not done (e.g., 22)
	APIBaseURL         string // Base URL for API (e.g., https://www.ablesci.com)
	APILoginPath       string // Path for login endpoint
	APISignPath        string // Path for sign-in endpoint
}

// Load reads configuration from environment variables.
// Note: Accounts are loaded separately using LoadAccounts().
func Load() (*AppConfig, error) {
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = DefaultDataDir
	}

	interval := parseDurationWithDefault(os.Getenv("CHECK_INTERVAL"), DefaultCheckInterval)
	retryInterval := parseDurationWithDefault(os.Getenv("RETRY_INTERVAL"), DefaultRetryInterval)
	forceSignOnStart := parseBoolWithDefault(os.Getenv("FORCE_SIGN_ON_START"), DefaultForceSignOnStart)

	earlyHourThreshold := parseIntWithDefault(os.Getenv("EARLY_HOUR_THRESHOLD"), DefaultEarlyHourThreshold)
	lateHourThreshold := parseIntWithDefault(os.Getenv("LATE_HOUR_THRESHOLD"), DefaultLateHourThreshold)

	apiBaseURL := os.Getenv("API_BASE_URL")
	if apiBaseURL == "" {
		apiBaseURL = DefaultAPIBaseURL
	}

	apiLoginPath := os.Getenv("API_LOGIN_PATH")
	if apiLoginPath == "" {
		apiLoginPath = DefaultAPILoginPath
	}

	apiSignPath := os.Getenv("API_SIGN_PATH")
	if apiSignPath == "" {
		apiSignPath = DefaultAPISignPath
	}

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
		DataDir:            dataDir,
		CheckInterval:      interval,
		Location:           loc,
		RetryInterval:      retryInterval,
		ForceSignOnStart:   forceSignOnStart,
		EarlyHourThreshold: earlyHourThreshold,
		LateHourThreshold:  lateHourThreshold,
		APIBaseURL:         apiBaseURL,
		APILoginPath:       apiLoginPath,
		APISignPath:        apiSignPath,
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

// parseIntWithDefault parses integer string or returns default.
func parseIntWithDefault(raw string, fallback int) int {
	if raw == "" {
		return fallback
	}
	var val int
	_, err := fmt.Sscanf(raw, "%d", &val)
	if err != nil {
		log.Printf("invalid integer %q, using default %d: %v", raw, fallback, err)
		return fallback
	}
	return val
}
