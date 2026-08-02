package core

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"time"
)

const (
	DateLayout = "2006-01-02"
	TimeLayout = "15:04:05"
)

func randomInt(max int64) int64 {
	if max <= 0 {
		return 0
	}
	value, err := rand.Int(rand.Reader, big.NewInt(max))
	if err != nil {
		return time.Now().UnixNano() % max
	}
	return value.Int64()
}

func CalculateJitter(max time.Duration) time.Duration {
	if max <= 0 {
		return 0
	}
	return time.Duration(randomInt(int64(max)))
}

func WaitWithContext(ctx context.Context, duration time.Duration) bool {
	if duration <= 0 {
		return true
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func WaitWithJitter(ctx context.Context, max time.Duration) (time.Duration, bool) {
	jitter := CalculateJitter(max)
	return jitter, WaitWithContext(ctx, jitter)
}

func GenerateRandomSignTime(startHour, endHour int) string {
	startSeconds := int64(startHour * 60 * 60)
	endSeconds := int64(endHour * 60 * 60)
	offset := randomInt(endSeconds - startSeconds)
	totalSeconds := startSeconds + offset
	hour := totalSeconds / 3600
	minute := totalSeconds % 3600 / 60
	second := totalSeconds % 60
	return fmt.Sprintf("%02d:%02d:%02d", hour, minute, second)
}

func UpdateSignHistory(history []SignRecord, date, timeString string) []SignRecord {
	const maxHistoryDays = 14
	for index := range history {
		if history[index].Date == date {
			history[index].Time = timeString
			return history
		}
	}
	history = append(history, SignRecord{Date: date, Time: timeString})
	if len(history) > maxHistoryDays {
		history = history[len(history)-maxHistoryDays:]
	}
	return history
}

func ParseDateTime(dateString, timeString string, location *time.Location) (time.Time, error) {
	return time.ParseInLocation(DateLayout+" "+TimeLayout, dateString+" "+timeString, location)
}

func IsWithinHourRange(now time.Time, location *time.Location, startHour, endHour int) bool {
	hour := now.In(location).Hour()
	return hour >= startHour && hour < endHour
}

func NextWorkingTime(now time.Time, location *time.Location, startHour int) time.Time {
	local := now.In(location)
	if local.Hour() < startHour {
		return time.Date(local.Year(), local.Month(), local.Day(), startHour, 0, 0, 0, location)
	}
	tomorrow := local.AddDate(0, 0, 1)
	return time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), startHour, 0, 0, 0, location)
}

func TimeUntilNextWorkingTime(now time.Time, location *time.Location, startHour int) time.Duration {
	return NextWorkingTime(now, location, startHour).Sub(now)
}
