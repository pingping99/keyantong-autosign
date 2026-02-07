package domain

// Account represents a login credential pair.
type Account struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}
