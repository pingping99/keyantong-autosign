package scheduler

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"keyantong/domain"
	mathrand "math/rand"
	"time"
)

// newSecureRandom creates a cryptographically secure random number generator.
func newSecureRandom() *mathrand.Rand {
	var seed int64
	// Use crypto/rand as primary entropy source
	if err := binary.Read(rand.Reader, binary.BigEndian, &seed); err != nil {
		// Fallback to time-based seed if crypto/rand fails (unlikely but safe)
		seed = time.Now().UnixNano()
	} else {
		// Mix in time to ensure uniqueness even if crypto/rand is somehow static (e.g. VM snapshot/container)
		seed ^= time.Now().UnixNano()
	}
	return mathrand.New(mathrand.NewSource(seed))
}

// SleepWithJitter sleeps for a random duration between 0 and maxSeconds.
func SleepWithJitter(maxSeconds int) time.Duration {
	rng := newSecureRandom()
	jitter := time.Duration(rng.Intn(maxSeconds)) * time.Second
	time.Sleep(jitter)
	return jitter
}

// GenerateRandomSignTime generates a random time string (HH:MM:SS) within [startHour:00, endHour:00).
// A 1-hour buffer is reserved before endHour to ensure the sign-in can be captured
// by periodic checks before the working hours cutoff.
func GenerateRandomSignTime(startHour, endHour int) string {
	// Reserve 1-hour buffer before endHour for safety
	effectiveEndHour := endHour - 1
	if effectiveEndHour <= startHour {
		effectiveEndHour = startHour + 1
	}

	rng := newSecureRandom()
	totalMinutes := (effectiveEndHour - startHour) * 60
	randomMinutes := rng.Intn(totalMinutes)

	hour := startHour + randomMinutes/60
	minute := randomMinutes % 60
	second := rng.Intn(60)

	return fmt.Sprintf("%02d:%02d:%02d", hour, minute, second)
}

// UpdateSignHistory adds a new sign record and maintains history window.
func UpdateSignHistory(history []domain.SignRecord, date, timeStr string) []domain.SignRecord {
	const maxHistoryDays = 14

	// Add new record
	newRecord := domain.SignRecord{
		Date: date,
		Time: timeStr,
	}

	history = append(history, newRecord)

	// Keep only recent records (last 14 days)
	if len(history) > maxHistoryDays {
		history = history[len(history)-maxHistoryDays:]
	}

	return history
}
