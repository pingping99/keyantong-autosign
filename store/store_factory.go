package store

import (
	"crypto/md5"
	"fmt"
)

// StoreFactory creates StateStore instances for different accounts.
type StoreFactory interface {
	CreateStore(accountID string) StateStore
}

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
	return NewFileStoreWithAccountID(f.dataDir, accountID)
}

// GenerateAccountID generates a unique ID for an account (email hash for consistency).
func GenerateAccountID(email string) string {
	hash := md5.Sum([]byte(email))
	return fmt.Sprintf("%x", hash)[:12] // Use first 12 chars of MD5
}
