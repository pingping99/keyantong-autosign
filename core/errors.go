package core

import (
	"errors"
	"fmt"
)

// ErrorType 定义错误类型枚举
type ErrorType int

const (
	// ErrTypeUnknown 未知错误
	ErrTypeUnknown ErrorType = iota
	// ErrTypeNetwork 网络错误（可重试）
	ErrTypeNetwork
	// ErrTypeAuth 认证错误（需要重新登录）
	ErrTypeAuth
	// ErrTypeRateLimit 频率限制（需要等待后重试）
	ErrTypeRateLimit
	// ErrTypeAlreadySigned 今日已签到
	ErrTypeAlreadySigned
	// ErrTypeTimeout 超时错误
	ErrTypeTimeout
	// ErrTypeServer 服务器错误
	ErrTypeServer
	// ErrTypeConfig 配置错误
	ErrTypeConfig
	// ErrTypeStore 存储错误
	ErrTypeStore
)

// String 返回错误类型的字符串表示
func (et ErrorType) String() string {
	switch et {
	case ErrTypeNetwork:
		return "网络错误"
	case ErrTypeAuth:
		return "认证错误"
	case ErrTypeRateLimit:
		return "频率限制"
	case ErrTypeAlreadySigned:
		return "今日已签到"
	case ErrTypeTimeout:
		return "超时错误"
	case ErrTypeServer:
		return "服务器错误"
	case ErrTypeConfig:
		return "配置错误"
	case ErrTypeStore:
		return "存储错误"
	default:
		return "未知错误"
	}
}

// IsRetryable 判断该类型错误是否可重试
func (et ErrorType) IsRetryable() bool {
	switch et {
	case ErrTypeNetwork, ErrTypeRateLimit, ErrTypeTimeout, ErrTypeServer:
		return true
	default:
		return false
	}
}

// SignError 签到错误结构体
type SignError struct {
	Type    ErrorType // 错误类型
	Message string    // 错误消息
	Cause   error     // 原始错误
}

// Error 实现 error 接口
func (e *SignError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Type.String(), e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Type.String(), e.Message)
}

// Unwrap 支持 errors.Unwrap
func (e *SignError) Unwrap() error {
	return e.Cause
}

// Is 支持 errors.Is 比较错误类型
func (e *SignError) Is(target error) bool {
	t, ok := target.(*SignError)
	if !ok {
		return false
	}
	return e.Type == t.Type
}

// NewSignError 创建新的签到错误
func NewSignError(errType ErrorType, message string, cause error) *SignError {
	return &SignError{
		Type:    errType,
		Message: message,
		Cause:   cause,
	}
}

// NewSignErrorf 创建带格式化消息的签到错误
func NewSignErrorf(errType ErrorType, cause error, format string, args ...interface{}) *SignError {
	return &SignError{
		Type:    errType,
		Message: fmt.Sprintf(format, args...),
		Cause:   cause,
	}
}

// 预定义的哨兵错误，用于 errors.Is 比较
var (
	ErrNetwork       = &SignError{Type: ErrTypeNetwork}
	ErrAuth          = &SignError{Type: ErrTypeAuth}
	ErrRateLimit     = &SignError{Type: ErrTypeRateLimit}
	ErrAlreadySigned = &SignError{Type: ErrTypeAlreadySigned}
	ErrTimeout       = &SignError{Type: ErrTypeTimeout}
	ErrServer        = &SignError{Type: ErrTypeServer}
	ErrConfig        = &SignError{Type: ErrTypeConfig}
	ErrStore         = &SignError{Type: ErrTypeStore}
)

// IsRetryableError 判断错误是否可重试
func IsRetryableError(err error) bool {
	var signErr *SignError
	if errors.As(err, &signErr) {
		return signErr.Type.IsRetryable()
	}
	return false
}

// GetErrorType 从错误中提取错误类型
func GetErrorType(err error) ErrorType {
	var signErr *SignError
	if errors.As(err, &signErr) {
		return signErr.Type
	}
	return ErrTypeUnknown
}

// WrapError 包装错误为 SignError
// 如果已经是 SignError 则直接返回，否则包装为指定类型
func WrapError(err error, errType ErrorType, message string) error {
	if err == nil {
		return nil
	}
	var signErr *SignError
	if errors.As(err, &signErr) {
		return signErr
	}
	return NewSignError(errType, message, err)
}
