package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadYAMLSingleAccount(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "config.yml")
	content := "email: test@example.com\npassword: 'a: b # c'\nforce_sign_on_start: false\napi:\n  base_url: https://example.com\n  login_path: /login\n  sign_path: /sign\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadYAML(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Email != "test@example.com" || cfg.Password != "a: b # c" {
		t.Fatalf("unexpected credentials: %#v", cfg)
	}
	if cfg.APIBaseURL != "https://example.com" {
		t.Fatalf("unexpected API URL: %s", cfg.APIBaseURL)
	}
}

func TestLoadYAMLRejectsMultiAccount(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("accounts:\n  - email: a@example.com\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadYAML(path); err == nil {
		t.Fatal("expected multi-account configuration error")
	}
}

func TestValidateRejectsInvalidIntervals(t *testing.T) {
	cfg := validConfig()
	cfg.CheckInterval = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error")
	}
}

func validConfig() *AppConfig {
	return &AppConfig{
		Email:              "test@example.com",
		Password:           "secret",
		DataDir:            tTempDataDir,
		CheckInterval:      time.Minute,
		RetryInterval:      time.Minute,
		Location:           time.UTC,
		EarlyHourThreshold: 8,
		LateHourThreshold:  22,
		HealthCheckPort:    8080,
		APIBaseURL:         "https://example.com",
		APILoginPath:       "/login",
		APISignPath:        "/sign",
	}
}

const tTempDataDir = "data"
