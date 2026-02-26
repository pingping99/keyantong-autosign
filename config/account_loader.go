package config

import (
	"encoding/json"
	"errors"
	"keyantong/domain"
	"os"
	"path/filepath"
)

// LoadAccounts loads accounts from file or environment variables.
// Tries to load from data/accounts.json first, then falls back to env vars.
func LoadAccounts() ([]domain.Account, error) {
	// Try loading from accounts.json file
	accountsPath := filepath.Join(DefaultDataDir, "accounts.json")
	if data, err := os.ReadFile(accountsPath); err == nil {
		var accounts []domain.Account
		if err := json.Unmarshal(data, &accounts); err != nil {
			return nil, err
		}
		if len(accounts) > 0 {
			return accounts, nil
		}
	}

	// Fall back to environment variables (single account mode)
	email := os.Getenv("ABLESCI_EMAIL")
	password := os.Getenv("ABLESCI_PASSWORD")
	if email == "" || password == "" {
		return nil, errors.New("no accounts found in data/accounts.json and ABLESCI_EMAIL/ABLESCI_PASSWORD not set")
	}

	return []domain.Account{
		{Email: email, Password: password},
	}, nil
}
