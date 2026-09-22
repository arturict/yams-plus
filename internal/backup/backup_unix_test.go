//go:build !windows

package backup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arturict/yams-plus/internal/layout"
)

// A backup that cannot read a source tree must fail and must not leave a
// partial encrypted archive behind.
func TestCreateFailsAndRemovesArchiveOnUnreadableTree(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory permissions")
	}
	root := t.TempDir()
	paths := layout.New(root)
	if err := os.MkdirAll(paths.ConfigDir(), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.ConfigFile(), []byte("schemaVersion: 1\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	parent := filepath.Dir(paths.ConfigDir())
	if err := os.Chmod(parent, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(parent, 0o750) })

	destination := filepath.Join(t.TempDir(), "backup.tar.gz.age")
	if err := Create(paths, destination, "correct horse battery staple", true); err == nil {
		t.Fatal("expected an unreadable configuration tree to fail the backup")
	}
	if _, err := os.Stat(destination); !os.IsNotExist(err) {
		t.Fatalf("failed backup left %s behind: %v", destination, err)
	}
}

// The application containers can create entries under the state directory, so
// a directory component there may already be a symlink when restore runs.
// O_NOFOLLOW only guards the last path element, and os.MkdirAll resolves the
// rest, so a planted apps/radarr -> /elsewhere redirected a root-owned write
// out of the managed trees.
func TestRestoreRefusesSymlinkedDirectoryComponent(t *testing.T) {
	source := layout.New(t.TempDir())
	radarr := filepath.Join(source.AppsDir(), "radarr")
	if err := os.MkdirAll(radarr, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(radarr, "config.xml"), []byte("<Config/>\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(t.TempDir(), "backup.tar.gz.age")
	const passphrase = "correct horse battery staple"
	if err := Create(source, archive, passphrase, false); err != nil {
		t.Fatal(err)
	}

	target := layout.New(t.TempDir())
	outside := t.TempDir()
	if err := os.MkdirAll(target.AppsDir(), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(target.AppsDir(), "radarr")); err != nil {
		t.Fatal(err)
	}

	err := Restore(target, archive, passphrase)
	if err == nil {
		t.Fatal("a symlinked directory component leaving the managed tree must be refused")
	}
	if !strings.Contains(err.Error(), "escapes") {
		t.Fatalf("restore failed for the wrong reason: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(outside, "config.xml")); !os.IsNotExist(err) {
		t.Fatalf("restore wrote outside the managed tree: %v", err)
	}
}
