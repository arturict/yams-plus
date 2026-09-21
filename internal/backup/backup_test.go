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

// Restore resolves archive entries below the install root, including the
// production root "/", and rejects traversal.
func TestSafeTarget(t *testing.T) {
	// Restore always resolves the root first; on Windows that is the volume root.
	root, err := filepath.Abs(string(filepath.Separator))
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := safeTarget(root, "etc/yamsplus/yamsplus.yaml")
	if err != nil {
		t.Fatalf("production root rejected a managed path: %v", err)
	}
	if want := filepath.Join(root, "etc", "yamsplus", "yamsplus.yaml"); resolved != want {
		t.Fatalf("resolved = %q, want %q", resolved, want)
	}
	if _, err := safeTarget(root, "."); err == nil {
		t.Fatal("an entry resolving to the root itself must be refused")
	}

	nested := filepath.Join(root, "srv", "yamsplus")
	if _, err := safeTarget(nested, "etc/yamsplus.yaml"); err != nil {
		t.Fatal(err)
	}
	if _, err := safeTarget(nested, "../../etc/shadow"); err == nil {
		t.Fatal("traversal outside the root must be refused")
	}
}
