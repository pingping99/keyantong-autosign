package signer

import (
	"fmt"
	"keyantong/config"
	"keyantong/domain"
	"keyantong/scheduler"
	"keyantong/service"
	"keyantong/store"
	"log"
	"time"
)

const signTolerance = 15 * time.Minute

// Signer defines sign operations for an account.
type Signer interface {
	AttemptSign(now time.Time) error
	ForceSign(now time.Time) error
}

// AccountSigner implements Signer using service.Service.
type AccountSigner struct {
	service    *service.Service
	store      store.StateStore
	cfg        *config.AppConfig
	fileLogger *log.Logger
}

// NewAccountSigner creates a new account signer.
func NewAccountSigner(
	svc *service.Service,
	store store.StateStore,
	cfg *config.AppConfig,
	fileLogger *log.Logger,
) *AccountSigner {
	return &AccountSigner{
		service:    svc,
		store:      store,
		cfg:        cfg,
		fileLogger: fileLogger,
	}
}

// ForceSign performs forced sign-in (used on startup).
func (as *AccountSigner) ForceSign(now time.Time) error {
	log.Printf("强制执行登录与签到")

	// Load existing state to preserve sign history
	state, err := as.store.Load()
	if err != nil {
		log.Printf("无法加载状态，使用空状态: %v", err)
		state = &domain.SignState{}
	}

	if err := as.service.AutoSign(); err != nil {
		return fmt.Errorf("强制签到失败: %w", err)
	}

	today := now.In(as.cfg.Location).Format(config.DateLayout)
	nowTime := now.In(as.cfg.Location).Format(config.TimeLayout)
	state.LastSignDate = today
	state.LastAttemptDate = today
	state.LastAttemptTime = nowTime
	state.LastResult = "success"
	state.SignHistory = scheduler.UpdateSignHistory(state.SignHistory, today, nowTime)

	// Log next sign info after successful force sign
	as.logNextSignInfo(state)

	if err := as.store.Save(state); err != nil {
		log.Printf("保存状态失败: %v", err)
	}
	return nil
}

// AttemptSign attempts to sign in based on smart timing with pattern avoidance.
func (as *AccountSigner) AttemptSign(now time.Time) error {
	nowLocal := now.In(as.cfg.Location)
	today := nowLocal.Format(config.DateLayout)
	nowTime := nowLocal.Format(config.TimeLayout)

	// Load state
	state, err := as.store.Load()
	if err != nil {
		log.Printf("无法加载状态: %v", err)
		state = &domain.SignState{}
	}

	// Check if already signed today
	if state.LastSignDate == today {
		as.fileLogger.Printf("今天 (%s) 已完成签到，跳过", today)
		return nil
	}

	// Generate target sign time if needed (new day)
	if state.TargetSignTime == "" || state.TargetSignDate != today {
		// Use near-full-day range (00:00 - latest schedulable target).
		// Keep a buffer before midnight so the polling loop can still reach the target today.
		start := 0 * time.Hour
		end := as.latestSchedulableTargetEnd()

		targetTime := scheduler.GenerateSmartSignTime(
			start,
			end,
			state.SignHistory,
			today,
		)
		state.TargetSignDate = today
		state.TargetSignTime = targetTime
		log.Printf("今日目标签到时间: %s (基于历史模式规避算法生成)", targetTime)
		if saveErr := as.store.Save(state); saveErr != nil {
			log.Printf("保存目标时间失败: %v", saveErr)
		}
	}

	// Parse target time
	targetDur, err := scheduler.ParseTimeWindow(state.TargetSignTime)
	if err != nil {
		log.Printf("解析目标时间失败: %v", err)
		return nil
	}

	// Check if we're in the sign-in window (target time ± tolerance)
	currentDur := time.Duration(nowLocal.Hour())*time.Hour +
		time.Duration(nowLocal.Minute())*time.Minute +
		time.Duration(nowLocal.Second())*time.Second

	// Allow sign-in within 15 minutes before or after target time
	tolerance := signTolerance
	timeDiff := currentDur - targetDur
	if timeDiff < 0 {
		timeDiff = -timeDiff
	}

	if timeDiff > tolerance {
		if currentDur < targetDur {
			as.fileLogger.Printf("未到目标签到时间 %s (当前 %s)", state.TargetSignTime, nowTime)
			return nil
		}
		// Window missed — fallback: sign immediately if still within configured range
		end := 24 * time.Hour

		if currentDur <= end {
			log.Printf("已错过目标时间 %s，在允许范围内执行补签 (当前 %s)", state.TargetSignTime, nowTime)
		} else {
			as.fileLogger.Printf("已过目标签到时间 %s 且超出允许范围 (当前 %s)", state.TargetSignTime, nowTime)
			return nil
		}
	}

	// Throttle: check if last attempt was too recent
	if state.LastAttemptDate == today && state.LastAttemptTime != "" {
		if as.shouldThrottle(nowLocal, state.LastAttemptTime) {
			as.fileLogger.Printf("距离上次尝试 (%s) 不足 %v，节流跳过",
				state.LastAttemptTime, as.cfg.RetryInterval)
			return nil
		}
	}

	// Update attempt time before trying
	state.LastAttemptDate = today
	state.LastAttemptTime = nowTime

	// Attempt sign
	log.Printf("到达签到时间窗口，开始签到... (目标: %s, 当前: %s)", state.TargetSignTime, nowTime)
	resp, err := as.performSignWithRetry()
	if err != nil {
		state.LastResult = "failed"
		if saveErr := as.store.Save(state); saveErr != nil {
			log.Printf("保存失败状态出错: %v", saveErr)
		}
		return fmt.Errorf("签到失败: %w", err)
	}

	// Record result
	as.recordSignState(resp, state, today, nowTime)
	return nil
}

// shouldThrottle checks if we should skip this attempt based on retry interval.
func (as *AccountSigner) shouldThrottle(now time.Time, lastAttemptTime string) bool {
	// Construct full datetime for comparison
	today := now.Format(config.DateLayout)
	lastAttempt, err := time.ParseInLocation(config.DateLayout+" "+config.TimeLayout,
		today+" "+lastAttemptTime, as.cfg.Location)
	if err != nil {
		return false // If parse fails, don't throttle
	}

	elapsed := now.Sub(lastAttempt)
	return elapsed < as.cfg.RetryInterval
}

// performSignWithRetry attempts sign-in with login retry logic.
func (as *AccountSigner) performSignWithRetry() (*service.SignResponse, error) {
	resp, err := as.service.Sign()
	if err != nil {
		return nil, fmt.Errorf("签到请求失败: %w", err)
	}

	// Handle login required
	if isLoginRequired(resp) {
		log.Printf("会话未登录或已过期，重新登录")
		if err := as.service.Login(); err != nil {
			return nil, fmt.Errorf("登录失败: %w", err)
		}
		resp, err = as.service.Sign()
		if err != nil {
			return nil, fmt.Errorf("登录后签到请求失败: %w", err)
		}
		if isLoginRequired(resp) {
			return nil, fmt.Errorf("重新登录后仍无法签到，请检查账号凭证")
		}
	}

	return resp, nil
}

// isLoginRequired checks if login is needed based on response.
func isLoginRequired(resp *service.SignResponse) bool {
	if resp == nil {
		return true
	}
	// Check if response indicates not logged in
	if resp.Code != 0 && resp.Code != 1 {
		return true
	}
	return false
}

// recordSignState saves sign result to state.
func (as *AccountSigner) recordSignState(resp *service.SignResponse, state *domain.SignState, today, signTime string) {
	if resp == nil {
		log.Printf("签到响应为空")
		return
	}

	switch resp.Code {
	case 0:
		// Sign-in succeeded
		logSignSuccess(resp)
		state.LastSignDate = today
		state.LastResult = "success"
		state.SignHistory = scheduler.UpdateSignHistory(state.SignHistory, today, signTime)
		as.logNextSignInfo(state)
	case 1:
		// Already signed today
		log.Printf("%s", resp.Msg)
		state.LastSignDate = today
		state.LastResult = "success"
		state.SignHistory = scheduler.UpdateSignHistory(state.SignHistory, today, signTime)
		as.logNextSignInfo(state)
	default:
		log.Printf("签到未成功: %s", resp.Msg)
		state.LastResult = "failed"
	}

	if err := as.store.Save(state); err != nil {
		log.Printf("保存签到状态失败: %v", err)
	}
}

// logSignSuccess logs successful sign-in details.
func logSignSuccess(resp *service.SignResponse) {
	log.Printf("✓ %s", resp.Msg)
	log.Printf("  连续签到: %d 次", resp.Data.SignCount)
	log.Printf("  本次获得: %d 积分", resp.Data.SignPoint)
}

// logNextSignInfo logs next sign-in window and estimated time.
func (as *AccountSigner) logNextSignInfo(state *domain.SignState) {
	now := time.Now().In(as.cfg.Location)
	tomorrow := now.AddDate(0, 0, 1)
	tomorrowDate := tomorrow.Format(config.DateLayout)

	// Generate tomorrow's target sign time
	start := 0 * time.Hour
	end := as.latestSchedulableTargetEnd()

	nextTargetTime := scheduler.GenerateSmartSignTime(
		start,
		end,
		state.SignHistory,
		tomorrowDate,
	)

	log.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Printf("下一次签到信息：")
	log.Printf("  签到日期: %s", tomorrowDate)
	log.Printf("  预计时间: %s", nextTargetTime)
	log.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}

// latestSchedulableTargetEnd returns the latest target time that can still be
// reached by the periodic polling loop before date rollover.
func (as *AccountSigner) latestSchedulableTargetEnd() time.Duration {
	fullDay := 24 * time.Hour
	if as.cfg == nil {
		return fullDay - signTolerance
	}

	buffer := as.cfg.CheckInterval - signTolerance
	if buffer < signTolerance {
		buffer = signTolerance
	}
	if buffer >= fullDay {
		return fullDay - signTolerance
	}

	return fullDay - buffer
}
