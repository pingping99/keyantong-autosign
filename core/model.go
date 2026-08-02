package core

const CurrentStateVersion = 2

// SignRecord represents one successful or already-completed sign-in day.
type SignRecord struct {
	Date string `json:"date"`
	Time string `json:"time"`
}

// SignState is a local cache. The remote API response remains the source of truth.
type SignState struct {
	Version         int          `json:"version"`
	LastSignDate    string       `json:"last_sign_date"`
	LastAttemptDate string       `json:"last_attempt_date"`
	LastAttemptTime string       `json:"last_attempt_time"`
	LastResult      string       `json:"last_result"`
	SignHistory     []SignRecord `json:"sign_history"`
	WindowDate      string       `json:"window_date,omitempty"`
	WindowSignTime  string       `json:"window_sign_time,omitempty"`
}
