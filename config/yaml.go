package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// YAMLConfig represents the parsed configuration from config.yml.
type YAMLConfig struct {
	DataDir            string    `yaml:"data_dir"`
	CheckInterval      string    `yaml:"check_interval"`
	RetryInterval      string    `yaml:"retry_interval"`
	Timezone           string    `yaml:"timezone"`
	ForceSignOnStart   *bool     `yaml:"force_sign_on_start"`
	EarlyHourThreshold *int      `yaml:"early_hour_threshold"`
	LateHourThreshold  *int      `yaml:"late_hour_threshold"`
	API                yamlAPI   `yaml:"api"`
	Accounts           []Account `yaml:"accounts"`
}

type yamlAPI struct {
	BaseURL   string `yaml:"base_url"`
	LoginPath string `yaml:"login_path"`
	SignPath  string `yaml:"sign_path"`
}

// configSearchPaths defines the ordered list of paths to search for config.yml.
var configSearchPaths = []string{
	"config.yaml",
	"data/config.yaml",
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

// LoadYAML reads and parses a config.yml file.
// Returns nil (no error) if the file doesn't exist.
func LoadYAML(path string) (*YAMLConfig, error) {
	if path == "" {
		return nil, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var cfg YAMLConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
