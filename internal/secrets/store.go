package secrets

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Store struct{ Dir string }

func (s Store) Write(name, value string) error {
	if err := validateName(name); err != nil {
		return err
	}
	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("secret %s must be a single line", name)
	}
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return err
	}
	path := filepath.Join(s.Dir, name)
	tmp, err := os.CreateTemp(s.Dir, ".secret-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.WriteString(value); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	return nil
}

func (s Store) Read(name string) (string, error) {
	if err := validateName(name); err != nil {
		return "", err
	}
	raw, err := os.ReadFile(filepath.Join(s.Dir, name))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(raw)), nil
}

func (s Store) Exists(name string) bool {
	if validateName(name) != nil {
		return false
	}
	_, err := os.Stat(filepath.Join(s.Dir, name))
	return err == nil
}

func validateName(name string) error {
	if name == "" || filepath.Base(name) != name || strings.ContainsAny(name, `/\\`) {
		return fmt.Errorf("invalid secret name %q", name)
	}
	return nil
}

func Redact(text string, values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			text = strings.ReplaceAll(text, value, "[REDACTED]")
		}
	}
	return text
}
