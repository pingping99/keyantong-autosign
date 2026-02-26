package scheduler

import (
	"crypto/rand"
	"encoding/binary"
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
