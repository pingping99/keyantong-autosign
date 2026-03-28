package core

import (
	"context"
	"errors"
	"keyantong/config"
	"log"
	"sync"
	"time"
)

// =============================================================================
// 接口定义
// =============================================================================

// Signer 签到器接口（支持 Context）
type Signer interface {
	// SignOnStartup 启动时签到（原 ForceSign）
	// 在工作时间内执行签到，非工作时间跳过
	SignOnStartup(ctx context.Context, now time.Time) error

	// AttemptSign 尝试签到
	// 检查时间窗口和随机签到时间
	AttemptSign(ctx context.Context, now time.Time) error
}

// LegacySigner 旧版接口（向后兼容）
// Deprecated: 建议使用带 Context 的 Signer 接口
type LegacySigner interface {
	AttemptSign(now time.Time) error
	ForceSign(now time.Time) error
}

// =============================================================================
// AccountSigner 实现
// =============================================================================

// AccountSigner 账户签到器实现
type AccountSigner struct {
	service    *Service
	store      StateStore
	cfg        *config.AppConfig
	fileLogger *log.Logger
	mu         sync.Mutex
}

// NewAccountSigner 创建新的账户签到器
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

// SignOnStartup 启动时签到（带 Context 支持）
// 在工作时间内执行签到，非工作时间跳过
func (as *AccountSigner) SignOnStartup(ctx context.Context, now time.Time) error {
	nowLocal := now.In(as.cfg.Location)
	currentHour := nowLocal.Hour()
	nowTime := nowLocal.Format(TimeLayout)

	// 检查工作时间
	if currentHour < as.cfg.EarlyHourThreshold || currentHour >= as.cfg.LateHourThreshold {
		log.Printf("当前时间 %s 不在工作时间范围 (%02d:00-%02d:00)，跳过启动签到",
			nowTime, as.cfg.EarlyHourThreshold, as.cfg.LateHourThreshold)
		log.Printf("将在工作时间内根据随机窗口自动签到")
		return nil
	}

	log.Printf("执行启动签到")

	// 加载状态
	state, err := as.store.Load()
	if err != nil {
		log.Printf("无法加载状态，使用空状态: %v", err)
		state = &SignState{}
	}

	// 非阻塞抖动等待（支持取消）
	jitter, ok := WaitWithJitter(ctx, 300)
	if !ok {
		return NewSignError(ErrTypeTimeout, "签到被取消（抖动等待期间）", ctx.Err())
	}
	log.Printf("执行前随机抖动: %v", jitter)

	// 执行签到
	return as.doSignWithContext(ctx, state, nowLocal)
}

// AttemptSign 尝试签到（带 Context 支持）
func (as *AccountSigner) AttemptSign(ctx context.Context, now time.Time) error {
	nowLocal := now.In(as.cfg.Location)
	today := nowLocal.Format(DateLayout)
	nowTime := nowLocal.Format(TimeLayout)

	// 加载状态
	state, err := as.store.Load()
	if err != nil {
		log.Printf("无法加载状态: %v", err)
		state = &SignState{}
	}

	// 检查今日是否已签到
	if state.LastSignDate == today {
		as.fileLogger.Printf("今天 (%s) 已完成签到，跳过", today)
		return NewSignError(ErrTypeAlreadySigned, "今日已签到", nil)
	}

	currentHour := nowLocal.Hour()

	// 检查工作时间
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

	// 确保今日有签到窗口
	as.ensureWindowForToday(state, today)

	// 检查是否是最后机会
	isLastChance := false
	nextCheck := nowLocal.Add(as.cfg.CheckInterval)
	if nextCheck.Hour() >= as.cfg.LateHourThreshold || nextCheck.Day() != nowLocal.Day() {
		isLastChance = true
	}

	// 检查是否到达签到时间
	if !isLastChance && nowTime < state.WindowSignTime {
		as.fileLogger.Printf("未到今日随机签到时间 (%s)，当前 %s，等待中",
			state.WindowSignTime, nowTime)
		return nil
	}

	if isLastChance && nowTime < state.WindowSignTime {
		log.Printf("⚠️  接近工作时间结束，提前执行签到（原计划: %s，当前: %s）",
			state.WindowSignTime, nowTime)
	}

	// 节流检查
	if state.LastAttemptDate == today && state.LastAttemptTime != "" {
		if as.shouldThrottle(nowLocal, state.LastAttemptTime) {
			as.fileLogger.Printf("距离上次尝试 (%s) 不足 %v，节流跳过",
				state.LastAttemptTime, as.cfg.RetryInterval)
			return nil
		}
	}

	// 更新尝试时间
	state.LastAttemptDate = today
	state.LastAttemptTime = nowTime

	log.Printf("执行签到... (当前: %s, 计划时间: %s)", nowTime, state.WindowSignTime)

	// 非阻塞抖动等待
	jitter, ok := WaitWithJitter(ctx, 300)
	if !ok {
		return NewSignError(ErrTypeTimeout, "签到被取消（抖动等待期间）", ctx.Err())
	}
	log.Printf("执行前随机抖动: %v", jitter)

	// 执行签到
	resp, err := as.performSignWithRetryContext(ctx)
	if err != nil {
		state.LastResult = "failed"
		UpdateHealth("failed")
		if saveErr := as.store.Save(state); saveErr != nil {
			log.Printf("保存失败状态出错: %v", saveErr)
		}
		return WrapError(err, ErrTypeNetwork, "签到失败")
	}

	// 更新时间（签到后）
	nowLocal = time.Now().In(as.cfg.Location)
	today = nowLocal.Format(DateLayout)
	nowTime = nowLocal.Format(TimeLayout)

	as.recordSignState(resp, state, today, nowTime)
	return nil
}

// doSignWithContext 执行签到的公共逻辑
func (as *AccountSigner) doSignWithContext(ctx context.Context, state *SignState, nowLocal time.Time) error {
	log.Printf("正在登录...")
	if err := as.service.LoginWithContext(ctx); err != nil {
		return NewSignError(ErrTypeAuth, "登录失败", err)
	}
	log.Printf("✓ 登录成功")

	log.Printf("正在签到...")
	resp, err := as.service.SignWithContext(ctx)
	if err != nil {
		return NewSignError(ErrTypeNetwork, "签到失败", err)
	}

	// 更新状态
	today := nowLocal.Format(DateLayout)
	nowTime := nowLocal.Format(TimeLayout)

	logSignSuccess(resp)

	state.LastSignDate = today
	state.LastAttemptDate = today
	state.LastAttemptTime = nowTime
	state.LastResult = "success"
		UpdateHealth("success")
	state.SignHistory = UpdateSignHistory(state.SignHistory, today, nowTime)

	as.logNextSignInfo()

	if err := as.store.Save(state); err != nil {
		log.Printf("保存状态失败: %v", err)
	}
	return nil
}

// performSignWithRetryContext 带重试的签到（支持 Context）
func (as *AccountSigner) performSignWithRetryContext(ctx context.Context) (*SignResponse, error) {
	resp, err := as.service.SignWithContext(ctx)
	if err != nil {
		if errors.Is(err, ErrLoginRequired) {
			log.Printf("会话未登录或已过期，重新登录")
			if loginErr := as.service.LoginWithContext(ctx); loginErr != nil {
				return nil, NewSignError(ErrTypeAuth, "登录失败", loginErr)
			}
			resp, err = as.service.SignWithContext(ctx)
			if err != nil {
				return nil, NewSignError(ErrTypeNetwork, "登录后签到请求失败", err)
			}
		} else {
			return nil, NewSignError(ErrTypeNetwork, "签到请求失败", err)
		}
	}

	if isLoginRequiredByCode(resp) {
		log.Printf("响应码表明需要重新登录，重新登录")
		if loginErr := as.service.LoginWithContext(ctx); loginErr != nil {
			return nil, NewSignError(ErrTypeAuth, "登录失败", loginErr)
		}
		resp, err = as.service.SignWithContext(ctx)
		if err != nil {
			return nil, NewSignError(ErrTypeNetwork, "登录后签到请求失败", err)
		}
		if isLoginRequiredByCode(resp) {
			return nil, NewSignError(ErrTypeAuth, "重新登录后仍无法签到，请检查账号凭证", nil)
		}
	}

	return resp, nil
}

// =============================================================================
// 辅助方法
// =============================================================================

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
		UpdateHealth("success")
		state.SignHistory = UpdateSignHistory(state.SignHistory, today, signTime)
		as.logNextSignInfo()
	case 1:
		log.Printf("%s", resp.Msg)
		state.LastSignDate = today
		state.LastResult = "success"
		UpdateHealth("success")
		state.SignHistory = UpdateSignHistory(state.SignHistory, today, signTime)
		as.logNextSignInfo()
	default:
		log.Printf("签到未成功: %s", resp.Msg)
		state.LastResult = "failed"
		UpdateHealth("failed")
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

// =============================================================================
// 向后兼容：旧版方法（无 Context）
// =============================================================================

// ForceSign 启动时签到（旧版接口，向后兼容）
// Deprecated: 建议使用 SignOnStartup(ctx, now)
func (as *AccountSigner) ForceSign(now time.Time) error {
	return as.SignOnStartup(context.Background(), now)
}

// AttemptSignLegacy 尝试签到（旧版接口，向后兼容）
// 注意：原 AttemptSign(time.Time) 已被新版覆盖
func (as *AccountSigner) AttemptSignLegacy(now time.Time) error {
	return as.AttemptSign(context.Background(), now)
}

// =============================================================================
// 多账户并发签到
// =============================================================================

// MultiAccountSigner 多账户签到管理器
type MultiAccountSigner struct {
	signers []Signer
}

// NewMultiAccountSigner 创建多账户签到管理器
func NewMultiAccountSigner(signers []Signer) *MultiAccountSigner {
	return &MultiAccountSigner{signers: signers}
}

// SignAllOnStartup 启动时所有账户并发签到
func (m *MultiAccountSigner) SignAllOnStartup(ctx context.Context, now time.Time) []error {
	return m.signAllConcurrent(ctx, now, func(s Signer, ctx context.Context, t time.Time) error {
		return s.SignOnStartup(ctx, t)
	})
}

// AttemptSignAll 尝试所有账户签到
func (m *MultiAccountSigner) AttemptSignAll(ctx context.Context, now time.Time) []error {
	return m.signAllConcurrent(ctx, now, func(s Signer, ctx context.Context, t time.Time) error {
		return s.AttemptSign(ctx, t)
	})
}

func (m *MultiAccountSigner) signAllConcurrent(
	ctx context.Context,
	now time.Time,
	signFunc func(Signer, context.Context, time.Time) error,
) []error {
	if len(m.signers) == 0 {
		return nil
	}

	var wg sync.WaitGroup
	errs := make([]error, len(m.signers))

	for i, signer := range m.signers {
		wg.Add(1)
		go func(idx int, s Signer) {
			defer wg.Done()
			errs[idx] = signFunc(s, ctx, now)
		}(i, signer)
	}

	wg.Wait()
	return errs
}
