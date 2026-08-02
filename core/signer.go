package core

import (
	"context"
	"errors"
	"fmt"
	"keyantong/config"
	"log"
	"strings"
	"sync"
	"time"
)

type Signer interface {
	SignOnStartup(ctx context.Context, now time.Time) error
	AttemptSign(ctx context.Context, now time.Time) error
}

type AccountSigner struct {
	service        SignService
	store          StateStore
	cfg            *config.AppConfig
	fileLogger     *log.Logger
	accountID      string
	now            func() time.Time
	waitWithJitter func(context.Context, time.Duration) (time.Duration, bool)
	mu             sync.Mutex
}

func NewAccountSigner(service SignService, store StateStore, cfg *config.AppConfig, fileLogger *log.Logger) *AccountSigner {
	return &AccountSigner{
		service:        service,
		store:          store,
		cfg:            cfg,
		fileLogger:     fileLogger,
		accountID:      GenerateAccountID(cfg.Email),
		now:            time.Now,
		waitWithJitter: WaitWithJitter,
	}
}

func (signer *AccountSigner) SignOnStartup(ctx context.Context, now time.Time) error {
	signer.mu.Lock()
	defer signer.mu.Unlock()

	localNow := now.In(signer.cfg.Location)
	if !IsWithinHourRange(localNow, signer.cfg.Location, signer.cfg.EarlyHourThreshold, signer.cfg.LateHourThreshold) {
		log.Printf("当前时间 %s 不在工作时间范围，跳过启动签到", localNow.Format(TimeLayout))
		return nil
	}

	state, err := signer.store.Load(signer.accountID)
	if err != nil {
		return NewSignError(ErrTypeStore, "加载签到状态失败", err)
	}
	if state.Version >= CurrentStateVersion && state.LastSignDate == localNow.Format(DateLayout) {
		return NewSignError(ErrTypeAlreadySigned, "今日已签到", nil)
	}
	if signer.shouldThrottle(localNow, state) {
		return nil
	}
	return signer.executeSign(ctx, state, localNow, "启动签到")
}

func (signer *AccountSigner) AttemptSign(ctx context.Context, now time.Time) error {
	signer.mu.Lock()
	defer signer.mu.Unlock()

	localNow := now.In(signer.cfg.Location)
	today := localNow.Format(DateLayout)
	nowTime := localNow.Format(TimeLayout)
	state, err := signer.store.Load(signer.accountID)
	if err != nil {
		return NewSignError(ErrTypeStore, "加载签到状态失败", err)
	}
	if state.Version >= CurrentStateVersion && state.LastSignDate == today {
		signer.fileLogger.Printf("今天 (%s) 已完成签到，跳过", today)
		return NewSignError(ErrTypeAlreadySigned, "今日已签到", nil)
	}
	if !IsWithinHourRange(localNow, signer.cfg.Location, signer.cfg.EarlyHourThreshold, signer.cfg.LateHourThreshold) {
		return nil
	}
	if err := signer.ensureWindowForToday(state, today); err != nil {
		return err
	}

	nextCheck := localNow.Add(signer.cfg.CheckInterval)
	isLastChance := nextCheck.Hour() >= signer.cfg.LateHourThreshold || nextCheck.Day() != localNow.Day()
	if !isLastChance && nowTime < state.WindowSignTime {
		signer.fileLogger.Printf("未到今日随机签到时间 (%s)，当前 %s", state.WindowSignTime, nowTime)
		return nil
	}
	if signer.shouldThrottle(localNow, state) {
		return nil
	}
	return signer.executeSign(ctx, state, localNow, "计划签到")
}

func (signer *AccountSigner) executeSign(ctx context.Context, state *SignState, scheduledAt time.Time, reason string) error {
	state.AccountID = signer.accountID
	state.LastScheduledAt = scheduledAt.Format(time.RFC3339Nano)
	state.LastResult = "pending"
	if err := signer.store.Save(state); err != nil {
		storeErr := NewSignError(ErrTypeStore, "保存签到计划状态失败", err)
		MarkHealthFailure(scheduledAt, storeErr)
		return storeErr
	}

	MarkHealthAttempt(scheduledAt)
	log.Printf("执行%s，当前时间 %s", reason, scheduledAt.Format(TimeLayout))

	maxJitter := signer.maxJitterBeforeWorkEnd(scheduledAt)
	jitter, completed := signer.waitWithJitter(ctx, maxJitter)
	if !completed {
		err := NewSignError(ErrTypeTimeout, "签到在随机等待期间被取消", ctx.Err())
		signer.markFailure(state, signer.localNow(), err)
		return err
	}
	if jitter > 0 {
		log.Printf("执行前随机等待: %v", jitter.Round(time.Second))
	}

	requestAt := signer.localNow()
	if !IsWithinHourRange(requestAt, signer.cfg.Location, signer.cfg.EarlyHourThreshold, signer.cfg.LateHourThreshold) {
		return signer.markSkipped(state, requestAt, "随机等待结束时已超出工作时间")
	}
	state.LastRequestAt = requestAt.Format(time.RFC3339Nano)
	if err := signer.store.Save(state); err != nil {
		storeErr := NewSignError(ErrTypeStore, "保存请求开始状态失败", err)
		MarkHealthFailure(requestAt, storeErr)
		return storeErr
	}

	response, err := signer.performSignWithRetry(ctx)
	completedAt := signer.localNow()
	if err != nil {
		signer.markFailure(state, completedAt, err)
		return err
	}
	return signer.recordSignResult(response, state, completedAt)
}

func (signer *AccountSigner) localNow() time.Time {
	return signer.now().In(signer.cfg.Location)
}

func (signer *AccountSigner) maxJitterBeforeWorkEnd(at time.Time) time.Duration {
	local := at.In(signer.cfg.Location)
	workEnd := time.Date(
		local.Year(), local.Month(), local.Day(),
		signer.cfg.LateHourThreshold, 0, 0, 0,
		signer.cfg.Location,
	)
	remaining := workEnd.Sub(local) - time.Second
	if remaining <= 0 {
		return 0
	}
	if signer.cfg.SignJitterMax < remaining {
		return signer.cfg.SignJitterMax
	}
	return remaining
}

func (signer *AccountSigner) performSignWithRetry(ctx context.Context) (*SignResponse, error) {
	response, err := signer.service.SignWithContext(ctx)
	if err == nil {
		return response, nil
	}
	if !errors.Is(err, ErrLoginRequired) {
		return nil, WrapError(err, ErrTypeNetwork, "签到请求失败")
	}

	log.Printf("会话未登录或已过期，重新登录")
	if err := signer.service.LoginWithContext(ctx); err != nil {
		return nil, NewSignError(ErrTypeAuth, "登录失败", err)
	}
	response, err = signer.service.SignWithContext(ctx)
	if err != nil {
		if errors.Is(err, ErrLoginRequired) {
			return nil, NewSignError(ErrTypeAuth, "重新登录后仍要求登录，请检查账号凭证", err)
		}
		return nil, WrapError(err, ErrTypeNetwork, "登录后签到请求失败")
	}
	return response, nil
}

func (signer *AccountSigner) recordSignResult(response *SignResponse, state *SignState, completedAt time.Time) error {
	if response == nil {
		err := NewSignError(ErrTypeServer, "签到响应为空", nil)
		signer.markFailure(state, completedAt, err)
		return err
	}

	today := completedAt.Format(DateLayout)
	timeString := completedAt.Format(TimeLayout)
	switch response.Code {
	case 0:
		log.Printf("✓ %s", response.Msg)
		log.Printf("连续签到: %d 次，本次获得: %d 积分", response.Data.SignCount, response.Data.SignPoint)
	case 1:
		log.Printf("✓ %s", response.Msg)
	default:
		message := strings.TrimSpace(response.Msg)
		if message == "" {
			message = "未知业务错误"
		}
		err := NewSignError(
			ErrTypeServer,
			fmt.Sprintf("签到未成功：code=%d, message=%s", response.Code, message),
			nil,
		)
		signer.markFailure(state, completedAt, err)
		return err
	}

	state.Version = CurrentStateVersion
	state.AccountID = signer.accountID
	state.LastSignDate = today
	signer.setCompletion(state, completedAt, "success")
	state.SignHistory = UpdateSignHistory(state.SignHistory, today, timeString)
	if err := signer.store.Save(state); err != nil {
		storeErr := NewSignError(ErrTypeStore, "保存签到成功状态失败", err)
		MarkHealthFailure(completedAt, storeErr)
		return storeErr
	}
	MarkHealthSuccess(completedAt)
	return nil
}

func (signer *AccountSigner) setCompletion(state *SignState, at time.Time, result string) {
	state.LastAttemptDate = at.Format(DateLayout)
	state.LastAttemptTime = at.Format(TimeLayout)
	state.LastCompletedAt = at.Format(time.RFC3339Nano)
	state.LastResult = result
}

func (signer *AccountSigner) markFailure(state *SignState, at time.Time, signErr error) {
	signer.setCompletion(state, at, "failed")
	if err := signer.store.Save(state); err != nil {
		log.Printf("保存签到失败状态出错: %v", err)
	}
	MarkHealthFailure(at, signErr)
}

func (signer *AccountSigner) markSkipped(state *SignState, at time.Time, reason string) error {
	signer.setCompletion(state, at, "skipped")
	if err := signer.store.Save(state); err != nil {
		return NewSignError(ErrTypeStore, "保存跳过状态失败", err)
	}
	signer.fileLogger.Printf("%s", reason)
	return nil
}

func (signer *AccountSigner) ensureWindowForToday(state *SignState, today string) error {
	if state.WindowDate == today && state.WindowSignTime != "" {
		return nil
	}
	state.AccountID = signer.accountID
	state.WindowDate = today
	state.WindowSignTime = GenerateRandomSignTime(
		signer.cfg.EarlyHourThreshold,
		signer.cfg.LateHourThreshold,
	)
	log.Printf("生成今日随机签到时间: %s %s", today, state.WindowSignTime)
	if err := signer.store.Save(state); err != nil {
		return NewSignError(ErrTypeStore, "保存签到窗口失败", err)
	}
	return nil
}

func (signer *AccountSigner) shouldThrottle(now time.Time, state *SignState) bool {
	var lastAttempt time.Time
	var err error
	if state.LastCompletedAt != "" {
		lastAttempt, err = time.Parse(time.RFC3339Nano, state.LastCompletedAt)
	} else if state.LastAttemptDate == now.Format(DateLayout) && state.LastAttemptTime != "" {
		lastAttempt, err = ParseDateTime(
			state.LastAttemptDate,
			state.LastAttemptTime,
			signer.cfg.Location,
		)
	} else {
		return false
	}
	if err != nil {
		return false
	}
	if now.Sub(lastAttempt) >= signer.cfg.RetryInterval {
		return false
	}
	signer.fileLogger.Printf("距离上次完成尝试不足 %v，跳过", signer.cfg.RetryInterval)
	return true
}
