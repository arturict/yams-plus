package secrets

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStoreWritesRestrictedFile(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	if err := s.Write("token", "super-secret"); err != nil {
		t.Fatal(err)
	}
	got, err := s.Read("token")
	if err != nil {
		t.Fatal(err)
	}
	if got != "super-secret" {
		t.Fatalf("got %q", got)
	}
	if os.PathSeparator != '\\' {
		info, err := os.Stat(filepath.Join(s.Dir, "token"))
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("mode = %o", info.Mode().Perm())
		}
	}
}

func TestRedact(t *testing.T) {
	if got := Redact("token=abc123", "abc123"); got != "token=[REDACTED]" {
		t.Fatalf("got %q", got)
	}
}
