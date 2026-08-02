package core

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type StateStore interface {
	Load() (*SignState, error)
	Save(state *SignState) error
}

type FileStore struct {
	dataDir string
}

func NewFileStore(dataDir string) (*FileStore, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}
	return &FileStore{dataDir: dataDir}, nil
}

func (store *FileStore) statePath() string {
	return filepath.Join(store.dataDir, "state.json")
}

func (store *FileStore) Load() (*SignState, error) {
	path := store.statePath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &SignState{}, nil
		}
		return nil, fmt.Errorf("read state: %w", err)
	}
	var state SignState
	if err := json.Unmarshal(data, &state); err != nil {
		corruptPath := fmt.Sprintf("%s.corrupt-%s", path, time.Now().Format("20060102-150405"))
		if renameErr := os.Rename(path, corruptPath); renameErr != nil {
			return nil, fmt.Errorf("decode state: %w; quarantine failed: %v", err, renameErr)
		}
		return nil, fmt.Errorf("decode state: %w; corrupt file moved to %s", err, corruptPath)
	}
	return &state, nil
}

func (store *FileStore) Save(state *SignState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode state: %w", err)
	}

	temporary, err := os.CreateTemp(store.dataDir, ".state-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary state: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)

	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return fmt.Errorf("set state permissions: %w", err)
	}
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return fmt.Errorf("write state: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return fmt.Errorf("sync state: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close state: %w", err)
	}
	if err := os.Rename(temporaryPath, store.statePath()); err != nil {
		return fmt.Errorf("replace state: %w", err)
	}
	return nil
}

// MigrateSingleAccountState copies a previous account-specific state file to state.json.
func MigrateSingleAccountState(dataDir, email string) error {
	target := filepath.Join(dataDir, "state.json")
	if _, err := os.Stat(target); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	for _, accountID := range []string{generateAccountID(email), generateLegacyAccountID(email)} {
		source := filepath.Join(dataDir, fmt.Sprintf("state_%s.json", accountID))
		data, err := os.ReadFile(source)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("read legacy state: %w", err)
		}
		var state SignState
		if err := json.Unmarshal(data, &state); err != nil {
			return fmt.Errorf("legacy state %s is invalid: %w", source, err)
		}
		if err := os.WriteFile(target, data, 0o600); err != nil {
			return fmt.Errorf("write migrated state: %w", err)
		}
		return nil
	}
	return nil
}

func generateAccountID(email string) string {
	normalized := strings.ToLower(strings.TrimSpace(email))
	hash := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(hash[:])[:12]
}

func generateLegacyAccountID(email string) string {
	hash := md5.Sum([]byte(email))
	return fmt.Sprintf("%x", hash)[:12]
}
