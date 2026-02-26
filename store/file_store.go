package store

import (
	"encoding/json"
	"fmt"
	"keyantong/domain"
	"os"
	"path/filepath"
)

// FileStore implements StateStore using local file system.
type FileStore struct {
	dataDir   string
	accountID string // Empty for default (backward compatibility), or account-specific
}

// NewFileStore creates a new file-based state store (default behavior for backward compatibility).
// This uses the default "state.json" file and should only be used for single-account setups.
func NewFileStore(dataDir string) (*FileStore, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create data directory %q: %w", dataDir, err)
	}
	return &FileStore{dataDir: dataDir, accountID: ""}, nil
}

// NewFileStoreWithAccountID creates a file-based state store for a specific account.
// Uses state file named "state_<accountID>.json" to support multiple accounts.
func NewFileStoreWithAccountID(dataDir, accountID string) *FileStore {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		// Log error but don't fail - allow state operations to fail later
		// This matches the behavior of existing code
	}
	return &FileStore{dataDir: dataDir, accountID: accountID}
}

// getStatePath returns the state file path for this store's account.
func (fs *FileStore) getStatePath() string {
	if fs.accountID == "" {
		// Default behavior for backward compatibility (single account)
		return filepath.Join(fs.dataDir, "state.json")
	}
	// Per-account state file
	return filepath.Join(fs.dataDir, fmt.Sprintf("state_%s.json", fs.accountID))
}

// Load reads sign state from file.
func (fs *FileStore) Load() (*domain.SignState, error) {
	path := fs.getStatePath()
	data, err := os.ReadFile(path)
	if err != nil {
		// Return empty state if file doesn't exist (not an error on first run)
		if os.IsNotExist(err) {
			return &domain.SignState{}, nil
		}
		return &domain.SignState{}, err
	}
	var state domain.SignState
	if err := json.Unmarshal(data, &state); err != nil {
		return &domain.SignState{}, err
	}
	return &state, nil
}

// Save writes sign state to file atomically (write to temp, then rename).
func (fs *FileStore) Save(state *domain.SignState) error {
	path := fs.getStatePath()
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write temp state file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("failed to rename temp state file: %w", err)
	}
	return nil
}
