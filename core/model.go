package core

const CurrentStateVersion = 3

// SignRecord represents one successful or already-completed sign-in day.
type SignRecord struct {
	Date string `json:"date"`
	Time string `json:"time"`
}

// SignState is a local cache. AccountID prevents one account from trusting
// another account's sign-in state when the same data directory is reused.
type SignState struct {
	Version         int          `json:"version"`
	AccountID       string       `json:"account_id"`
	LastSignDate    string       `json:"last_sign_date"`
	LastAttemptDate string       `json:"last_attempt_date"`
	LastAttemptTime string       `json:"last_attempt_time"`
	LastScheduledAt string       `json:"last_scheduled_at,omitempty"`
	LastRequestAt   string       `json:"last_request_at,omitempty"`
	LastCompletedAt string       `json:"last_completed_at,omitempty"`
	LastResult      string       `json:"last_result"`
	SignHistory     []SignRecord `json:"sign_history"`
	WindowDate      string       `json:"window_date,omitempty"`
	WindowSignTime  string       `json:"window_sign_time,omitempty"`
}
