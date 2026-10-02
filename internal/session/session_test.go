package session

import (
	"os"
	"path/filepath"
	"strings"
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

func TestSaveRejectsCrossKindReplacement(t *testing.T) {
	for _, originalKind := range []string{"", "competition", "evaluation"} {
		t.Run("original-"+originalKind, func(t *testing.T) {
			state := t.TempDir()
			t.Setenv("CTFPC9N_STATE_DIR", state)
			original := `{"apiBase":"https://original.example","token":"original-secret","createdAt":"2026-10-02T00:00:00Z"}`
			if originalKind != "" {
				original = strings.Replace(original, `{`, `{"kind":"`+originalKind+`",`, 1)
			}
			directory, err := sessionsDir()
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(directory, "shared.json")
			if err = os.WriteFile(path, []byte(original), 0o600); err != nil {
				t.Fatal(err)
			}
			nextKind := "evaluation"
			if originalKind == "evaluation" {
				nextKind = "competition"
			}
			_, err = Save("shared", Value{Kind: nextKind, APIBase: "https://new.example", Token: "new-secret"})
			if err == nil {
				t.Fatal("cross-kind login overwrote the existing session")
			}
			if strings.Contains(err.Error(), "secret") {
				t.Fatal("error exposed token")
			}
			got, err := os.ReadFile(path)
			if err != nil || string(got) != original {
				t.Fatalf("existing session changed: %v", err)
			}
		})
	}
}

func TestSaveAllowsSameKindReauthentication(t *testing.T) {
	t.Setenv("CTFPC9N_STATE_DIR", t.TempDir())
	for _, kind := range []string{"competition", "evaluation"} {
		for _, token := range []string{"before", "after"} {
			if _, err := Save(kind, Value{Kind: kind, APIBase: "https://example.test", Token: token}); err != nil {
				t.Fatal(err)
			}
		}
		got, err := Load(kind)
		if err != nil || got.Token != "after" {
			t.Fatalf("same-kind reauthentication failed: %v", err)
		}
	}
}
