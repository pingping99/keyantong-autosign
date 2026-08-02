package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileStoreQuarantinesCorruptState(t *testing.T) {
	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.statePath(), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load("account-a"); err == nil {
		t.Fatal("expected decode error")
	}
	matches, err := filepath.Glob(store.statePath() + ".corrupt-*")
	if err != nil || len(matches) != 1 {
		t.Fatalf("expected one quarantined state file, matches=%v err=%v", matches, err)
	}
}

func TestFileStoreSupportsConsecutiveSaves(t *testing.T) {
	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	state := &SignState{AccountID: "account-a", LastResult: "pending"}
	if err := store.Save(state); err != nil {
		t.Fatal(err)
	}
	state.LastResult = "success"
	if err := store.Save(state); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load("account-a")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.LastResult != "success" {
		t.Fatalf("unexpected state: %#v", loaded)
	}
}

func TestFileStoreQuarantinesAnotherAccountsState(t *testing.T) {
	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(&SignState{
		AccountID:    "account-a",
		LastSignDate: "2026-08-02",
	}); err != nil {
		t.Fatal(err)
	}

	loaded, err := store.Load("account-b")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.AccountID != "account-b" || loaded.LastSignDate != "" {
		t.Fatalf("state leaked across accounts: %#v", loaded)
	}
	matches, err := filepath.Glob(store.statePath() + ".account-mismatch-*")
	if err != nil || len(matches) != 1 {
		t.Fatalf("expected mismatch quarantine, matches=%v err=%v", matches, err)
	}
}
