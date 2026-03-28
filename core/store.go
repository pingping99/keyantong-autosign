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
)

// StateStore 状态存储接口
type StateStore interface {
	Load() (*SignState, error)
	Save(state *SignState) error
}

// StoreFactory 存储工厂接口
type StoreFactory interface {
	CreateStore(accountID string) StateStore
}

// FileStore 基于文件的状态存储实现
type FileStore struct {
	dataDir   string
	accountID string
}

// NewFileStore 创建新的文件存储实例
func NewFileStore(dataDir, accountID string) *FileStore {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		// 忽略错误，后续操作会处理
	}
	return &FileStore{dataDir: dataDir, accountID: accountID}
}

// getStatePath 获取状态文件路径
func (fs *FileStore) getStatePath() string {
	if fs.accountID == "" {
		return filepath.Join(fs.dataDir, "state.json")
	}
	return filepath.Join(fs.dataDir, fmt.Sprintf("state_%s.json", fs.accountID))
}

// Load 从文件加载状态
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

// Save 保存状态到文件（原子写入）
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

// FileStoreFactory 文件存储工厂
type FileStoreFactory struct {
	dataDir string
}

// NewFileStoreFactory 创建新的文件存储工厂
func NewFileStoreFactory(dataDir string) StoreFactory {
	return &FileStoreFactory{dataDir: dataDir}
}

// CreateStore 创建状态存储实例
func (f *FileStoreFactory) CreateStore(accountID string) StateStore {
	return NewFileStore(f.dataDir, accountID)
}

// GenerateAccountID 使用 SHA-256 生成账户唯一标识
// 输入会被规范化（小写、去除空白），确保相同邮箱始终生成相同 ID
func GenerateAccountID(email string) string {
	// 规范化输入
	normalized := strings.ToLower(strings.TrimSpace(email))
	hash := sha256.Sum256([]byte(normalized))
	// 取前 12 字符，足够区分度且便于阅读
	return hex.EncodeToString(hash[:])[:12]
}

// GenerateAccountIDLegacy 使用 MD5 生成账户 ID（仅用于迁移兼容）
// Deprecated: 请使用 GenerateAccountID，此函数仅用于读取旧版状态文件
func GenerateAccountIDLegacy(email string) string {
	hash := md5.Sum([]byte(email))
	return fmt.Sprintf("%x", hash)[:12]
}

// MigrateAccountState 迁移旧版账户状态文件到新版
// 如果旧版文件存在且新版文件不存在，则进行迁移
func MigrateAccountState(dataDir, email string) error {
	oldID := GenerateAccountIDLegacy(email)
	newID := GenerateAccountID(email)

	// 如果 ID 相同则无需迁移
	if oldID == newID {
		return nil
	}

	oldPath := filepath.Join(dataDir, fmt.Sprintf("state_%s.json", oldID))
	newPath := filepath.Join(dataDir, fmt.Sprintf("state_%s.json", newID))

	// 检查旧文件是否存在
	if _, err := os.Stat(oldPath); os.IsNotExist(err) {
		return nil // 无旧文件，无需迁移
	}

	// 检查新文件是否已存在
	if _, err := os.Stat(newPath); err == nil {
		return nil // 新文件已存在，无需迁移
	}

	// 复制旧文件到新路径
	data, err := os.ReadFile(oldPath)
	if err != nil {
		return fmt.Errorf("读取旧状态文件失败: %w", err)
	}

	if err := os.WriteFile(newPath, data, 0o644); err != nil {
		return fmt.Errorf("写入新状态文件失败: %w", err)
	}

	// 可选：删除旧文件（或保留作为备份）
	// os.Remove(oldPath)

	return nil
}
