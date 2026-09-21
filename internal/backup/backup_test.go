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

// Restore accepts the three managed trees, including under the production root
// "/", and refuses everything else. Confining to the root alone is vacuous at
// "/", so these cases are what actually stops a tampered archive.
func TestSafeTarget(t *testing.T) {
	// Restore always resolves the root first; on Windows that is the volume root.
	root, err := filepath.Abs(string(filepath.Separator))
	if err != nil {
		t.Fatal(err)
	}
	paths := layout.New(root)

	for _, name := range []string{
		"etc/yamsplus/yamsplus.yaml",
		"etc/yamsplus/secrets/radarr_api_key",
		"var/lib/yamsplus/state.json",
		"opt/yamsplus/compose.yaml",
	} {
		resolved, err := safeTarget(paths, root, name)
		if err != nil {
			t.Fatalf("managed path %q rejected under the production root: %v", name, err)
		}
		if want := filepath.Join(root, filepath.FromSlash(name)); resolved != want {
			t.Fatalf("resolved = %q, want %q", resolved, want)
		}
	}

	// Everything outside the managed trees must be refused even though it is
	// trivially "below" the production root.
	for _, name := range []string{
		".",
		"../root/.ssh/authorized_keys",
		"../../../etc/cron.d/yams",
		"etc/shadow",
		"etc/yamsplus-not-ours/file",
		"usr/local/bin/yamsplus",
	} {
		if _, err := safeTarget(paths, root, name); err == nil {
			t.Fatalf("entry %q must be refused under the production root", name)
		}
	}

	// The same holds for a nested test root.
	nested := layout.New(filepath.Join(root, "srv", "sandbox"))
	if _, err := safeTarget(nested, nested.Root, "etc/yamsplus/yamsplus.yaml"); err != nil {
		t.Fatal(err)
	}
	if _, err := safeTarget(nested, nested.Root, "../../etc/shadow"); err == nil {
		t.Fatal("traversal outside the root must be refused")
	}
}
