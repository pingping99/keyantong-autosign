package main

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	mathrand "math/rand"
	"time"
)

func newSecureRandom() *mathrand.Rand {
	var seed int64
	if err := binary.Read(rand.Reader, binary.BigEndian, &seed); err != nil {
		seed = time.Now().UnixNano()
	} else {
		seed ^= time.Now().UnixNano()
	}
	return mathrand.New(mathrand.NewSource(seed))
}

func simulateSignWithJitter(checkTime time.Time, maxSeconds int) time.Time {
	rng := newSecureRandom()
	jitter := time.Duration(rng.Intn(maxSeconds)) * time.Second
	return checkTime.Add(jitter)
}

func main() {
	fmt.Println("=== 分钟级抖动测试 ===\n")
	fmt.Println("抖动范围: 0-300秒 (5分钟)")
	fmt.Println()
	
	// 模拟连续7天，每天检查触发时间相同的最坏情况
	fmt.Println("场景: 程序每天都在 10:08:00 检查签到（最坏情况）")
	fmt.Println("=======================================================")
	fmt.Println()
	
	baseTime := time.Date(2026, 2, 20, 10, 8, 0, 0, time.Local)
	
	for day := 0; day < 7; day++ {
		checkTime := baseTime.AddDate(0, 0, day)
		signTime := simulateSignWithJitter(checkTime, 300)
		
		minuteDiff := signTime.Minute() - checkTime.Minute()
		if minuteDiff < 0 {
			minuteDiff += 60
		}
		
		fmt.Printf("2026-02-%02d\n", 20+day)
		fmt.Printf("  检查触发: %s\n", checkTime.Format("15:04:05"))
		fmt.Printf("  实际签到: %s\n", signTime.Format("15:04:05"))
		fmt.Printf("  变化量:   分钟+%d, 秒数=%d\n", minuteDiff, signTime.Second())
		fmt.Println()
	}
	
	fmt.Println("\n=== 时间分布分析 ===")
	fmt.Println("生成30次签到，统计分钟分布：")
	fmt.Println()
	
	minuteCount := make(map[int]int)
	checkTime := time.Date(2026, 2, 26, 10, 8, 0, 0, time.Local)
	
	var times []string
	for i := 0; i < 30; i++ {
		signTime := simulateSignWithJitter(checkTime, 300)
		minute := signTime.Minute()
		minuteCount[minute]++
		times = append(times, signTime.Format("15:04:05"))
	}
	
	// 显示前10个示例
	fmt.Println("前10个签到时间示例:")
	for i := 0; i < 10 && i < len(times); i++ {
		fmt.Printf("  %2d. %s\n", i+1, times[i])
	}
	
	fmt.Println("\n分钟分布统计:")
	for min := 8; min <= 13; min++ {
		count := minuteCount[min]
		if count > 0 {
			bar := ""
			for j := 0; j < count; j++ {
				bar += "█"
			}
			fmt.Printf("  10:%02d - %2d次 %s\n", min, count, bar)
		}
	}
	
	uniqueMinutes := len(minuteCount)
	fmt.Printf("\n✓ 覆盖了 %d 个不同的分钟数（理论最多6个：08-13）\n", uniqueMinutes)
	
	fmt.Println("\n=== 对比：旧机制 vs 新机制 ===")
	fmt.Println()
	fmt.Println("旧机制（60秒抖动）:")
	fmt.Println("  检查: 10:08:00 → 签到范围: 10:08:00 - 10:08:59")
	fmt.Println("  特点: 分钟固定(10:08)，只有秒数变化")
	fmt.Println()
	fmt.Println("新机制（300秒抖动）:")
	fmt.Println("  检查: 10:08:00 → 签到范围: 10:08:00 - 10:13:00")
	fmt.Println("  特点: 分钟和秒数都会变化，分布在6个不同分钟")
	fmt.Println()
	
	fmt.Println("=== 与您的历史数据对比 ===")
	fmt.Println()
	fmt.Println("您之前看到的数据（旧窗口机制）:")
	fmt.Println("  2026-02-26 17:08:43")
	fmt.Println("  2026-02-25 05:08:43  ← 完全相同!")
	fmt.Println("  2026-02-24 02:08:43  ← 完全相同!")
	fmt.Println()
	fmt.Println("新机制预期数据（分钟级抖动）:")
	for day := 24; day <= 26; day++ {
		checkTime := time.Date(2026, 2, day, 10, 8, 0, 0, time.Local)
		signTime := simulateSignWithJitter(checkTime, 300)
		fmt.Printf("  2026-02-%02d %s  ← 分钟和秒数都不同\n", day, signTime.Format("15:04:05"))
	}
	
	fmt.Println("\n✅ 分钟级抖动已实现！")
	fmt.Println("✓ 分钟会在 5 分钟范围内随机变化")
	fmt.Println("✓ 秒数也会随机变化")
	fmt.Println("✓ 即使检查时间相同，签到时间也会显著不同")
}
