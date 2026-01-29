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
	account   domain.Account
	service   *service.Service
	store     store.StateStore
	cfg       *config.AppConfig
	logPrefix string
}

// NewAccountSigner creates a new account signer.
func NewAccountSigner(
	account domain.Account,
	svc *service.Service,
	store store.StateStore,
	cfg *config.AppConfig,
) *AccountSigner {
	return &AccountSigner{
		account:   account,
		service:   svc,
		store:     store,
		cfg:       cfg,
		logPrefix: fmt.Sprintf("[%s]", account.Email),
	}
}

// ForceSign performs forced sign-in (used on first run).
func (as *AccountSigner) ForceSign(now time.Time) error {
	log.Printf("%s 首次运行强制执行登录与签到", as.logPrefix)
	if err := as.service.AutoSign(); err != nil {
		return fmt.Errorf("首次签到失败: %w", err)
	}

	today := now.In(as.cfg.Location).Format(config.DateLayout)
	state := &domain.SignState{LastSignDate: today}
	if err := as.store.Save(as.account.ID, state); err != nil {
		log.Printf("%s 保存状态失败: %v", as.logPrefix, err)
	}
	return nil
}

// AttemptSign attempts to sign in during the configured window.
func (as *AccountSigner) AttemptSign(now time.Time) error {
	nowLocal := now.In(as.cfg.Location)
	today := nowLocal.Format(config.DateLayout)
	windowRange := fmt.Sprintf("%s-%s",
		scheduler.FormatWindow(as.cfg.WindowStart),
		scheduler.FormatWindow(as.cfg.WindowEnd))

	// Check if within window
	if !scheduler.IsWithinWindow(nowLocal, as.cfg.WindowStart, as.cfg.WindowEnd) {
		log.Printf("%s 当前时间 %s 不在签到窗口 %s，等待下一次检查",
			as.logPrefix, nowLocal.Format("15:04"), windowRange)
		return nil
	}

	// Load state
	state, err := as.store.Load(as.account.ID)
	if err != nil {
		log.Printf("%s 无法加载状态: %v", as.logPrefix, err)
		state = &domain.SignState{}
	}

	// Check if already signed today
	if state.LastSignDate == today {
		log.Printf("%s 今天 (%s) 已完成签到，跳过", as.logPrefix, today)
		return nil
	}

	// Attempt sign
	log.Printf("%s 签到窗口开启，开始签到... (当前时间 %s)", as.logPrefix, nowLocal.Format("15:04"))
	resp, err := as.service.Sign()
	if err != nil {
		return fmt.Errorf("签到请求失败: %w", err)
	}

	// Handle login required
	if isLoginRequired(resp) {
		log.Printf("%s 会话未登录或已过期，重新登录", as.logPrefix)
		if err := as.service.Login(); err != nil {
			return fmt.Errorf("登录失败: %w", err)
		}
		resp, err = as.service.Sign()
		if err != nil {
			return fmt.Errorf("登录后签到请求失败: %w", err)
		}
	}

	// Record result
	as.recordSignState(resp, state, today)
	return nil
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
		if err := as.store.Save(as.account.ID, state); err != nil {
			log.Printf("%s 保存签到状态失败: %v", as.logPrefix, err)
		}
		return
	}
	if resp.Code == 1 {
		log.Printf("%s %s", as.logPrefix, resp.Msg)
		state.LastSignDate = today
		if err := as.store.Save(as.account.ID, state); err != nil {
			log.Printf("%s 保存签到状态失败: %v", as.logPrefix, err)
		}
		return
	}
	log.Printf("%s 签到未成功: %s", as.logPrefix, resp.Msg)
}

// logSignSuccess logs successful sign-in details.
func logSignSuccess(prefix string, resp *service.SignResponse) {
	log.Printf("%s ✓ %s", prefix, resp.Msg)
	log.Printf("%s   连续签到: %d 次", prefix, resp.Data.SignCount)
	log.Printf("%s   本次获得: %d 积分", prefix, resp.Data.SignPoint)
}
