package domain

import (
	"crypto/sha1"
	"fmt"
	"strings"
)

// Account represents a login credential pair.
type Account struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	ID       string `json:"-"`
}

// BuildAccountID generates a unique hash ID for an account email.
func BuildAccountID(email string) string {
	normalized := strings.ToLower(strings.TrimSpace(email))
	hash := sha1.Sum([]byte(normalized))
	return fmt.Sprintf("%x", hash)
}

// SetID assigns the computed ID to the account.
func (a *Account) SetID() {
	a.ID = BuildAccountID(a.Email)
}
