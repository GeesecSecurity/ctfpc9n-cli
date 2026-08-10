package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var namePattern = regexp.MustCompile(`^[A-Za-z0-9-]{1,64}$`)

type Value struct {
	APIBase   string    `json:"apiBase"`
	Token     string    `json:"token"`
	CreatedAt time.Time `json:"createdAt"`
}

func ValidateName(name string) error {
	if !namePattern.MatchString(name) {
		return fmt.Errorf("session name must match %s", namePattern.String())
	}
	return nil
}

func StateDir() (string, error) {
	if value := strings.TrimSpace(os.Getenv("CTFPC9N_STATE_DIR")); value != "" {
		if !filepath.IsAbs(value) {
			return "", errors.New("CTFPC9N_STATE_DIR must be an absolute path")
		}
		return filepath.Clean(value), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".local", "state", "ctfpc9n-cli"), nil
}

func Save(name string, value Value) (Value, error) {
	if err := ValidateName(name); err != nil {
		return Value{}, err
	}
	value.APIBase = strings.TrimRight(strings.TrimSpace(value.APIBase), "/")
	value.Token = strings.TrimSpace(value.Token)
	if value.APIBase == "" || value.Token == "" {
		return Value{}, errors.New("session API base and token are required")
	}
	if value.CreatedAt.IsZero() {
		value.CreatedAt = time.Now().UTC()
	} else {
		value.CreatedAt = value.CreatedAt.UTC()
	}
	directory, err := sessionsDir()
	if err != nil {
		return Value{}, err
	}
	path := filepath.Join(directory, name+".json")
	if err := rejectUnsafeFile(path, true); err != nil {
		return Value{}, err
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return Value{}, fmt.Errorf("encode session: %w", err)
	}
	temporary, err := os.CreateTemp(directory, ".session-*")
	if err != nil {
		return Value{}, fmt.Errorf("create temporary session: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return Value{}, fmt.Errorf("secure temporary session: %w", err)
	}
	if _, err := temporary.Write(append(encoded, '\n')); err != nil {
		temporary.Close()
		return Value{}, fmt.Errorf("write session: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return Value{}, fmt.Errorf("close session: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return Value{}, fmt.Errorf("replace session: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return Value{}, fmt.Errorf("secure session: %w", err)
	}
	return value, nil
}

func Load(name string) (Value, error) {
	if err := ValidateName(name); err != nil {
		return Value{}, err
	}
	directory, err := sessionsDir()
	if err != nil {
		return Value{}, err
	}
	path := filepath.Join(directory, name+".json")
	if err := rejectUnsafeFile(path, false); err != nil {
		return Value{}, err
	}
	encoded, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Value{}, fmt.Errorf("session %q does not exist", name)
		}
		return Value{}, fmt.Errorf("read session: %w", err)
	}
	var value Value
	if err := json.Unmarshal(encoded, &value); err != nil {
		return Value{}, fmt.Errorf("decode session: %w", err)
	}
	value.APIBase = strings.TrimRight(strings.TrimSpace(value.APIBase), "/")
	value.Token = strings.TrimSpace(value.Token)
	if value.APIBase == "" || value.Token == "" || value.CreatedAt.IsZero() {
		return Value{}, errors.New("session is incomplete")
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return Value{}, fmt.Errorf("secure session: %w", err)
	}
	return value, nil
}

func Delete(name string) (bool, error) {
	if err := ValidateName(name); err != nil {
		return false, err
	}
	directory, err := sessionsDir()
	if err != nil {
		return false, err
	}
	path := filepath.Join(directory, name+".json")
	if err := rejectUnsafeFile(path, false); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	if err := os.Remove(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("delete session: %w", err)
	}
	return true, nil
}

func sessionsDir() (string, error) {
	stateDir, err := StateDir()
	if err != nil {
		return "", err
	}
	if err := ensurePrivateDir(stateDir); err != nil {
		return "", err
	}
	directory := filepath.Join(stateDir, "sessions")
	if err := ensurePrivateDir(directory); err != nil {
		return "", err
	}
	return directory, nil
}

func ensurePrivateDir(directory string) error {
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create state directory: %w", err)
	}
	info, err := os.Lstat(directory)
	if err != nil {
		return fmt.Errorf("inspect state directory: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("state directory %q is not a directory", directory)
	}
	if err := os.Chmod(directory, 0o700); err != nil {
		return fmt.Errorf("secure state directory: %w", err)
	}
	return nil
}

func rejectUnsafeFile(path string, allowMissing bool) error {
	info, err := os.Lstat(path)
	if err != nil {
		if allowMissing && errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("session file %q is not a regular file", path)
	}
	return nil
}
