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
func (fs *FileStore) Load() (*domain.SignState, error) {
	path := filepath.Join(fs.dataDir, "state.json")
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
	path := filepath.Join(fs.dataDir, "state.json")
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
