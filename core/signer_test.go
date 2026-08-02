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

func (store *memoryStore) Load() (*SignState, error) {
	copy := *store.state
	copy.SignHistory = append([]SignRecord(nil), store.state.SignHistory...)
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
	if store.state.LastSignDate != "2026-08-02" || store.state.LastResult != "success" {
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
	store := &memoryStore{state: &SignState{Version: CurrentStateVersion, LastSignDate: "2026-08-02"}}
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
		t.Fatalf("legacy state was trusted without verification: calls=%d state=%#v", service.calls, store.state)
	}
}

func newTestSigner(service SignService, store StateStore) *AccountSigner {
	cfg := &config.AppConfig{
		CheckInterval:      30 * time.Minute,
		RetryInterval:      10 * time.Minute,
		SignJitterMax:      0,
		Location:           time.FixedZone("CST", 8*60*60),
		EarlyHourThreshold: 8,
		LateHourThreshold:  22,
	}
	return NewAccountSigner(service, store, cfg, log.New(io.Discard, "", 0))
}

func testTime() time.Time {
	return time.Date(2026, 8, 2, 10, 0, 0, 0, time.FixedZone("CST", 8*60*60))
}
