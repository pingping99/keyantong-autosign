package core

import (
	"sync"
	"time"
)

type HealthInfo struct {
	Status        string    `json:"status"`
	LastAttemptAt time.Time `json:"last_attempt_at,omitempty"`
	LastSuccessAt time.Time `json:"last_success_at,omitempty"`
	LastError     string    `json:"last_error,omitempty"`
	Uptime        string    `json:"uptime"`
}

var healthState = struct {
	sync.RWMutex
	status        string
	lastAttemptAt time.Time
	lastSuccessAt time.Time
	lastError     string
	startedAt     time.Time
}{
	status:    "pending",
	startedAt: time.Now(),
}

func MarkHealthAttempt(at time.Time) {
	healthState.Lock()
	defer healthState.Unlock()
	healthState.lastAttemptAt = at
}

func MarkHealthSuccess(at time.Time) {
	healthState.Lock()
	defer healthState.Unlock()
	healthState.status = "success"
	healthState.lastAttemptAt = at
	healthState.lastSuccessAt = at
	healthState.lastError = ""
}

func MarkHealthFailure(at time.Time, err error) {
	healthState.Lock()
	defer healthState.Unlock()
	healthState.status = "failed"
	healthState.lastAttemptAt = at
	healthState.lastError = PublicError(err)
}

func GetHealth() HealthInfo {
	healthState.RLock()
	defer healthState.RUnlock()
	return HealthInfo{
		Status:        healthState.status,
		LastAttemptAt: healthState.lastAttemptAt,
		LastSuccessAt: healthState.lastSuccessAt,
		LastError:     healthState.lastError,
		Uptime:        time.Since(healthState.startedAt).Round(time.Second).String(),
	}
}
