package signer

import (
	"keyantong/config"
	"keyantong/service"
	"testing"
	"time"
)

func TestShouldThrottle(t *testing.T) {
	location := time.UTC
	cfg := &config.AppConfig{
		Location:      location,
		RetryInterval: 10 * time.Minute,
	}
	s := &AccountSigner{cfg: cfg}

	now := time.Date(2024, 6, 1, 10, 20, 0, 0, location)

	if !s.shouldThrottle(now, "10:15") {
		t.Fatalf("expected throttle when last attempt is within retry interval")
	}

	if s.shouldThrottle(now, "10:00") {
		t.Fatalf("expected no throttle when last attempt is outside retry interval")
	}
}

func TestIsLoginRequired(t *testing.T) {
	if !isLoginRequired(nil) {
		t.Fatalf("expected login required for nil response")
	}

	okResp := &service.SignResponse{Code: 0}
	if isLoginRequired(okResp) {
		t.Fatalf("expected no login required when code is 0")
	}

	alreadyResp := &service.SignResponse{Code: 1}
	if isLoginRequired(alreadyResp) {
		t.Fatalf("expected no login required when code is 1")
	}

	failResp := &service.SignResponse{Code: 2}
	if !isLoginRequired(failResp) {
		t.Fatalf("expected login required when code is non-0/1")
	}
}
