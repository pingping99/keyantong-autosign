package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultTZ                 = "Asia/Shanghai"
	DefaultDataDir            = "./data"
	DefaultCheckInterval      = 30 * time.Minute
	DefaultRetryInterval      = 10 * time.Minute
	DefaultSignJitterMax      = 5 * time.Minute
	DefaultForceSignOnStart   = false
	DefaultEarlyHourThreshold = 8
	DefaultLateHourThreshold  = 22
	DefaultHealthCheckHost    = "127.0.0.1"
	DefaultHealthCheckPort    = 8080
	DefaultAPIBaseURL         = "https://www.ablesci.com"
	DefaultAPILoginPath       = "/site/login"
	DefaultAPISignPath        = "/user/sign"
)

type AppConfig struct {
	Email              string
	Password           string
	DataDir            string
	CheckInterval      time.Duration
	RetryInterval      time.Duration
	SignJitterMax      time.Duration
	Location           *time.Location
	ForceSignOnStart   bool
	EarlyHourThreshold int
	LateHourThreshold  int
	HealthCheckHost    string
	HealthCheckPort    int
	APIBaseURL         string
	APILoginPath       string
	APISignPath        string
}

func Load() (*AppConfig, error) {
	var yamlCfg *YAMLConfig
	configPath := FindConfigFile()
	if configPath != "" {
		var err error
		yamlCfg, err = LoadYAML(configPath)
		if err != nil {
			return nil, fmt.Errorf("load configuration %q: %w", configPath, err)
		}
	}

	email, password, err := resolveCredentials(
		os.Getenv("ABLESCI_EMAIL"), os.Getenv("ABLESCI_PASSWORD"),
		yamlString(yamlCfg, "email"), yamlString(yamlCfg, "password"),
	)
	if err != nil {
		return nil, err
	}

	checkInterval, err := parseDuration(
		resolve(os.Getenv("CHECK_INTERVAL"), yamlString(yamlCfg, "check_interval"), DefaultCheckInterval.String()),
		"CHECK_INTERVAL",
	)
	if err != nil {
		return nil, err
	}
	retryInterval, err := parseDuration(
		resolve(os.Getenv("RETRY_INTERVAL"), yamlString(yamlCfg, "retry_interval"), DefaultRetryInterval.String()),
		"RETRY_INTERVAL",
	)
	if err != nil {
		return nil, err
	}
	signJitterMax, err := parseDuration(
		resolve(os.Getenv("SIGN_JITTER_MAX"), yamlString(yamlCfg, "sign_jitter_max"), DefaultSignJitterMax.String()),
		"SIGN_JITTER_MAX",
	)
	if err != nil {
		return nil, err
	}
	forceSignOnStart, err := resolveBool(
		os.Getenv("FORCE_SIGN_ON_START"),
		yamlBool(yamlCfg),
		DefaultForceSignOnStart,
	)
	if err != nil {
		return nil, fmt.Errorf("FORCE_SIGN_ON_START: %w", err)
	}
	earlyHour, err := resolveInt(
		os.Getenv("EARLY_HOUR_THRESHOLD"),
		yamlInt(yamlCfg, "early"),
		DefaultEarlyHourThreshold,
	)
	if err != nil {
		return nil, fmt.Errorf("EARLY_HOUR_THRESHOLD: %w", err)
	}
	lateHour, err := resolveInt(
		os.Getenv("LATE_HOUR_THRESHOLD"),
		yamlInt(yamlCfg, "late"),
		DefaultLateHourThreshold,
	)
	if err != nil {
		return nil, fmt.Errorf("LATE_HOUR_THRESHOLD: %w", err)
	}
	healthPort, err := resolveInt(
		os.Getenv("HEALTH_CHECK_PORT"),
		yamlIntString(yamlCfg, "health_check_port"),
		DefaultHealthCheckPort,
	)
	if err != nil {
		return nil, fmt.Errorf("HEALTH_CHECK_PORT: %w", err)
	}

	locationName := resolve(os.Getenv("TZ"), yamlString(yamlCfg, "timezone"), DefaultTZ)
	location, err := time.LoadLocation(locationName)
	if err != nil {
		return nil, fmt.Errorf("load timezone %q: %w", locationName, err)
	}

	cfg := &AppConfig{
		Email:              email,
		Password:           password,
		DataDir:            resolve(os.Getenv("DATA_DIR"), yamlString(yamlCfg, "data_dir"), DefaultDataDir),
		CheckInterval:      checkInterval,
		RetryInterval:      retryInterval,
		SignJitterMax:      signJitterMax,
		Location:           location,
		ForceSignOnStart:   forceSignOnStart,
		EarlyHourThreshold: earlyHour,
		LateHourThreshold:  lateHour,
		HealthCheckHost:    resolve(os.Getenv("HEALTH_CHECK_HOST"), yamlString(yamlCfg, "health_check_host"), DefaultHealthCheckHost),
		HealthCheckPort:    healthPort,
		APIBaseURL:         strings.TrimRight(resolve(os.Getenv("API_BASE_URL"), yamlString(yamlCfg, "api_base_url"), DefaultAPIBaseURL), "/"),
		APILoginPath:       resolve(os.Getenv("API_LOGIN_PATH"), yamlString(yamlCfg, "api_login_path"), DefaultAPILoginPath),
		APISignPath:        resolve(os.Getenv("API_SIGN_PATH"), yamlString(yamlCfg, "api_sign_path"), DefaultAPISignPath),
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func resolveCredentials(envEmail, envPassword, yamlEmail, yamlPassword string) (string, string, error) {
	envEmail = strings.TrimSpace(envEmail)
	yamlEmail = strings.TrimSpace(yamlEmail)

	if envEmail != "" || envPassword != "" {
		if envEmail == "" || envPassword == "" {
			return "", "", fmt.Errorf("ABLESCI_EMAIL and ABLESCI_PASSWORD must be set together")
		}
		return envEmail, envPassword, nil
	}
	if yamlEmail == "" || yamlPassword == "" {
		return "", "", fmt.Errorf("single account credentials are required: set both ABLESCI_EMAIL and ABLESCI_PASSWORD, or email/password in config.yaml")
	}
	return yamlEmail, yamlPassword, nil
}

func (cfg *AppConfig) Validate() error {
	if cfg.Email == "" || cfg.Password == "" {
		return fmt.Errorf("single account credentials are required")
	}
	if cfg.DataDir == "" {
		return fmt.Errorf("DATA_DIR must not be empty")
	}
	if cfg.CheckInterval <= 0 {
		return fmt.Errorf("CHECK_INTERVAL must be greater than zero")
	}
	if cfg.RetryInterval <= 0 {
		return fmt.Errorf("RETRY_INTERVAL must be greater than zero")
	}
	if cfg.SignJitterMax < 0 {
		return fmt.Errorf("SIGN_JITTER_MAX must not be negative")
	}
	if cfg.EarlyHourThreshold < 0 || cfg.EarlyHourThreshold > 23 {
		return fmt.Errorf("EARLY_HOUR_THRESHOLD must be between 0 and 23")
	}
	if cfg.LateHourThreshold < 1 || cfg.LateHourThreshold > 24 {
		return fmt.Errorf("LATE_HOUR_THRESHOLD must be between 1 and 24")
	}
	if cfg.EarlyHourThreshold >= cfg.LateHourThreshold {
		return fmt.Errorf("EARLY_HOUR_THRESHOLD must be earlier than LATE_HOUR_THRESHOLD")
	}
	if strings.TrimSpace(cfg.HealthCheckHost) == "" || strings.ContainsAny(cfg.HealthCheckHost, " \t\r\n") {
		return fmt.Errorf("HEALTH_CHECK_HOST must be a host name or IP address")
	}
	if cfg.HealthCheckPort < 1 || cfg.HealthCheckPort > 65535 {
		return fmt.Errorf("HEALTH_CHECK_PORT must be between 1 and 65535")
	}

	parsedURL, err := url.Parse(cfg.APIBaseURL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return fmt.Errorf("API_BASE_URL must be an absolute URL")
	}
	if parsedURL.Scheme != "https" && parsedURL.Scheme != "http" {
		return fmt.Errorf("API_BASE_URL scheme must be http or https")
	}
	if !strings.HasPrefix(cfg.APILoginPath, "/") || !strings.HasPrefix(cfg.APISignPath, "/") {
		return fmt.Errorf("API paths must start with /")
	}
	return nil
}

func resolve(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func parseDuration(raw, name string) (time.Duration, error) {
	duration, err := time.ParseDuration(strings.TrimSpace(raw))
	if err != nil {
		return 0, fmt.Errorf("%s: invalid duration %q: %w", name, raw, err)
	}
	return duration, nil
}

func resolveBool(envValue string, yamlValue *bool, fallback bool) (bool, error) {
	if envValue != "" {
		value, err := strconv.ParseBool(strings.ToLower(strings.TrimSpace(envValue)))
		if err != nil {
			return false, fmt.Errorf("invalid boolean %q", envValue)
		}
		return value, nil
	}
	if yamlValue != nil {
		return *yamlValue, nil
	}
	return fallback, nil
}

func resolveInt(envValue string, yamlValue *int, fallback int) (int, error) {
	if envValue != "" {
		value, err := strconv.Atoi(strings.TrimSpace(envValue))
		if err != nil {
			return 0, fmt.Errorf("invalid integer %q", envValue)
		}
		return value, nil
	}
	if yamlValue != nil {
		return *yamlValue, nil
	}
	return fallback, nil
}

func yamlString(cfg *YAMLConfig, field string) string {
	if cfg == nil {
		return ""
	}
	switch field {
	case "email":
		return cfg.Email
	case "password":
		return cfg.Password
	case "data_dir":
		return cfg.DataDir
	case "check_interval":
		return cfg.CheckInterval
	case "retry_interval":
		return cfg.RetryInterval
	case "sign_jitter_max":
		return cfg.SignJitterMax
	case "timezone":
		return cfg.Timezone
	case "health_check_host":
		return cfg.HealthCheckHost
	case "api_base_url":
		return cfg.APIBaseURL
	case "api_login_path":
		return cfg.APILoginPath
	case "api_sign_path":
		return cfg.APISignPath
	default:
		return ""
	}
}

func yamlBool(cfg *YAMLConfig) *bool {
	if cfg == nil {
		return nil
	}
	return cfg.ForceSignOnStart
}

func yamlInt(cfg *YAMLConfig, field string) *int {
	if cfg == nil {
		return nil
	}
	switch field {
	case "early":
		return cfg.EarlyHourThreshold
	case "late":
		return cfg.LateHourThreshold
	default:
		return nil
	}
}

func yamlIntString(cfg *YAMLConfig, field string) *int {
	if cfg == nil || field != "health_check_port" || cfg.HealthCheckPort == "" {
		return nil
	}
	value, err := strconv.Atoi(cfg.HealthCheckPort)
	if err != nil {
		invalid := 0
		return &invalid
	}
	return &value
}
