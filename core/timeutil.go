package core

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	mathrand "math/rand"
	"time"
)

// Time layout constants for parsing and formatting.
const (
	DateLayout = "2006-01-02"
	TimeLayout = "15:04:05"
)

// rng is a package-level random number generator, seeded with crypto/rand.
var rng = newSecureRandom()

func newSecureRandom() *mathrand.Rand {
	var seed int64
	if err := binary.Read(rand.Reader, binary.BigEndian, &seed); err != nil {
		seed = time.Now().UnixNano()
	} else {
		seed ^= time.Now().UnixNano()
	}
	return mathrand.New(mathrand.NewSource(seed))
}

// SleepWithJitter sleeps for a random duration between 0 and maxSeconds.
func SleepWithJitter(maxSeconds int) time.Duration {
	jitter := time.Duration(rng.Intn(maxSeconds)) * time.Second
	time.Sleep(jitter)
	return jitter
}

// GenerateRandomSignTime generates a random time string (HH:MM:SS) within [startHour:00, endHour:00).
func GenerateRandomSignTime(startHour, endHour int) string {
	effectiveEndHour := endHour - 1
	if effectiveEndHour <= startHour {
		effectiveEndHour = startHour + 1
	}

	totalMinutes := (effectiveEndHour - startHour) * 60
	randomMinutes := rng.Intn(totalMinutes)

	hour := startHour + randomMinutes/60
	minute := randomMinutes % 60
	second := rng.Intn(60)

	return fmt.Sprintf("%02d:%02d:%02d", hour, minute, second)
}

// UpdateSignHistory adds a new sign record and maintains history window (last 14 days).
func UpdateSignHistory(history []SignRecord, date, timeStr string) []SignRecord {
	const maxHistoryDays = 14
	history = append(history, SignRecord{Date: date, Time: timeStr})
	if len(history) > maxHistoryDays {
		history = history[len(history)-maxHistoryDays:]
	}
	return history
}

// ParseDateTime parses a date and time string into a time.Time using the given location.
func ParseDateTime(dateStr, timeStr string, loc *time.Location) (time.Time, error) {
	return time.ParseInLocation(DateLayout+" "+TimeLayout, dateStr+" "+timeStr, loc)
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

// IsWithinHourRange checks if the current hour is within [startHour, endHour).
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
