package core

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	mathrand "math/rand"
	"time"
)

// 时间格式常量
const (
	DateLayout = "2006-01-02"
	TimeLayout = "15:04:05"
)

// rng 是包级别的随机数生成器，使用 crypto/rand 初始化种子
var rng = newSecureRandom()

// newSecureRandom 创建安全的随机数生成器
func newSecureRandom() *mathrand.Rand {
	var seed int64
	if err := binary.Read(rand.Reader, binary.BigEndian, &seed); err != nil {
		seed = time.Now().UnixNano()
	} else {
		seed ^= time.Now().UnixNano()
	}
	return mathrand.New(mathrand.NewSource(seed))
}

// CalculateJitter 计算随机抖动时间（非阻塞）
// 返回 0 到 maxSeconds 秒之间的随机时长
func CalculateJitter(maxSeconds int) time.Duration {
	if maxSeconds <= 0 {
		return 0
	}
	return time.Duration(rng.Intn(maxSeconds)) * time.Second
}

// SleepWithJitter 带随机抖动的休眠（阻塞）
// Deprecated: 建议使用 CalculateJitter + WaitWithContext 替代
func SleepWithJitter(maxSeconds int) time.Duration {
	jitter := CalculateJitter(maxSeconds)
	time.Sleep(jitter)
	return jitter
}

// WaitWithContext 带 context 的等待，支持取消
// 返回 true 表示等待完成，false 表示被取消
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

// WaitWithJitter 带随机抖动的等待，支持取消
// 返回实际等待的时长和是否被取消
func WaitWithJitter(ctx context.Context, maxSeconds int) (time.Duration, bool) {
	jitter := CalculateJitter(maxSeconds)
	if jitter <= 0 {
		return 0, true
	}
	completed := WaitWithContext(ctx, jitter)
	return jitter, completed
}

// GenerateRandomSignTime 在工作时间范围内生成随机签到时间
// 返回格式为 HH:MM:SS 的时间字符串
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

// UpdateSignHistory 更新签到历史记录，保留最近 14 天
func UpdateSignHistory(history []SignRecord, date, timeStr string) []SignRecord {
	const maxHistoryDays = 14
	history = append(history, SignRecord{Date: date, Time: timeStr})
	if len(history) > maxHistoryDays {
		history = history[len(history)-maxHistoryDays:]
	}
	return history
}

// ParseDateTime 解析日期时间字符串
func ParseDateTime(dateStr, timeStr string, loc *time.Location) (time.Time, error) {
	return time.ParseInLocation(DateLayout+" "+TimeLayout, dateStr+" "+timeStr, loc)
}

// GetTodayString 获取今天的日期字符串
func GetTodayString(now time.Time, loc *time.Location) string {
	return now.In(loc).Format(DateLayout)
}

// GetTimeString 获取时间字符串
func GetTimeString(now time.Time, loc *time.Location) string {
	return now.In(loc).Format(TimeLayout)
}

// GetLocalDateTime 获取本地化的日期和时间字符串
func GetLocalDateTime(now time.Time, loc *time.Location) (date, timeStr string) {
	local := now.In(loc)
	return local.Format(DateLayout), local.Format(TimeLayout)
}

// IsWithinHourRange 检查当前时间是否在指定小时范围内 [startHour, endHour)
func IsWithinHourRange(now time.Time, loc *time.Location, startHour, endHour int) bool {
	hour := now.In(loc).Hour()
	return hour >= startHour && hour < endHour
}

// FormatDuration 格式化时长为人类可读格式
func FormatDuration(d time.Duration) string {
	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60
	if minutes > 0 {
		return fmt.Sprintf("%dm%ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}

// NextWorkingTime 计算下一个工作时间的开始时刻
func NextWorkingTime(now time.Time, loc *time.Location, startHour int) time.Time {
	local := now.In(loc)
	hour := local.Hour()

	// 如果当前时间在工作时间开始之前，返回今天的开始时间
	if hour < startHour {
		return time.Date(local.Year(), local.Month(), local.Day(), startHour, 0, 0, 0, loc)
	}

	// 否则返回明天的开始时间
	tomorrow := local.AddDate(0, 0, 1)
	return time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), startHour, 0, 0, 0, loc)
}

// TimeUntilNextWorkingTime 计算到下一个工作时间的等待时长
func TimeUntilNextWorkingTime(now time.Time, loc *time.Location, startHour int) time.Duration {
	next := NextWorkingTime(now, loc, startHour)
	return next.Sub(now)
}
