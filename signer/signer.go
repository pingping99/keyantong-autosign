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

// Signer defines sign operations.
type Signer interface {
AttemptSign(now time.Time) error
ForceSign(now time.Time) error
}

// SimpleSigner implements Signer using service.Service.
type SimpleSigner struct {
service    *service.Service
store      store.StateStore
cfg        *config.AppConfig
fileLogger *log.Logger
}

// NewSigner creates a new signer.
func NewSigner(
svc *service.Service,
store store.StateStore,
cfg *config.AppConfig,
fileLogger *log.Logger,
) *SimpleSigner {
return &SimpleSigner{
service:    svc,
store:      store,
cfg:        cfg,
fileLogger: fileLogger,
}
}

// ForceSign performs forced sign-in (used on startup).
func (s *SimpleSigner) ForceSign(now time.Time) error {
log.Printf("强制执行登录与签到")
if err := s.service.AutoSign(); err != nil {
return fmt.Errorf("强制签到失败: %w", err)
}

today := now.In(s.cfg.Location).Format(config.DateLayout)
nowTime := now.In(s.cfg.Location).Format(config.TimeLayout)
state := &domain.SignState{
LastSignDate:    today,
LastAttemptDate: today,
LastAttemptTime: nowTime,
LastResult:      "success",
}
if err := s.store.Save(state); err != nil {
log.Printf("保存状态失败: %v", err)
}
return nil
}

// AttemptSign attempts to sign in during the configured window with randomization.
func (s *SimpleSigner) AttemptSign(now time.Time) error {
nowLocal := now.In(s.cfg.Location)
today := nowLocal.Format(config.DateLayout)
nowTime := nowLocal.Format(config.TimeLayout)

// Load or generate dynamic window for today
state, err := s.store.Load()
if err != nil {
log.Printf("无法加载状态: %v", err)
state = &domain.SignState{}
}

// Generate dynamic window and scheduled time if needed
windowStart, windowEnd, scheduledTime := s.getOrGenerateDynamicWindow(state, today, nowLocal)
windowRange := fmt.Sprintf("%s-%s", windowStart, windowEnd)

// Parse window times to duration
windowStartDur, err := scheduler.ParseTimeWindow(windowStart)
if err != nil {
log.Printf("解析窗口起始时间失败: %v", err)
return nil
}
windowEndDur, err := scheduler.ParseTimeWindow(windowEnd)
if err != nil {
log.Printf("解析窗口结束时间失败: %v", err)
return nil
}

// Check if within window
inWindow := scheduler.IsWithinWindow(nowLocal, windowStartDur, windowEndDur)
if !inWindow {
// Log to file only (not to stdout) when outside sign window
s.fileLogger.Printf("当前时间 %s 不在签到窗口 %s", nowTime, windowRange)
return nil
}

// Check if already signed today
if state.LastSignDate == today {
s.fileLogger.Printf("今天 (%s) 已完成签到，跳过", today)
return nil
}

// Check if we've reached the scheduled time for signing
scheduledTimeDur, err := scheduler.ParseTimeWindow(scheduledTime)
if err != nil {
log.Printf("解析计划签到时间失败: %v", err)
return nil
}

currentDur := scheduler.GetCurrentDuration(nowLocal)
if currentDur < scheduledTimeDur {
// Haven't reached scheduled time yet
s.fileLogger.Printf("当前时间 %s，等待计划签到时间 %s", nowTime, scheduledTime)
return nil
}

// Throttle: check if last attempt was too recent
if state.LastAttemptDate == today && state.LastAttemptTime != "" {
if s.shouldThrottle(nowLocal, state.LastAttemptTime) {
s.fileLogger.Printf("距离上次尝试 (%s) 不足 %v，节流跳过",
state.LastAttemptTime, s.cfg.RetryInterval)
return nil
}
}

// Update attempt time before trying
state.LastAttemptDate = today
state.LastAttemptTime = nowTime

// Attempt sign
log.Printf("签到时间到达（计划时间 %s），开始签到...", scheduledTime)
resp, err := s.performSignWithRetry()
if err != nil {
state.LastResult = "failed"
if saveErr := s.store.Save(state); saveErr != nil {
log.Printf("保存失败状态出错: %v", saveErr)
}
return fmt.Errorf("签到失败: %w", err)
}

// Record result
s.recordSignState(resp, state, today)
return nil
}

// shouldThrottle checks if we should skip this attempt based on retry interval.
func (s *SimpleSigner) shouldThrottle(now time.Time, lastAttemptTime string) bool {
// Construct full datetime for comparison
today := now.Format(config.DateLayout)
lastAttempt, err := time.ParseInLocation(config.DateLayout+" "+config.TimeLayout,
today+" "+lastAttemptTime, s.cfg.Location)
if err != nil {
return false // If parse fails, don't throttle
}

elapsed := now.Sub(lastAttempt)
return elapsed < s.cfg.RetryInterval
}

// performSignWithRetry attempts sign-in with login retry logic.
func (s *SimpleSigner) performSignWithRetry() (*service.SignResponse, error) {
resp, err := s.service.Sign()
if err != nil {
return nil, fmt.Errorf("签到请求失败: %w", err)
}

// Handle login required
if isLoginRequired(resp) {
log.Printf("会话未登录或已过期，重新登录")
if err := s.service.Login(); err != nil {
return nil, fmt.Errorf("登录失败: %w", err)
}
resp, err = s.service.Sign()
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
func (s *SimpleSigner) recordSignState(resp *service.SignResponse, state *domain.SignState, today string) {
if resp == nil {
log.Printf("签到响应为空")
return
}
if resp.Code == 0 {
logSignSuccess(resp)
state.LastSignDate = today
state.LastResult = "success"
if err := s.store.Save(state); err != nil {
log.Printf("保存签到状态失败: %v", err)
}
return
}
if resp.Code == 1 {
log.Printf("%s", resp.Msg)
state.LastSignDate = today
state.LastResult = "success"
if err := s.store.Save(state); err != nil {
log.Printf("保存签到状态失败: %v", err)
}
return
}
log.Printf("签到未成功: %s", resp.Msg)
state.LastResult = "failed"
if err := s.store.Save(state); err != nil {
log.Printf("保存失败状态出错: %v", err)
}
}

// getOrGenerateDynamicWindow gets or generates today's dynamic window and scheduled time.
func (s *SimpleSigner) getOrGenerateDynamicWindow(state *domain.SignState, today string, nowLocal time.Time) (string, string, string) {
// If window exists for today, use it
if state.WindowDate == today && state.WindowStart != "" && state.WindowEnd != "" && state.ScheduledTime != "" {
return state.WindowStart, state.WindowEnd, state.ScheduledTime
}

// Generate new window for today
seed := nowLocal.Unix() / 86400 // Use day-based seed for consistency
windowStart, windowEnd := scheduler.GenerateDynamicWindow(
s.cfg.DynamicWindowStart,
s.cfg.DynamicWindowEnd,
s.cfg.DynamicWindowSpan,
seed,
)

// Generate random scheduled time within the window
windowStartDur, err := scheduler.ParseTimeWindow(windowStart)
if err != nil {
log.Printf("解析窗口起始时间失败: %v", err)
return windowStart, windowEnd, windowStart // Fallback to window start
}
windowEndDur, err := scheduler.ParseTimeWindow(windowEnd)
if err != nil {
log.Printf("解析窗口结束时间失败: %v", err)
return windowStart, windowEnd, windowStart // Fallback to window start
}
windowSpan := windowEndDur - windowStartDur
randomDelay := scheduler.GenerateRandomDelay(windowSpan)
scheduledTimeDur := windowStartDur + randomDelay
scheduledTime := scheduler.FormatWindow(scheduledTimeDur)

// Save to state
state.WindowDate = today
state.WindowStart = windowStart
state.WindowEnd = windowEnd
state.ScheduledTime = scheduledTime
if err := s.store.Save(state); err != nil {
log.Printf("保存窗口状态失败: %v", err)
} else {
log.Printf("今日动态签到窗口: %s - %s，计划签到时间: %s", windowStart, windowEnd, scheduledTime)
}

return windowStart, windowEnd, scheduledTime
}

// logSignSuccess logs successful sign-in details.
func logSignSuccess(resp *service.SignResponse) {
log.Printf("✓ %s", resp.Msg)
log.Printf("  连续签到: %d 次", resp.Data.SignCount)
log.Printf("  本次获得: %d 积分", resp.Data.SignPoint)
}
