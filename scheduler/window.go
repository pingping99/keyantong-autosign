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

// GenerateRandomSecondDelay generates a random second delay (0-59) for today's sign-in.
// This ensures that even if multiple days have the same minute, seconds will vary.
// The delay value is deterministic per day (same seed for same date) but varies daily.
func GenerateRandomSecondDelay(dateStr string) int {
	rng := newSecureRandom()
	return rng.Intn(60)
}

// SleepWithJitter sleeps for a random duration between 0 and maxSeconds.
func SleepWithJitter(maxSeconds int) time.Duration {
	rng := newSecureRandom()
	jitter := time.Duration(rng.Intn(maxSeconds)) * time.Second
	time.Sleep(jitter)
	return jitter
}

// FormatWindowWithSeconds formats duration as HH:MM:SS.
func FormatWindowWithSeconds(d time.Duration, seconds int) string {
	h := int(d / time.Hour)
	m := int((d % time.Hour) / time.Minute)
	return fmt.Sprintf("%02d:%02d:%02d", h, m, seconds)
}
// UpdateSignHistory adds a new sign record and maintains history window.
func UpdateSignHistory(history []domain.SignRecord, date, time string) []domain.SignRecord {
	const maxHistoryDays = 14

	// Add new record
	newRecord := domain.SignRecord{
		Date: date,
		Time: time,
	}

	history = append(history, newRecord)

	// Keep only recent records (last 14 days)
	if len(history) > maxHistoryDays {
		history = history[len(history)-maxHistoryDays:]
	}

	return history
}