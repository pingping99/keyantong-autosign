package scheduler

import (
	"fmt"
	"time"
)

// Time layout constants for parsing and formatting
const (
	DateLayout = "2006-01-02"
	TimeLayout = "15:04:05"
)

// ParseDateTime parses a date and time string into a time.Time using the given location.
// dateStr should be in format "2006-01-02"
// timeStr should be in format "15:04:05"
func ParseDateTime(dateStr, timeStr string, loc *time.Location) (time.Time, error) {
	combinedStr := dateStr + " " + timeStr
	return time.ParseInLocation(DateLayout+" "+TimeLayout, combinedStr, loc)
}

// GetTodayString returns today's date as a string in DateLayout format.
func GetTodayString(now time.Time, loc *time.Location) string {
	return now.In(loc).Format(DateLayout)
}

// GetTimeString returns the time portion as a string in TimeLayout format.
func GetTimeString(now time.Time, loc *time.Location) string {
	return now.In(loc).Format(TimeLayout)
}

// GetLocalDateTime returns localized date and time strings.
func GetLocalDateTime(now time.Time, loc *time.Location) (date, timeStr string) {
	local := now.In(loc)
	return local.Format(DateLayout), local.Format(TimeLayout)
}

// IsWithinHourRange checks if the current hour is within the specified range [startHour, endHour).
func IsWithinHourRange(now time.Time, loc *time.Location, startHour, endHour int) bool {
	hour := now.In(loc).Hour()
	return hour >= startHour && hour < endHour
}

// FormatDuration formats a duration for display (e.g., "5m30s").
func FormatDuration(d time.Duration) string {
	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60
	if minutes > 0 {
		return fmt.Sprintf("%dm%ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}
