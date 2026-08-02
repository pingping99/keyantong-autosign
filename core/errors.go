package core

import (
	"errors"
	"fmt"
)

type ErrorType int

const (
	ErrTypeUnknown ErrorType = iota
	ErrTypeNetwork
	ErrTypeAuth
	ErrTypeRateLimit
	ErrTypeAlreadySigned
	ErrTypeTimeout
	ErrTypeServer
	ErrTypeConfig
	ErrTypeStore
)

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

func (et ErrorType) IsRetryable() bool {
	switch et {
	case ErrTypeNetwork, ErrTypeRateLimit, ErrTypeTimeout, ErrTypeServer:
		return true
	default:
		return false
	}
}

type SignError struct {
	Type    ErrorType
	Message string
	Cause   error
}

func (e *SignError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Type.String(), e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Type.String(), e.Message)
}

func (e *SignError) Unwrap() error { return e.Cause }

func (e *SignError) Is(target error) bool {
	typed, ok := target.(*SignError)
	return ok && e.Type == typed.Type
}

func NewSignError(errType ErrorType, message string, cause error) *SignError {
	return &SignError{Type: errType, Message: message, Cause: cause}
}

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

func IsRetryableError(err error) bool {
	var signErr *SignError
	return errors.As(err, &signErr) && signErr.Type.IsRetryable()
}

func GetErrorType(err error) ErrorType {
	var signErr *SignError
	if errors.As(err, &signErr) {
		return signErr.Type
	}
	return ErrTypeUnknown
}

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

func IsAlreadySignedError(err error) bool {
	return errors.Is(err, ErrAlreadySigned)
}
