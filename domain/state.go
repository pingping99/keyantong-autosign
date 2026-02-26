package domain

// SignRecord represents a single sign-in record.
type SignRecord struct {
	Date string `json:"date"` // Sign date (YYYY-MM-DD)
	Time string `json:"time"` // Sign time (HH:MM:SS)
}

// SignState tracks sign-in state and attempt history.
type SignState struct {
	LastSignDate    string       `json:"last_sign_date"`    // Last successful sign date (YYYY-MM-DD)
	LastAttemptDate string       `json:"last_attempt_date"` // Last attempt date (YYYY-MM-DD)
	LastAttemptTime string       `json:"last_attempt_time"` // Last attempt time (HH:MM:SS)
	LastResult      string       `json:"last_result"`       // success/failed/skip
	SignHistory     []SignRecord `json:"sign_history"`      // Recent sign history (last 14 days)
}
