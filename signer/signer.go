package signer

import (
	"context"
	"errors"
	"fmt"
	"keyantong/config"
	"keyantong/domain"
	"keyantong/scheduler"
	"keyantong/service"
	"keyantong/store"
	"log"
	"time"
)

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

	// Check current time and warn if outside recommended hours
	nowLocal := now.In(as.cfg.Location)
	currentHour := nowLocal.Hour()
	if currentHour < as.cfg.EarlyHourThreshold || currentHour >= as.cfg.LateHourThreshold {
		log.Printf("⚠️  当前时间不在推荐的签到时间范围内 (%02d:00-%02d:00)，但仍将执行强制签到", as.cfg.EarlyHourThreshold, as.cfg.LateHourThreshold)
	}

	// Add jitter (up to 5 minutes) to ensure startup time correlation is broken
	jitter := scheduler.SleepWithJitter(300)
	log.Printf("执行前随机抖动: %v", jitter)

	ctx := context.Background()
	log.Printf("正在登录...")
	if err := as.service.Login(ctx); err != nil {
		return fmt.Errorf("登录失败: %w", err)
	}
	log.Printf("✓ 登录成功")

	log.Printf("正在签到...")
	resp, err := as.service.Sign(ctx)
	if err != nil {
		return fmt.Errorf("强制签到失败: %w", err)
	}

	// Refresh time to reflect actual sign time
	now = time.Now().In(as.cfg.Location)
	today := now.Format(scheduler.DateLayout)
	nowTime := now.Format(scheduler.TimeLayout)

	// Log result
	logSignSuccess(resp)

	state.LastSignDate = today
	state.LastAttemptDate = today
	state.LastAttemptTime = nowTime
	state.LastResult = "success"
	state.SignHistory = scheduler.UpdateSignHistory(state.SignHistory, today, nowTime)

	// Log next sign info after successful force sign
	as.logNextSignInfo()

	if err := as.store.Save(state); err != nil {
		log.Printf("保存状态失败: %v", err)
	}
	return nil
}

// AttemptSign attempts to sign in if not already signed today.
// Uses a simple daily sign-in approach without time windows.
func (as *AccountSigner) AttemptSign(now time.Time) error {
	nowLocal := now.In(as.cfg.Location)
	today := nowLocal.Format(scheduler.DateLayout)
	nowTime := nowLocal.Format(scheduler.TimeLayout)

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

	// Check if current time is within allowed sign-in hours
	currentHour := nowLocal.Hour()

	// Time window logic:
	// - [EarlyHourThreshold, LateHourThreshold): Normal sign-in window (preferred)
	// - [LateHourThreshold, 24): Late window (sign immediately to avoid missing today)
	// - [0, EarlyHourThreshold): Too early, skip and wait for morning window
	if currentHour < as.cfg.EarlyHourThreshold {
		as.fileLogger.Printf("当前时间 %s 过早 (< %02d:00)，等待进入签到时间窗口", nowTime, as.cfg.EarlyHourThreshold)
		return nil
	}

	// If it's late but haven't signed today, log warning but proceed
	if currentHour >= as.cfg.LateHourThreshold {
		log.Printf("⚠️  当前时间 %s 较晚 (>= %02d:00)，但今日尚未签到，立即执行以避免遗漏", nowTime, as.cfg.LateHourThreshold)
	}

	// Throttle: check if last attempt was too recent (prevent API spam)
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

	// Attempt sign-in
	log.Printf("执行签到... (当前: %s)", nowTime)

	// Add execution jitter (up to 5 minutes) to minimize detection patterns and vary both minutes and seconds
	jitter := scheduler.SleepWithJitter(300)
	log.Printf("执行前随机抖动: %v", jitter)

	resp, err := as.performSignWithRetry()
	if err != nil {
		state.LastResult = "failed"
		if saveErr := as.store.Save(state); saveErr != nil {
			log.Printf("保存失败状态出错: %v", saveErr)
		}
		return fmt.Errorf("签到失败: %w", err)
	}

	// Refresh time to reflect actual sign time
	nowLocal = time.Now().In(as.cfg.Location)
	today = nowLocal.Format(scheduler.DateLayout)
	nowTime = nowLocal.Format(scheduler.TimeLayout)

	// Record result
	as.recordSignState(resp, state, today, nowTime)
	return nil
}

// shouldThrottle checks if we should skip this attempt based on retry interval.
func (as *AccountSigner) shouldThrottle(now time.Time, lastAttemptTime string) bool {
	// Construct full datetime for comparison
	today := now.Format(scheduler.DateLayout)
	lastAttempt, err := scheduler.ParseDateTime(today, lastAttemptTime, as.cfg.Location)
	if err != nil {
		return false // If parse fails, don't throttle
	}

	elapsed := now.Sub(lastAttempt)
	return elapsed < as.cfg.RetryInterval
}

// performSignWithRetry attempts sign-in with login retry logic.
func (as *AccountSigner) performSignWithRetry() (*service.SignResponse, error) {
	ctx := context.Background()

	resp, err := as.service.Sign(ctx)
	if err != nil {
		// Check if session expired (HTTP redirect detected)
		if errors.Is(err, service.ErrLoginRequired) {
			log.Printf("会话未登录或已过期，重新登录")
			if loginErr := as.service.Login(ctx); loginErr != nil {
				return nil, fmt.Errorf("登录失败: %w", loginErr)
			}
			resp, err = as.service.Sign(ctx)
			if err != nil {
				return nil, fmt.Errorf("登录后签到请求失败: %w", err)
			}
		} else {
			return nil, fmt.Errorf("签到请求失败: %w", err)
		}
	}

	// Also check response code for legacy login-required indication
	if isLoginRequiredByCode(resp) {
		log.Printf("响应码表明需要重新登录，重新登录")
		if loginErr := as.service.Login(ctx); loginErr != nil {
			return nil, fmt.Errorf("登录失败: %w", loginErr)
		}
		resp, err = as.service.Sign(ctx)
		if err != nil {
			return nil, fmt.Errorf("登录后签到请求失败: %w", err)
		}
		if isLoginRequiredByCode(resp) {
			return nil, fmt.Errorf("重新登录后仍无法签到，请检查账号凭证")
		}
	}

	return resp, nil
}

// isLoginRequiredByCode checks if login is needed based on response code.
// Only triggers for unexpected codes (not 0=success, not 1=already signed).
func isLoginRequiredByCode(resp *service.SignResponse) bool {
	if resp == nil {
		return true
	}
	return resp.Code != 0 && resp.Code != 1
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
		as.logNextSignInfo()
	case 1:
		// Already signed today
		log.Printf("%s", resp.Msg)
		state.LastSignDate = today
		state.LastResult = "success"
		state.SignHistory = scheduler.UpdateSignHistory(state.SignHistory, today, signTime)
		as.logNextSignInfo()
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
	if resp == nil {
		return
	}
	log.Printf("✓ %s", resp.Msg)
	if resp.Code == 0 {
		log.Printf("  连续签到: %d 次", resp.Data.SignCount)
		log.Printf("  本次获得: %d 积分", resp.Data.SignPoint)
	}
}

// logNextSignInfo logs next sign-in information.
func (as *AccountSigner) logNextSignInfo() {
	now := time.Now().In(as.cfg.Location)
	tomorrow := now.AddDate(0, 0, 1)
	tomorrowDate := tomorrow.Format(scheduler.DateLayout)

	log.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Printf("下一次签到信息：")
	log.Printf("  签到日期: %s", tomorrowDate)
	log.Printf("  签到时间: 每日 %02d:00-%02d:00 之间随机时间（避免规律检测）", as.cfg.EarlyHourThreshold, as.cfg.LateHourThreshold)
	log.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}
