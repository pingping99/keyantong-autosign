package config

import (
	"bufio"
	"os"
	"strings"
)

// YAMLConfig represents the parsed configuration from config.yml.
type YAMLConfig struct {
	DataDir            string
	CheckInterval      string
	RetryInterval      string
	Timezone           string
	ForceSignOnStart   *bool // pointer to distinguish "not set" from "false"
	EarlyHourThreshold *int
	LateHourThreshold  *int
	APIBaseURL         string
	APILoginPath       string
	APISignPath        string
	Accounts           []Account
}

// configSearchPaths defines the ordered list of paths to search for config.yml.
var configSearchPaths = []string{
	"config.yml",
	"config.yaml",
}

// FindConfigFile searches for a configuration file in the default locations.
func FindConfigFile() string {
	for _, path := range configSearchPaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

// LoadYAML reads and parses a config.yml file using a lightweight line-based parser.
func LoadYAML(path string) (*YAMLConfig, error) {
	if path == "" {
		return nil, nil
	}

	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer file.Close()

	cfg := &YAMLConfig{}
	var inAccounts bool
	var currentAccount *Account

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		indent := len(line) - len(strings.TrimLeft(line, " "))

		if indent == 0 {
			if currentAccount != nil {
				cfg.Accounts = append(cfg.Accounts, *currentAccount)
				currentAccount = nil
			}

			if trimmed == "accounts:" {
				inAccounts = true
				continue
			}
			if strings.HasPrefix(trimmed, "api:") {
				inAccounts = false
				continue
			}

			inAccounts = false
			key, value := parseKV(trimmed)
			applyTopLevel(cfg, key, value)
			continue
		}

		if inAccounts {
			if strings.HasPrefix(trimmed, "- ") {
				if currentAccount != nil {
					cfg.Accounts = append(cfg.Accounts, *currentAccount)
				}
				currentAccount = &Account{}
				rest := strings.TrimPrefix(trimmed, "- ")
				key, value := parseKV(rest)
				applyAccount(currentAccount, key, value)
			} else if currentAccount != nil {
				key, value := parseKV(trimmed)
				applyAccount(currentAccount, key, value)
			}
			continue
		}

		if indent > 0 {
			key, value := parseKV(trimmed)
			applyAPIConfig(cfg, key, value)
		}
	}

	if currentAccount != nil {
		cfg.Accounts = append(cfg.Accounts, *currentAccount)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func parseKV(line string) (string, string) {
	idx := strings.Index(line, ":")
	if idx < 0 {
		return strings.TrimSpace(line), ""
	}
	key := strings.TrimSpace(line[:idx])
	value := strings.TrimSpace(line[idx+1:])
	value = strings.Trim(value, "\"'")
	return key, value
}

func applyTopLevel(cfg *YAMLConfig, key, value string) {
	switch key {
	case "data_dir":
		cfg.DataDir = value
	case "check_interval":
		cfg.CheckInterval = value
	case "retry_interval":
		cfg.RetryInterval = value
	case "timezone":
		cfg.Timezone = value
	case "force_sign_on_start":
		b := parseBoolValue(value)
		cfg.ForceSignOnStart = &b
	case "early_hour_threshold":
		i := parseIntValue(value)
		cfg.EarlyHourThreshold = &i
	case "late_hour_threshold":
		i := parseIntValue(value)
		cfg.LateHourThreshold = &i
	}
}

func applyAPIConfig(cfg *YAMLConfig, key, value string) {
	switch key {
	case "base_url":
		cfg.APIBaseURL = value
	case "login_path":
		cfg.APILoginPath = value
	case "sign_path":
		cfg.APISignPath = value
	}
}

func applyAccount(acc *Account, key, value string) {
	switch key {
	case "email":
		acc.Email = value
	case "password":
		acc.Password = value
	}
}

func parseBoolValue(s string) bool {
	switch strings.ToLower(s) {
	case "true", "1", "yes", "on":
		return true
	default:
		return false
	}
}

func parseIntValue(s string) int {
	var val int
	for _, c := range s {
		if c >= '0' && c <= '9' {
			val = val*10 + int(c-'0')
		} else {
			break
		}
	}
	return val
}
