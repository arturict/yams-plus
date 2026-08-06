package backup

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/arturict/yams-plus/internal/layout"
)

func TestEncryptedRoundTrip(t *testing.T) {
	sourceRoot := t.TempDir()
	source := layout.New(sourceRoot)
	if err := os.MkdirAll(source.ConfigDir(), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source.ConfigFile(), []byte("schemaVersion: 1\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(t.TempDir(), "backup.age")
	if err := Create(source, archive, "a sufficiently long backup passphrase", false); err != nil {
		t.Fatal(err)
	}
	target := layout.New(t.TempDir())
	if err := Restore(target, archive, "a sufficiently long backup passphrase"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(target.ConfigFile()); err != nil {
		t.Fatal(err)
	}
}
