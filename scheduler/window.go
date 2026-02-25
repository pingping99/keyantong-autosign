package scheduler

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"keyantong/config"
	"keyantong/domain"
	"math"
	mathrand "math/rand"
	"time"
)

const (
	// HistoryWindowDays is how many days of history to consider for pattern avoidance
	HistoryWindowDays = 14
	// MinTimeBetweenSigns is minimum time difference from recent signs (in hours)
	MinTimeBetweenSigns = 2.0
)

// GenerateSmartSignTime generates a randomized sign time that avoids historical patterns.
// Returns target sign time as HH:MM:SS string.
func GenerateSmartSignTime(rangeStart, rangeEnd time.Duration, history []domain.SignRecord, today string) string {
	// Use cryptographic random for true randomness
	rng := newSecureRandom()

	// Calculate available range
	availableRange := rangeEnd - rangeStart
	if availableRange <= 0 {
		return FormatWindowWithSeconds(rangeStart, rng.Intn(60))
	}

	// Generate candidate times (try up to 20 times to find good time)
	maxAttempts := 20
	var bestTime time.Duration
	bestScore := -1.0

	for attempt := 0; attempt < maxAttempts; attempt++ {
		// Generate random time within range
		offset := time.Duration(rng.Int63n(int64(availableRange)))
		candidateTime := rangeStart + offset

		// Calculate score based on historical pattern avoidance
		score := scoreSignTime(candidateTime, history, today)

		if score > bestScore {
			bestScore = score
			bestTime = candidateTime
		}

		// If we found a perfect score, use it immediately
		if score >= 1.0 {
			break
		}
	}

	// Add random jitter (0-60 minutes) to ensure minutes are also random
	jitter := time.Duration(rng.Int63n(int64(60 * time.Minute)))
	finalTime := bestTime + jitter

	// Ensure final time doesn't exceed range
	if finalTime > rangeEnd {
		finalTime = rangeEnd - time.Duration(rng.Int63n(int64(60*time.Minute)))
	}
	if finalTime < rangeStart {
		finalTime = rangeStart
	}

	// Generate random seconds (0-59) to ensure variety
	randomSeconds := rng.Intn(60)

	return FormatWindowWithSeconds(finalTime, randomSeconds)
}

// scoreSignTime calculates how good a candidate time is (higher = better).
// Avoids times similar to recent sign-ins.
func scoreSignTime(candidateTime time.Duration, history []domain.SignRecord, today string) float64 {
	if len(history) == 0 {
		return 1.0 // No history, all times are equally good
	}

	todayDate, todayErr := time.Parse(config.DateLayout, today)
	candidateHours := candidateTime.Hours()
	candidateMinutes := (candidateTime.Minutes() - candidateHours*60)
	totalScore := 0.0

	// Check against recent history (last 7 days weighted more)
	for _, record := range history {
		histTime, err := ParseTimeWindow(record.Time)
		if err != nil {
			continue
		}

		histHours := histTime.Hours()
		histMinutes := (histTime.Minutes() - histHours*60)

		// Calculate time difference in hours (handle day wrap)
		hourDiff := math.Abs(candidateHours - histHours)
		if hourDiff > 12 {
			hourDiff = 24 - hourDiff // Handle wrap-around (e.g., 23:00 vs 01:00)
		}

		// Also consider minute difference for more precision
		minuteDiff := math.Abs(candidateMinutes - histMinutes)
		totalDiff := hourDiff + (minuteDiff / 60.0)

		// Calculate actual days ago from record date
		weight := 1.0
		if todayErr == nil {
			recordDate, err := time.Parse(config.DateLayout, record.Date)
			if err == nil {
				daysAgo := int(todayDate.Sub(recordDate).Hours() / 24)
				if daysAgo <= 7 {
					weight = 2.0
				}
				// Increase weight for very recent days
				if daysAgo <= 3 {
					weight = 3.0
				}
			}
		}

		// Score based on time difference
		// Further away = higher score
		timeScore := totalDiff / 12.0 // Normalize to 0-1 range (12 hours = perfect)
		if totalDiff < MinTimeBetweenSigns {
			timeScore = 0.0 // Penalize times too close to history
		}

		totalScore += timeScore * weight
	}

	// Normalize by history size
	return totalScore / float64(len(history))
}

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

// UpdateSignHistory adds a new sign record and maintains history window.
func UpdateSignHistory(history []domain.SignRecord, date, time string) []domain.SignRecord {
	// Add new record
	newRecord := domain.SignRecord{
		Date: date,
		Time: time,
	}

	history = append(history, newRecord)

	// Keep only recent records (last HistoryWindowDays days)
	if len(history) > HistoryWindowDays {
		history = history[len(history)-HistoryWindowDays:]
	}

	return history
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

// FormatWindowWithSeconds formats duration as HH:MM:SS.
func FormatWindowWithSeconds(d time.Duration, seconds int) string {
	h := int(d / time.Hour)
	m := int((d % time.Hour) / time.Minute)
	return fmt.Sprintf("%02d:%02d:%02d", h, m, seconds)
}

// ParseTimeWindow parses HH:MM or HH:MM:SS string to duration since midnight.
func ParseTimeWindow(timeStr string) (time.Duration, error) {
	// Try parsing with seconds first
	t, err := time.Parse("15:04:05", timeStr)
	if err != nil {
		// Fall back to HH:MM format for backward compatibility
		t, err = time.Parse("15:04", timeStr)
		if err != nil {
			return 0, err
		}
	}
	return time.Duration(t.Hour())*time.Hour + 
		time.Duration(t.Minute())*time.Minute + 
		time.Duration(t.Second())*time.Second, nil
}

// SleepWithJitter sleeps for a random duration between 0 and maxSeconds.
func SleepWithJitter(maxSeconds int) time.Duration {
	rng := newSecureRandom()
	jitter := time.Duration(rng.Intn(maxSeconds)) * time.Second
	time.Sleep(jitter)
	return jitter
}
