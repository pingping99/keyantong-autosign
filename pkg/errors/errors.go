package errors

import "fmt"

// Domain-level error definitions

var (
	// ErrLoginRequired indicates user must re-login (session expired).
	ErrLoginRequired = fmt.Errorf("login required: session expired or not authenticated")

	// ErrNetworkError indicates a network connectivity issue.
	ErrNetworkError = fmt.Errorf("network error")

	// ErrInvalidResponse indicates the server returned an invalid response.
	ErrInvalidResponse = fmt.Errorf("invalid response from server")

	// ErrAuthenticationFailed indicates invalid credentials.
	ErrAuthenticationFailed = fmt.Errorf("authentication failed: invalid email or password")
)

// IsLoginRequired checks if the error indicates login is required.
func IsLoginRequired(err error) bool {
	return err != nil && fmt.Sprint(err) == fmt.Sprint(ErrLoginRequired)
}

// IsNetworkError checks if the error indicates a network issue.
func IsNetworkError(err error) bool {
	return err != nil && fmt.Sprint(err) == fmt.Sprint(ErrNetworkError)
}

// IsAuthenticationFailed checks if authentication failed.
func IsAuthenticationFailed(err error) bool {
	return err != nil && fmt.Sprint(err) == fmt.Sprint(ErrAuthenticationFailed)
}
