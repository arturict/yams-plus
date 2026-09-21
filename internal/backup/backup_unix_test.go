//go:build !windows

package backup

import (
	"os"
	"path/filepath"
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
