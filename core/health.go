package core

import (
	"sync"
	"time"
)

// HealthInfo represents the health check response.
type HealthInfo struct {
	LastSignAt time.Time `json:"last_sign_at"`
	Status     string    `json:"status"`
	Uptime     string    `json:"uptime"`
}

var (
	healthMu         sync.RWMutex
	globalLastSignAt time.Time
	globalStatus     string = "pending"
	StartTime               = time.Now()
)

// UpdateHealth updates the global health status.
func UpdateHealth(status string) {
	healthMu.Lock()
	defer healthMu.Unlock()
	globalStatus = status
	if status == "success" {
		globalLastSignAt = time.Now()
	}
}

// GetHealth retrieves the current health status.
func GetHealth() HealthInfo {
	healthMu.RLock()
	defer healthMu.RUnlock()
	return HealthInfo{
		LastSignAt: globalLastSignAt,
		Status:     globalStatus,
		Uptime:     time.Since(StartTime).String(),
	}
}
