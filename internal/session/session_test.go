package session

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveLoadDeleteUsesPrivateStateFiles(t *testing.T) {
	state := filepath.Join(t.TempDir(), "state")
	t.Setenv("CTFPC9N_STATE_DIR", state)
	saved, err := Save("contest-a", Value{APIBase: "https://competition.example/agent/v1/", Token: "test-token"})
	if err != nil {
		t.Fatal(err)
	}
	if saved.APIBase != "https://competition.example/agent/v1" || saved.CreatedAt.IsZero() {
		t.Fatalf("saved = %#v", saved)
	}
	loaded, err := Load("contest-a")
	if err != nil {
		t.Fatal(err)
	}
	if loaded != saved {
		t.Fatalf("loaded = %#v, saved = %#v", loaded, saved)
	}
	for _, path := range []string{state, filepath.Join(state, "sessions")} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o700 {
			t.Fatalf("directory %s mode = %o", path, info.Mode().Perm())
		}
	}
	info, err := os.Stat(filepath.Join(state, "sessions", "contest-a.json"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("session mode = %o", info.Mode().Perm())
	}
	deleted, err := Delete("contest-a")
	if err != nil || !deleted {
		t.Fatalf("Delete() = %t, %v", deleted, err)
	}
	deleted, err = Delete("contest-a")
	if err != nil || deleted {
		t.Fatalf("second Delete() = %t, %v", deleted, err)
	}
}

func TestRejectsInvalidSessionNameAndRelativeStateDirectory(t *testing.T) {
	if err := ValidateName("../escape"); err == nil {
		t.Fatal("unsafe session name was accepted")
	}
	t.Setenv("CTFPC9N_STATE_DIR", "relative-state")
	if _, err := StateDir(); err == nil {
		t.Fatal("relative CTFPC9N_STATE_DIR was accepted")
	}
}
