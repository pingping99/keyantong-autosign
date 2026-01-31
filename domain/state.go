package domain

// SignState tracks sign-in state and attempt history.
type SignState struct {
	LastSignDate    string `json:"last_sign_date"`    // Last successful sign date (YYYY-MM-DD)
	LastAttemptDate string `json:"last_attempt_date"` // Last attempt date (YYYY-MM-DD)
	LastAttemptTime string `json:"last_attempt_time"` // Last attempt time (HH:MM)
	LastResult      string `json:"last_result"`       // success/failed/skip
	WindowDate      string `json:"window_date"`       // Dynamic window date (YYYY-MM-DD)
	WindowStart     string `json:"window_start"`      // Dynamic window start time (HH:MM)
	WindowEnd       string `json:"window_end"`        // Dynamic window end time (HH:MM)
}
