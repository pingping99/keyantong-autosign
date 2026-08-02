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
	if _, err := store.Load(); err == nil {
		t.Fatal("expected decode error")
	}
	matches, err := filepath.Glob(store.statePath() + ".corrupt-*")
	if err != nil || len(matches) != 1 {
		t.Fatalf("expected one quarantined state file, matches=%v err=%v", matches, err)
	}
}
