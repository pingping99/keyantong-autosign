package domain

// SignState tracks the last date the script successfully signed in.
type SignState struct {
	LastSignDate string `json:"last_sign_date"`
}
