package scheduler

import (
	"fmt"
	"time"
)

// IsWithinWindow checks if current time is within the sign window.
func IsWithinWindow(now time.Time, start, end time.Duration) bool {
	current := time.Duration(now.Hour())*time.Hour + time.Duration(now.Minute())*time.Minute
	if start <= end {
		return current >= start && current <= end
	}
	return current >= start || current <= end
}

// FormatWindow formats duration as HH:MM.
func FormatWindow(d time.Duration) string {
	h := int(d / time.Hour)
	m := int((d % time.Hour) / time.Minute)
	return fmt.Sprintf("%02d:%02d", h, m)
}
