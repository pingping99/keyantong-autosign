package domain

// SignRecord represents a single sign-in record.
type SignRecord struct {
	Date string `json:"date"` // Sign date (YYYY-MM-DD)
	Time string `json:"time"` // Sign time (HH:MM)
}

// SignState tracks sign-in state and attempt history.
type SignState struct {
	LastSignDate    string       `json:"last_sign_date"`    // Last successful sign date (YYYY-MM-DD)
	LastAttemptDate string       `json:"last_attempt_date"` // Last attempt date (YYYY-MM-DD)
	LastAttemptTime string       `json:"last_attempt_time"` // Last attempt time (HH:MM)
	LastResult      string       `json:"last_result"`       // success/failed/skip
	TargetSignDate  string       `json:"target_sign_date"`  // Date the target time was generated for (YYYY-MM-DD)
	TargetSignTime  string       `json:"target_sign_time"`  // Target sign time for today (HH:MM)
	SignHistory     []SignRecord `json:"sign_history"`      // Recent sign history (last 14 days)
}
