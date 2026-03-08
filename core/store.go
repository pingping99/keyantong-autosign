package core

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// StateStore handles sign state persistence.
type StateStore interface {
	Load() (*SignState, error)
	Save(state *SignState) error
}

// StoreFactory creates StateStore instances for different accounts.
type StoreFactory interface {
	CreateStore(accountID string) StateStore
}

// --- FileStore implementation ---

// FileStore implements StateStore using local file system.
type FileStore struct {
	dataDir   string
	accountID string
}

// NewFileStore creates a file-based state store for a specific account.
// Uses state file named "state_<accountID>.json" to support multiple accounts.
func NewFileStore(dataDir, accountID string) *FileStore {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		// Log error but don't fail - allow state operations to fail later
	}
	return &FileStore{dataDir: dataDir, accountID: accountID}
}

// getStatePath returns the state file path for this store's account.
func (fs *FileStore) getStatePath() string {
	if fs.accountID == "" {
		return filepath.Join(fs.dataDir, "state.json")
	}
	return filepath.Join(fs.dataDir, fmt.Sprintf("state_%s.json", fs.accountID))
}

// Load reads sign state from file.
func (fs *FileStore) Load() (*SignState, error) {
	path := fs.getStatePath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &SignState{}, nil
		}
		return &SignState{}, err
	}
	var state SignState
	if err := json.Unmarshal(data, &state); err != nil {
		return &SignState{}, err
	}
	return &state, nil
}

// Save writes sign state to file atomically (write to temp, then rename).
func (fs *FileStore) Save(state *SignState) error {
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

// --- FileStoreFactory ---

// FileStoreFactory creates FileStore instances.
type FileStoreFactory struct {
	dataDir string
}

// NewFileStoreFactory creates a new FileStoreFactory.
func NewFileStoreFactory(dataDir string) StoreFactory {
	return &FileStoreFactory{dataDir: dataDir}
}

// CreateStore creates a FileStore for the given account ID.
func (f *FileStoreFactory) CreateStore(accountID string) StateStore {
	return NewFileStore(f.dataDir, accountID)
}

// GenerateAccountID generates a unique ID for an account (email hash for consistency).
func GenerateAccountID(email string) string {
	hash := md5.Sum([]byte(email))
	return fmt.Sprintf("%x", hash)[:12]
}
