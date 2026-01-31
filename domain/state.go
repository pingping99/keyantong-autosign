package domain

// SignState tracks sign-in state and attempt history.
type SignState struct {
	LastSignDate    string `json:"last_sign_date"`    // Last successful sign date (YYYY-MM-DD)
	LastAttemptDate string `json:"last_attempt_date"` // Last attempt date (YYYY-MM-DD)
	LastAttemptTime string `json:"last_attempt_time"` // Last attempt time (HH:MM)
	LastResult      string `json:"last_result"`       // success/failed/skip
}
