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

// Signer defines sign operations for an account.
type Signer interface {
	AttemptSign(now time.Time) error
	ForceSign(now time.Time) error
}

// AccountSigner implements Signer using service.Service.
type AccountSigner struct {
	account    domain.Account
	service    *service.Service
	store      store.StateStore
	cfg        *config.AppConfig
	logPrefix  string
	fileLogger *log.Logger
}

// NewAccountSigner creates a new account signer.
func NewAccountSigner(
	account domain.Account,
	svc *service.Service,
	store store.StateStore,
	cfg *config.AppConfig,
	fileLogger *log.Logger,
) *AccountSigner {
	return &AccountSigner{
		account:    account,
		service:    svc,
		store:      store,
		cfg:        cfg,
		logPrefix:  fmt.Sprintf("[%s]", account.Email),
		fileLogger: fileLogger,
	}
}

// ForceSign performs forced sign-in (used on startup).
func (as *AccountSigner) ForceSign(now time.Time) error {
	log.Printf("%s 强制执行登录与签到", as.logPrefix)
	if err := as.service.AutoSign(); err != nil {
		return fmt.Errorf("强制签到失败: %w", err)
	}

	today := now.In(as.cfg.Location).Format(config.DateLayout)
	nowTime := now.In(as.cfg.Location).Format(config.TimeLayout)
	state := &domain.SignState{
		LastSignDate:    today,
		LastAttemptDate: today,
		LastAttemptTime: nowTime,
		LastResult:      "success",
	}
	if err := as.store.Save(as.account.ID, state); err != nil {
		log.Printf("%s 保存状态失败: %v", as.logPrefix, err)
	}
	return nil
}

// AttemptSign attempts to sign in during the configured window with throttling.
func (as *AccountSigner) AttemptSign(now time.Time) error {
	nowLocal := now.In(as.cfg.Location)
	today := nowLocal.Format(config.DateLayout)
	nowTime := nowLocal.Format(config.TimeLayout)

	// Load or generate dynamic window for today
	state, err := as.store.Load(as.account.ID)
	if err != nil {
		log.Printf("%s 无法加载状态: %v", as.logPrefix, err)
		state = &domain.SignState{}
	}

	// Generate dynamic window if needed
	windowStart, windowEnd := as.getOrGenerateDynamicWindow(state, today, nowLocal)
	windowRange := fmt.Sprintf("%s-%s", windowStart, windowEnd)

	// Parse window times to duration
	windowStartDur, err := scheduler.ParseTimeWindow(windowStart)
	if err != nil {
		log.Printf("%s 解析窗口起始时间失败: %v", as.logPrefix, err)
		return nil
	}
	windowEndDur, err := scheduler.ParseTimeWindow(windowEnd)
	if err != nil {
		log.Printf("%s 解析窗口结束时间失败: %v", as.logPrefix, err)
		return nil
	}

	// Check if within window
	inWindow := scheduler.IsWithinWindow(nowLocal, windowStartDur, windowEndDur)
	if !inWindow {
		// Log to file only (not to stdout) when outside sign window
		as.fileLogger.Printf("%s 当前时间 %s 不在签到窗口 %s",
			as.logPrefix, nowTime, windowRange)
		return nil
	}

	// Check if already signed today
	if state.LastSignDate == today {
		as.fileLogger.Printf("%s 今天 (%s) 已完成签到，跳过", as.logPrefix, today)
		return nil
	}

	// Throttle: check if last attempt was too recent
	if state.LastAttemptDate == today && state.LastAttemptTime != "" {
		if as.shouldThrottle(nowLocal, state.LastAttemptTime) {
			as.fileLogger.Printf("%s 距离上次尝试 (%s) 不足 %v，节流跳过",
				as.logPrefix, state.LastAttemptTime, as.cfg.RetryInterval)
			return nil
		}
	}

	// Update attempt time before trying
	state.LastAttemptDate = today
	state.LastAttemptTime = nowTime

	// Attempt sign
	log.Printf("%s 签到窗口开启，开始签到... (当前时间 %s)", as.logPrefix, nowTime)
	resp, err := as.performSignWithRetry()
	if err != nil {
		state.LastResult = "failed"
		if saveErr := as.store.Save(as.account.ID, state); saveErr != nil {
			log.Printf("%s 保存失败状态出错: %v", as.logPrefix, saveErr)
		}
		return fmt.Errorf("签到失败: %w", err)
	}

	// Record result
	as.recordSignState(resp, state, today)
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
		log.Printf("%s 会话未登录或已过期，重新登录", as.logPrefix)
		if err := as.service.Login(); err != nil {
			return nil, fmt.Errorf("登录失败: %w", err)
		}
		resp, err = as.service.Sign()
		if err != nil {
			return nil, fmt.Errorf("登录后签到请求失败: %w", err)
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
func (as *AccountSigner) recordSignState(resp *service.SignResponse, state *domain.SignState, today string) {
	if resp == nil {
		log.Printf("%s 签到响应为空", as.logPrefix)
		return
	}
	if resp.Code == 0 {
		logSignSuccess(as.logPrefix, resp)
		state.LastSignDate = today
		state.LastResult = "success"
		if err := as.store.Save(as.account.ID, state); err != nil {
			log.Printf("%s 保存签到状态失败: %v", as.logPrefix, err)
		}
		return
	}
	if resp.Code == 1 {
		log.Printf("%s %s", as.logPrefix, resp.Msg)
		state.LastSignDate = today
		state.LastResult = "success"
		if err := as.store.Save(as.account.ID, state); err != nil {
			log.Printf("%s 保存签到状态失败: %v", as.logPrefix, err)
		}
		return
	}
	log.Printf("%s 签到未成功: %s", as.logPrefix, resp.Msg)
	state.LastResult = "failed"
	if err := as.store.Save(as.account.ID, state); err != nil {
		log.Printf("%s 保存失败状态出错: %v", as.logPrefix, err)
	}
}

// getOrGenerateDynamicWindow gets or generates today's dynamic window.
func (as *AccountSigner) getOrGenerateDynamicWindow(state *domain.SignState, today string, nowLocal time.Time) (string, string) {
	// If window exists for today, use it
	if state.WindowDate == today && state.WindowStart != "" && state.WindowEnd != "" {
		return state.WindowStart, state.WindowEnd
	}

	// Generate new window for today
	seed := nowLocal.Unix() / 86400 // Use day-based seed for consistency
	windowStart, windowEnd := scheduler.GenerateDynamicWindow(
		as.cfg.DynamicWindowStart,
		as.cfg.DynamicWindowEnd,
		as.cfg.DynamicWindowSpan,
		seed,
	)

	// Save to state
	state.WindowDate = today
	state.WindowStart = windowStart
	state.WindowEnd = windowEnd
	if err := as.store.Save(as.account.ID, state); err != nil {
		log.Printf("%s 保存窗口状态失败: %v", as.logPrefix, err)
	} else {
		log.Printf("%s 今日动态签到窗口: %s - %s", as.logPrefix, windowStart, windowEnd)
	}

	return windowStart, windowEnd
}

// logSignSuccess logs successful sign-in details.
func logSignSuccess(prefix string, resp *service.SignResponse) {
	log.Printf("%s ✓ %s", prefix, resp.Msg)
	log.Printf("%s   连续签到: %d 次", prefix, resp.Data.SignCount)
	log.Printf("%s   本次获得: %d 积分", prefix, resp.Data.SignPoint)
}
