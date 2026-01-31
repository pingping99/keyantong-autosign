package scheduler

import (
	"fmt"
	"math/rand"
	"time"
)

// GenerateDynamicWindow generates a random sign window for today.
// Returns start and end time as HH:MM strings.
func GenerateDynamicWindow(rangeStart, rangeEnd, windowSpan time.Duration, seed int64) (string, string) {
	// Use seed based on date to ensure same window for same day
	rng := rand.New(rand.NewSource(seed))

	// Calculate available range for window start
	availableRange := rangeEnd - rangeStart - windowSpan
	if availableRange < 0 {
		availableRange = 0
	}

	// Generate random offset within available range
	offset := time.Duration(rng.Int63n(int64(availableRange) + 1))

	// Calculate window times
	windowStart := rangeStart + offset
	windowEnd := windowStart + windowSpan

	// Ensure window doesn't exceed range end
	if windowEnd > rangeEnd {
		windowEnd = rangeEnd
		windowStart = windowEnd - windowSpan
	}

	return FormatWindow(windowStart), FormatWindow(windowEnd)
}

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

// ParseTimeWindow parses HH:MM string to duration since midnight.
func ParseTimeWindow(timeStr string) (time.Duration, error) {
	t, err := time.Parse("15:04", timeStr)
	if err != nil {
		return 0, err
	}
	return time.Duration(t.Hour())*time.Hour + time.Duration(t.Minute())*time.Minute, nil
}
