package core

import (
	"context"
	"errors"
	"io"
	"keyantong/config"
	"log"
	"testing"
	"time"
)

type fakeService struct {
	responses []*SignResponse
	errors    []error
	loginErr  error
	logins    int
	calls     int
}

func (service *fakeService) LoginWithContext(context.Context) error {
	service.logins++
	return service.loginErr
}

func (service *fakeService) SignWithContext(context.Context) (*SignResponse, error) {
	index := service.calls
	service.calls++

	var response *SignResponse
	var err error
	if index < len(service.responses) {
		response = service.responses[index]
	}
	if index < len(service.errors) {
		err = service.errors[index]
	}
	return response, err
}

type memoryStore struct {
	state *SignState
}

func (store *memoryStore) Load(accountID string) (*SignState, error) {
	copy := *store.state
	copy.SignHistory = append([]SignRecord(nil), store.state.SignHistory...)
	if copy.AccountID == "" {
		copy.AccountID = accountID
	}
	return &copy, nil
}

func (store *memoryStore) Save(state *SignState) error {
	copy := *state
	copy.SignHistory = append([]SignRecord(nil), state.SignHistory...)
	store.state = &copy
	return nil
}

func TestUnknownBusinessCodeIsNotRecordedAsSuccess(t *testing.T) {
	service := &fakeService{responses: []*SignResponse{{Code: 9, Msg: "blocked"}}}
	store := &memoryStore{state: &SignState{}}
	signer := newTestSigner(service, store)

	err := signer.SignOnStartup(context.Background(), testTime())
	if err == nil || !errors.Is(err, ErrServer) {
		t.Fatalf("expected server error, got %v", err)
	}
	if store.state.LastSignDate != "" || store.state.LastResult != "failed" {
		t.Fatalf("business failure recorded as success: %#v", store.state)
	}
}

func TestAlreadySignedResponseUpdatesState(t *testing.T) {
	service := &fakeService{responses: []*SignResponse{{Code: 1, Msg: "already signed"}}}
	store := &memoryStore{state: &SignState{}}
	signer := newTestSigner(service, store)

	if err := signer.SignOnStartup(context.Background(), testTime()); err != nil {
		t.Fatal(err)
	}
	if store.state.LastSignDate != "2026-08-02" ||
		store.state.LastResult != "success" ||
		store.state.AccountID == "" {
		t.Fatalf("already-signed response not persisted: %#v", store.state)
	}
}

func TestLoginRequiredRetriesOnce(t *testing.T) {
	service := &fakeService{
		responses: []*SignResponse{nil, {Code: 0, Msg: "ok"}},
		errors:    []error{ErrLoginRequired, nil},
	}
	store := &memoryStore{state: &SignState{}}
	signer := newTestSigner(service, store)

	if err := signer.SignOnStartup(context.Background(), testTime()); err != nil {
		t.Fatal(err)
	}
	if service.logins != 1 || service.calls != 2 {
		t.Fatalf("unexpected retry counts: logins=%d calls=%d", service.logins, service.calls)
	}
}

func TestStartupSkipsKnownSignedDay(t *testing.T) {
	service := &fakeService{}
	store := &memoryStore{state: &SignState{
		Version:      CurrentStateVersion,
		AccountID:    GenerateAccountID("test@example.com"),
		LastSignDate: "2026-08-02",
	}}
	signer := newTestSigner(service, store)

	err := signer.SignOnStartup(context.Background(), testTime())
	if !IsAlreadySignedError(err) {
		t.Fatalf("expected already signed error, got %v", err)
	}
	if service.calls != 0 {
		t.Fatalf("sign service called %d times", service.calls)
	}
}

func TestLegacyStateIsRechecked(t *testing.T) {
	service := &fakeService{responses: []*SignResponse{{Code: 1, Msg: "already signed"}}}
	store := &memoryStore{state: &SignState{LastSignDate: "2026-08-02"}}
	signer := newTestSigner(service, store)

	if err := signer.SignOnStartup(context.Background(), testTime()); err != nil {
		t.Fatal(err)
	}
	if service.calls != 1 || store.state.Version != CurrentStateVersion {
		t.Fatalf("legacy state was trusted: calls=%d state=%#v", service.calls, store.state)
	}
}

func TestJitterIsCappedBeforeWorkEnd(t *testing.T) {
	signer := newTestSigner(&fakeService{}, &memoryStore{state: &SignState{}})
	signer.cfg.SignJitterMax = 5 * time.Minute
	at := time.Date(2026, 8, 2, 21, 59, 30, 0, signer.cfg.Location)

	maxJitter := signer.maxJitterBeforeWorkEnd(at)
	if maxJitter > 29*time.Second || maxJitter <= 0 {
		t.Fatalf("unexpected capped jitter: %v", maxJitter)
	}
}

func TestRequestSkippedWhenWaitEndsOutsideWorkHours(t *testing.T) {
	service := &fakeService{}
	store := &memoryStore{state: &SignState{}}
	signer := newTestSigner(service, store)
	signer.cfg.SignJitterMax = time.Minute
	signer.waitWithJitter = func(context.Context, time.Duration) (time.Duration, bool) {
		return 45 * time.Second, true
	}
	signer.now = func() time.Time {
		return time.Date(2026, 8, 2, 22, 0, 15, 0, signer.cfg.Location)
	}
	at := time.Date(2026, 8, 2, 21, 59, 30, 0, signer.cfg.Location)

	if err := signer.SignOnStartup(context.Background(), at); err != nil {
		t.Fatal(err)
	}
	if service.calls != 0 || store.state.LastResult != "skipped" {
		t.Fatalf("unexpected state calls=%d state=%#v", service.calls, store.state)
	}
}

func TestThrottleUsesCompletionTime(t *testing.T) {
	completed := testTime().Add(-5 * time.Minute)
	state := &SignState{LastCompletedAt: completed.Format(time.RFC3339Nano)}
	signer := newTestSigner(&fakeService{}, &memoryStore{state: state})

	if !signer.shouldThrottle(testTime(), state) {
		t.Fatal("expected throttling from completion time")
	}
}

func newTestSigner(service SignService, store StateStore) *AccountSigner {
	location := time.FixedZone("CST", 8*60*60)
	cfg := &config.AppConfig{
		Email:              "test@example.com",
		CheckInterval:      30 * time.Minute,
		RetryInterval:      10 * time.Minute,
		SignJitterMax:      0,
		Location:           location,
		EarlyHourThreshold: 8,
		LateHourThreshold:  22,
	}
	signer := NewAccountSigner(service, store, cfg, log.New(io.Discard, "", 0))
	signer.now = func() time.Time { return testTime() }
	return signer
}

func testTime() time.Time {
	return time.Date(2026, 8, 2, 10, 0, 0, 0, time.FixedZone("CST", 8*60*60))
}
