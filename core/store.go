package core

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type StateStore interface {
	Load(accountID string) (*SignState, error)
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

func (store *FileStore) Load(accountID string) (*SignState, error) {
	path := store.statePath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &SignState{AccountID: accountID}, nil
		}
		return nil, fmt.Errorf("read state: %w", err)
	}

	var state SignState
	if err := json.Unmarshal(data, &state); err != nil {
		quarantinePath := store.quarantinePath("corrupt")
		if renameErr := os.Rename(path, quarantinePath); renameErr != nil {
			return nil, fmt.Errorf("decode state: %w; quarantine failed: %v", err, renameErr)
		}
		return nil, fmt.Errorf("decode state: %w; corrupt file moved to %s", err, quarantinePath)
	}

	if state.AccountID != "" && state.AccountID != accountID {
		quarantinePath := store.quarantinePath("account-mismatch")
		if err := os.Rename(path, quarantinePath); err != nil {
			return nil, fmt.Errorf("quarantine state for another account: %w", err)
		}
		return &SignState{AccountID: accountID}, nil
	}
	if state.AccountID == "" {
		state.AccountID = accountID
	}
	return &state, nil
}

func (store *FileStore) Save(state *SignState) error {
	if state == nil {
		return fmt.Errorf("state must not be nil")
	}
	if state.AccountID == "" {
		return fmt.Errorf("state account_id must not be empty")
	}

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
	if err := replaceStateFile(temporaryPath, store.statePath()); err != nil {
		return fmt.Errorf("replace state: %w", err)
	}

	if runtime.GOOS != "windows" {
		if directory, err := os.Open(store.dataDir); err == nil {
			_ = directory.Sync()
			_ = directory.Close()
		}
	}
	return nil
}

// replaceStateFile uses rename-based replacement on Unix. Windows does not
// guarantee the same atomic semantics, so it uses a recoverable backup swap.
func replaceStateFile(source, target string) error {
	if runtime.GOOS != "windows" {
		return os.Rename(source, target)
	}

	backup := target + ".bak"
	_ = os.Remove(backup)
	if _, err := os.Stat(target); err == nil {
		if err := os.Rename(target, backup); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	if err := os.Rename(source, target); err != nil {
		if _, backupErr := os.Stat(backup); backupErr == nil {
			_ = os.Rename(backup, target)
		}
		return err
	}
	_ = os.Remove(backup)
	return nil
}

func (store *FileStore) quarantinePath(reason string) string {
	return fmt.Sprintf("%s.%s-%d", store.statePath(), reason, time.Now().UnixNano())
}

// MigrateSingleAccountState copies the matching legacy account state and marks
// it untrusted so the next valid attempt confirms the remote state again.
func MigrateSingleAccountState(dataDir, email string) error {
	target := filepath.Join(dataDir, "state.json")
	if _, err := os.Stat(target); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	accountID := GenerateAccountID(email)
	for _, legacyID := range []string{accountID, generateLegacyAccountID(email)} {
		source := filepath.Join(dataDir, fmt.Sprintf("state_%s.json", legacyID))
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
		state.AccountID = accountID
		state.Version = 0

		encoded, err := json.MarshalIndent(&state, "", "  ")
		if err != nil {
			return fmt.Errorf("encode migrated state: %w", err)
		}
		if err := os.WriteFile(target, encoded, 0o600); err != nil {
			return fmt.Errorf("write migrated state: %w", err)
		}
		return nil
	}
	return nil
}

func GenerateAccountID(email string) string {
	normalized := strings.ToLower(strings.TrimSpace(email))
	hash := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(hash[:])[:12]
}

func generateLegacyAccountID(email string) string {
	hash := md5.Sum([]byte(email))
	return fmt.Sprintf("%x", hash)[:12]
}
