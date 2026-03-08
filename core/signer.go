package core

import (
	"errors"
	"fmt"
	"keyantong/config"
	"log"
	"time"
)

// Signer defines sign operations for an account.
type Signer interface {
	AttemptSign(now time.Time) error
	ForceSign(now time.Time) error
}

// AccountSigner implements Signer using Service.
type AccountSigner struct {
	service    *Service
	store      StateStore
	cfg        *config.AppConfig
	fileLogger *log.Logger
}

// NewAccountSigner creates a new account signer.
func NewAccountSigner(
	svc *Service,
	store StateStore,
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
	nowLocal := now.In(as.cfg.Location)
	currentHour := nowLocal.Hour()
	nowTime := nowLocal.Format(TimeLayout)

	if currentHour < as.cfg.EarlyHourThreshold || currentHour >= as.cfg.LateHourThreshold {
		log.Printf("当前时间 %s 不在工作时间范围 (%02d:00-%02d:00)，跳过启动签到",
			nowTime, as.cfg.EarlyHourThreshold, as.cfg.LateHourThreshold)
		log.Printf("将在工作时间内根据随机窗口自动签到")
		return nil
	}

	log.Printf("强制执行登录与签到")

	state, err := as.store.Load()
	if err != nil {
		log.Printf("无法加载状态，使用空状态: %v", err)
		state = &SignState{}
	}

	jitter := SleepWithJitter(300)
	log.Printf("执行前随机抖动: %v", jitter)

	log.Printf("正在登录...")
	if err := as.service.Login(); err != nil {
		return fmt.Errorf("登录失败: %w", err)
	}
	log.Printf("✓ 登录成功")

	log.Printf("正在签到...")
	resp, err := as.service.Sign()
	if err != nil {
		return fmt.Errorf("强制签到失败: %w", err)
	}

	now = time.Now().In(as.cfg.Location)
	today := now.Format(DateLayout)
	nowTime = now.Format(TimeLayout)

	logSignSuccess(resp)

	state.LastSignDate = today
	state.LastAttemptDate = today
	state.LastAttemptTime = nowTime
	state.LastResult = "success"
	state.SignHistory = UpdateSignHistory(state.SignHistory, today, nowTime)

	as.logNextSignInfo()

	if err := as.store.Save(state); err != nil {
		log.Printf("保存状态失败: %v", err)
	}
	return nil
}

// AttemptSign attempts to sign in if not already signed today.
func (as *AccountSigner) AttemptSign(now time.Time) error {
	nowLocal := now.In(as.cfg.Location)
	today := nowLocal.Format(DateLayout)
	nowTime := nowLocal.Format(TimeLayout)

	state, err := as.store.Load()
	if err != nil {
		log.Printf("无法加载状态: %v", err)
		state = &SignState{}
	}

	if state.LastSignDate == today {
		as.fileLogger.Printf("今天 (%s) 已完成签到，跳过", today)
		return nil
	}

	currentHour := nowLocal.Hour()

	if currentHour < as.cfg.EarlyHourThreshold {
		as.fileLogger.Printf("当前时间 %s 不在工作时间 (< %02d:00)，跳过签到",
			nowTime, as.cfg.EarlyHourThreshold)
		return nil
	}

	if currentHour >= as.cfg.LateHourThreshold {
		as.fileLogger.Printf("当前时间 %s 已超过工作时间 (>= %02d:00)，今日不再尝试签到",
			nowTime, as.cfg.LateHourThreshold)
		return nil
	}

	as.ensureWindowForToday(state, today)

	isLastChance := false
	nextCheck := nowLocal.Add(as.cfg.CheckInterval)
	if nextCheck.Hour() >= as.cfg.LateHourThreshold || nextCheck.Day() != nowLocal.Day() {
		isLastChance = true
	}

	if !isLastChance && nowTime < state.WindowSignTime {
		as.fileLogger.Printf("未到今日随机签到时间 (%s)，当前 %s，等待中",
			state.WindowSignTime, nowTime)
		return nil
	}

	if isLastChance && nowTime < state.WindowSignTime {
		log.Printf("⚠️  接近工作时间结束，提前执行签到（原计划: %s，当前: %s）",
			state.WindowSignTime, nowTime)
	}

	if state.LastAttemptDate == today && state.LastAttemptTime != "" {
		if as.shouldThrottle(nowLocal, state.LastAttemptTime) {
			as.fileLogger.Printf("距离上次尝试 (%s) 不足 %v，节流跳过",
				state.LastAttemptTime, as.cfg.RetryInterval)
			return nil
		}
	}

	state.LastAttemptDate = today
	state.LastAttemptTime = nowTime

	log.Printf("执行签到... (当前: %s, 计划时间: %s)", nowTime, state.WindowSignTime)

	jitter := SleepWithJitter(300)
	log.Printf("执行前随机抖动: %v", jitter)

	resp, err := as.performSignWithRetry()
	if err != nil {
		state.LastResult = "failed"
		if saveErr := as.store.Save(state); saveErr != nil {
			log.Printf("保存失败状态出错: %v", saveErr)
		}
		return fmt.Errorf("签到失败: %w", err)
	}

	nowLocal = time.Now().In(as.cfg.Location)
	today = nowLocal.Format(DateLayout)
	nowTime = nowLocal.Format(TimeLayout)

	as.recordSignState(resp, state, today, nowTime)
	return nil
}

func (as *AccountSigner) ensureWindowForToday(state *SignState, today string) {
	if state.WindowDate == today && state.WindowSignTime != "" {
		as.fileLogger.Printf("今日签到窗口已存在: %s %s", state.WindowDate, state.WindowSignTime)
		return
	}

	signTime := GenerateRandomSignTime(as.cfg.EarlyHourThreshold, as.cfg.LateHourThreshold)
	state.WindowDate = today
	state.WindowSignTime = signTime

	log.Printf("生成今日随机签到时间: %s %s", today, signTime)

	if err := as.store.Save(state); err != nil {
		log.Printf("保存签到窗口失败: %v", err)
	}
}

func (as *AccountSigner) shouldThrottle(now time.Time, lastAttemptTime string) bool {
	today := now.Format(DateLayout)
	lastAttempt, err := ParseDateTime(today, lastAttemptTime, as.cfg.Location)
	if err != nil {
		return false
	}
	return now.Sub(lastAttempt) < as.cfg.RetryInterval
}

func (as *AccountSigner) performSignWithRetry() (*SignResponse, error) {
	resp, err := as.service.Sign()
	if err != nil {
		if errors.Is(err, ErrLoginRequired) {
			log.Printf("会话未登录或已过期，重新登录")
			if loginErr := as.service.Login(); loginErr != nil {
				return nil, fmt.Errorf("登录失败: %w", loginErr)
			}
			resp, err = as.service.Sign()
			if err != nil {
				return nil, fmt.Errorf("登录后签到请求失败: %w", err)
			}
		} else {
			return nil, fmt.Errorf("签到请求失败: %w", err)
		}
	}

	if isLoginRequiredByCode(resp) {
		log.Printf("响应码表明需要重新登录，重新登录")
		if loginErr := as.service.Login(); loginErr != nil {
			return nil, fmt.Errorf("登录失败: %w", loginErr)
		}
		resp, err = as.service.Sign()
		if err != nil {
			return nil, fmt.Errorf("登录后签到请求失败: %w", err)
		}
		if isLoginRequiredByCode(resp) {
			return nil, fmt.Errorf("重新登录后仍无法签到，请检查账号凭证")
		}
	}

	return resp, nil
}

func isLoginRequiredByCode(resp *SignResponse) bool {
	if resp == nil {
		return true
	}
	return resp.Code != 0 && resp.Code != 1
}

func (as *AccountSigner) recordSignState(resp *SignResponse, state *SignState, today, signTime string) {
	if resp == nil {
		log.Printf("签到响应为空")
		return
	}

	switch resp.Code {
	case 0:
		logSignSuccess(resp)
		state.LastSignDate = today
		state.LastResult = "success"
		state.SignHistory = UpdateSignHistory(state.SignHistory, today, signTime)
		as.logNextSignInfo()
	case 1:
		log.Printf("%s", resp.Msg)
		state.LastSignDate = today
		state.LastResult = "success"
		state.SignHistory = UpdateSignHistory(state.SignHistory, today, signTime)
		as.logNextSignInfo()
	default:
		log.Printf("签到未成功: %s", resp.Msg)
		state.LastResult = "failed"
	}

	if err := as.store.Save(state); err != nil {
		log.Printf("保存签到状态失败: %v", err)
	}
}

func logSignSuccess(resp *SignResponse) {
	if resp == nil {
		return
	}
	log.Printf("✓ %s", resp.Msg)
	if resp.Code == 0 {
		log.Printf("  连续签到: %d 次", resp.Data.SignCount)
		log.Printf("  本次获得: %d 积分", resp.Data.SignPoint)
		log.Printf("  签到结果详情: %+v", resp)
	}
}

func (as *AccountSigner) logNextSignInfo() {
	now := time.Now().In(as.cfg.Location)
	tomorrow := now.AddDate(0, 0, 1)
	tomorrowDate := tomorrow.Format(DateLayout)

	log.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Printf("下一次签到信息：")
	log.Printf("  签到日期: %s", tomorrowDate)
	log.Printf("  工作时间窗口: %02d:00 - %02d:00", as.cfg.EarlyHourThreshold, as.cfg.LateHourThreshold)
	log.Printf("  签到时间: 将在工作时间窗口内随机生成（避免规律检测）")
	log.Printf("  非工作时间: 自动跳过，不执行签到")
	log.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}
