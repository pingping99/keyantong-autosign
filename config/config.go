package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

const (
	DefaultTZ                 = "Asia/Shanghai"
	DefaultDataDir            = "./data"
	DefaultRetryInterval      = 10 * time.Minute
	DefaultForceSignOnStart   = true
	DefaultEarlyHourThreshold = 8
	DefaultLateHourThreshold  = 22
	DefaultAPIBaseURL         = "https://www.ablesci.com"
	DefaultAPILoginPath       = "/site/login"
	DefaultAPISignPath        = "/user/sign"
)

var DefaultCheckInterval = 30 * time.Minute

// Account represents a login credential pair.
type Account struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// AppConfig contains runtime configuration.
// Priority: Environment Variables > config.yml > Defaults
type AppConfig struct {
	DataDir            string
	CheckInterval      time.Duration
	Location           *time.Location
	RetryInterval      time.Duration
	ForceSignOnStart   bool
	EarlyHourThreshold int
	LateHourThreshold  int
	APIBaseURL         string
	APILoginPath       string
	APISignPath        string
}

// Load reads configuration following priority: ENV > config.yml > defaults.
func Load() (*AppConfig, error) {
	// Step 1: Try to load config.yml
	configPath := FindConfigFile()
	var yamlCfg *YAMLConfig
	if configPath != "" {
		var err error
		yamlCfg, err = LoadYAML(configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", configPath, err)
		}
		if yamlCfg != nil {
			log.Printf("从 %s 加载配置", configPath)
		}
	}

	// Step 2: Build config with priority: ENV > YAML > Default
	cfg := &AppConfig{}

	cfg.DataDir = resolve(os.Getenv("DATA_DIR"), yamlStr(yamlCfg, "data_dir"), DefaultDataDir)
	cfg.CheckInterval = parseDurationWithDefault(
		resolve(os.Getenv("CHECK_INTERVAL"), yamlStr(yamlCfg, "check_interval"), ""),
		DefaultCheckInterval,
	)
	cfg.RetryInterval = parseDurationWithDefault(
		resolve(os.Getenv("RETRY_INTERVAL"), yamlStr(yamlCfg, "retry_interval"), ""),
		DefaultRetryInterval,
	)
	cfg.ForceSignOnStart = resolveBool(
		os.Getenv("FORCE_SIGN_ON_START"),
		yamlBool(yamlCfg),
		DefaultForceSignOnStart,
	)
	cfg.EarlyHourThreshold = resolveInt(
		os.Getenv("EARLY_HOUR_THRESHOLD"),
		yamlInt(yamlCfg, "early"),
		DefaultEarlyHourThreshold,
	)
	cfg.LateHourThreshold = resolveInt(
		os.Getenv("LATE_HOUR_THRESHOLD"),
		yamlInt(yamlCfg, "late"),
		DefaultLateHourThreshold,
	)
	cfg.APIBaseURL = resolve(os.Getenv("API_BASE_URL"), yamlStr(yamlCfg, "api_base_url"), DefaultAPIBaseURL)
	cfg.APILoginPath = resolve(os.Getenv("API_LOGIN_PATH"), yamlStr(yamlCfg, "api_login_path"), DefaultAPILoginPath)
	cfg.APISignPath = resolve(os.Getenv("API_SIGN_PATH"), yamlStr(yamlCfg, "api_sign_path"), DefaultAPISignPath)

	locName := resolve(os.Getenv("TZ"), yamlStr(yamlCfg, "timezone"), DefaultTZ)
	loc, err := time.LoadLocation(locName)
	if err != nil {
		log.Printf("Failed to load timezone %q, falling back to Local: %v", locName, err)
		loc = time.Local
	}
	cfg.Location = loc

	return cfg, nil
}

// LoadAccounts loads accounts following priority:
//
//	ENV (ABLESCI_EMAIL/PASSWORD) > config.yml accounts > data/accounts.json
func LoadAccounts(dataDir string) ([]Account, error) {
	// Priority 1: Environment variables
	email := os.Getenv("ABLESCI_EMAIL")
	password := os.Getenv("ABLESCI_PASSWORD")
	if email != "" && password != "" {
		return []Account{{Email: email, Password: password}}, nil
	}

	// Priority 2: config.yml accounts section
	configPath := FindConfigFile()
	if configPath != "" {
		yamlCfg, err := LoadYAML(configPath)
		if err == nil && yamlCfg != nil && len(yamlCfg.Accounts) > 0 {
			return yamlCfg.Accounts, nil
		}
	}

	// Priority 3: data/accounts.json file
	accountsPath := filepath.Join(dataDir, "accounts.json")
	if data, err := os.ReadFile(accountsPath); err == nil {
		var accounts []Account
		if err := json.Unmarshal(data, &accounts); err != nil {
			return nil, err
		}
		if len(accounts) > 0 {
			return accounts, nil
		}
	}

	return nil, errors.New("no accounts found: set ABLESCI_EMAIL/PASSWORD env vars, or configure accounts in config.yml, or create data/accounts.json")
}

// --- Priority resolution helpers ---

func resolve(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func resolveBool(envVal string, yamlVal *bool, fallback bool) bool {
	if envVal != "" {
		return parseBoolWithDefault(envVal, fallback)
	}
	if yamlVal != nil {
		return *yamlVal
	}
	return fallback
}

func resolveInt(envVal string, yamlVal *int, fallback int) int {
	if envVal != "" {
		return parseIntWithDefault(envVal, fallback)
	}
	if yamlVal != nil {
		return *yamlVal
	}
	return fallback
}

// --- YAML accessor helpers ---

func yamlStr(cfg *YAMLConfig, field string) string {
	if cfg == nil {
		return ""
	}
	switch field {
	case "data_dir":
		return cfg.DataDir
	case "check_interval":
		return cfg.CheckInterval
	case "retry_interval":
		return cfg.RetryInterval
	case "timezone":
		return cfg.Timezone
	case "api_base_url":
		return cfg.API.BaseURL
	case "api_login_path":
		return cfg.API.LoginPath
	case "api_sign_path":
		return cfg.API.SignPath
	}
	return ""
}

func yamlBool(cfg *YAMLConfig) *bool {
	if cfg == nil {
		return nil
	}
	return cfg.ForceSignOnStart
}

func yamlInt(cfg *YAMLConfig, which string) *int {
	if cfg == nil {
		return nil
	}
	switch which {
	case "early":
		return cfg.EarlyHourThreshold
	case "late":
		return cfg.LateHourThreshold
	}
	return nil
}

// --- Parse helpers ---

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

// String implements fmt.Stringer to redact sensitive information
func (a Account) String() string {
	return "Account{Email: " + a.Email + ", Password: [REDACTED]}"
}
