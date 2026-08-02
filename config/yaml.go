package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// YAMLConfig represents the supported subset of config.yaml.
// The project deliberately supports only scalar keys and the legacy one-level
// api section so configuration remains dependency-free and predictable.
type YAMLConfig struct {
	Email              string
	Password           string
	DataDir            string
	CheckInterval      string
	RetryInterval      string
	SignJitterMax      string
	Timezone           string
	HealthCheckPort    string
	ForceSignOnStart   *bool
	EarlyHourThreshold *int
	LateHourThreshold  *int
	APIBaseURL         string
	APILoginPath       string
	APISignPath        string
}

var configSearchPaths = []string{
	"config.yaml",
	"config.yml",
	"data/config.yaml",
	"data/config.yml",
}

// FindConfigFile returns CONFIG_FILE when set, otherwise searches default paths.
func FindConfigFile() string {
	if path := strings.TrimSpace(os.Getenv("CONFIG_FILE")); path != "" {
		return path
	}
	for _, path := range configSearchPaths {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path
		}
	}
	return ""
}

// LoadYAML parses the small YAML subset used by this project.
func LoadYAML(path string) (*YAMLConfig, error) {
	if path == "" {
		return nil, nil
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	cfg := &YAMLConfig{}
	scanner := bufio.NewScanner(file)
	section := ""
	for lineNo := 1; scanner.Scan(); lineNo++ {
		raw := strings.TrimSpace(stripYAMLComment(scanner.Text()))
		if raw == "" || raw == "---" {
			continue
		}

		indent := leadingSpaces(scanner.Text())
		if strings.HasSuffix(raw, ":") && !strings.Contains(strings.TrimSuffix(raw, ":"), ":") {
			section = strings.TrimSpace(strings.TrimSuffix(raw, ":"))
			if section == "accounts" {
				return nil, fmt.Errorf("%s:%d: multi-account configuration is no longer supported", path, lineNo)
			}
			if section != "api" {
				return nil, fmt.Errorf("%s:%d: unsupported section %q", path, lineNo, section)
			}
			continue
		}

		key, value, ok := splitYAMLKeyValue(raw)
		if !ok {
			return nil, fmt.Errorf("%s:%d: expected key: value", path, lineNo)
		}
		if indent == 0 {
			section = ""
		}
		value, err = unquoteYAMLScalar(value)
		if err != nil {
			return nil, fmt.Errorf("%s:%d: %w", path, lineNo, err)
		}

		fullKey := key
		if section != "" && indent > 0 {
			fullKey = section + "." + key
		}
		if err := assignYAMLValue(cfg, fullKey, value); err != nil {
			return nil, fmt.Errorf("%s:%d: %w", path, lineNo, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func assignYAMLValue(cfg *YAMLConfig, key, value string) error {
	switch key {
	case "email":
		cfg.Email = value
	case "password":
		cfg.Password = value
	case "data_dir":
		cfg.DataDir = value
	case "check_interval":
		cfg.CheckInterval = value
	case "retry_interval":
		cfg.RetryInterval = value
	case "sign_jitter_max":
		cfg.SignJitterMax = value
	case "timezone":
		cfg.Timezone = value
	case "health_check_port":
		cfg.HealthCheckPort = value
	case "force_sign_on_start":
		parsed, err := strconv.ParseBool(strings.ToLower(value))
		if err != nil {
			return fmt.Errorf("invalid boolean for %s: %q", key, value)
		}
		cfg.ForceSignOnStart = &parsed
	case "early_hour_threshold":
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid integer for %s: %q", key, value)
		}
		cfg.EarlyHourThreshold = &parsed
	case "late_hour_threshold":
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid integer for %s: %q", key, value)
		}
		cfg.LateHourThreshold = &parsed
	case "api_base_url", "api.base_url":
		cfg.APIBaseURL = value
	case "api_login_path", "api.login_path":
		cfg.APILoginPath = value
	case "api_sign_path", "api.sign_path":
		cfg.APISignPath = value
	case "accounts":
		return fmt.Errorf("multi-account configuration is no longer supported")
	default:
		return fmt.Errorf("unsupported configuration key %q", key)
	}
	return nil
}

func splitYAMLKeyValue(line string) (string, string, bool) {
	inSingle, inDouble := false, false
	for i, r := range line {
		switch r {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case ':':
			if !inSingle && !inDouble {
				key := strings.TrimSpace(line[:i])
				value := strings.TrimSpace(line[i+1:])
				return key, value, key != ""
			}
		}
	}
	return "", "", false
}

func stripYAMLComment(line string) string {
	inSingle, inDouble := false, false
	for i, r := range line {
		switch r {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case '#':
			if !inSingle && !inDouble && (i == 0 || line[i-1] == ' ' || line[i-1] == '\t') {
				return line[:i]
			}
		}
	}
	return line
}

func unquoteYAMLScalar(value string) (string, error) {
	value = strings.TrimSpace(value)
	if len(value) >= 2 && value[0] == '\'' && value[len(value)-1] == '\'' {
		return strings.ReplaceAll(value[1:len(value)-1], "''", "'"), nil
	}
	if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
		unquoted, err := strconv.Unquote(value)
		if err != nil {
			return "", fmt.Errorf("invalid quoted string: %w", err)
		}
		return unquoted, nil
	}
	return value, nil
}

func leadingSpaces(line string) int {
	count := 0
	for _, r := range line {
		if r != ' ' {
			break
		}
		count++
	}
	return count
}
