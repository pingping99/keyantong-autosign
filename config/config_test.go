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
	content := "email: test@example.com\n" +
		"password: 'a: b # c'\n" +
		"force_sign_on_start: false\n" +
		"health_check_host: 127.0.0.1\n" +
		"api:\n" +
		"  base_url: https://example.com\n" +
		"  login_path: /login\n" +
		"  sign_path: /sign\n"
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
	if cfg.HealthCheckHost != "127.0.0.1" {
		t.Fatalf("unexpected health host: %s", cfg.HealthCheckHost)
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

func TestResolveCredentialsDoesNotMixSources(t *testing.T) {
	if _, _, err := resolveCredentials(
		"env@example.com",
		"",
		"yaml@example.com",
		"yaml-secret",
	); err == nil {
		t.Fatal("expected partial environment credentials to fail")
	}

	email, password, err := resolveCredentials(
		"env@example.com",
		"env-secret",
		"yaml@example.com",
		"yaml-secret",
	)
	if err != nil {
		t.Fatal(err)
	}
	if email != "env@example.com" || password != "env-secret" {
		t.Fatalf("unexpected credentials %q %q", email, password)
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
		DataDir:            "data",
		CheckInterval:      time.Minute,
		RetryInterval:      time.Minute,
		Location:           time.UTC,
		EarlyHourThreshold: 8,
		LateHourThreshold:  22,
		HealthCheckHost:    "127.0.0.1",
		HealthCheckPort:    8080,
		APIBaseURL:         "https://example.com",
		APILoginPath:       "/login",
		APISignPath:        "/sign",
	}
}
