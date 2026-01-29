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
	dataDir string
}

// NewFileStore creates a new file-based state store.
func NewFileStore(dataDir string) (*FileStore, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create data directory %q: %w", dataDir, err)
	}
	return &FileStore{dataDir: dataDir}, nil
}

// Load reads sign state from file.
func (fs *FileStore) Load(accountID string) (*domain.SignState, error) {
	path := filepath.Join(fs.dataDir, fmt.Sprintf("state_%s.json", accountID))
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

// Save writes sign state to file.
func (fs *FileStore) Save(accountID string, state *domain.SignState) error {
	path := filepath.Join(fs.dataDir, fmt.Sprintf("state_%s.json", accountID))
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
