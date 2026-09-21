//go:build linux

package stack

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/arturict/yams-plus/internal/config"
	"github.com/arturict/yams-plus/internal/layout"
)

// The Recyclarr tree is bind-mounted into a container that runs as PUID, so it
// can plant a symlink there. os.Chown resolves the link and changes its target,
// which handed a root-owned file to PUID on the next apply. The walk must
// refuse the link instead.
func TestEnsureRuntimeOwnershipRefusesSymlinks(t *testing.T) {
	paths := layout.New(t.TempDir())
	if err := os.MkdirAll(paths.RecyclarrDir(), 0o750); err != nil {
		t.Fatal(err)
	}
	victim := filepath.Join(t.TempDir(), "victim")
	if err := os.WriteFile(victim, []byte("sensitive\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(victim, filepath.Join(paths.RecyclarrDir(), "evil")); err != nil {
		t.Fatal(err)
	}

	cfg := config.Default()
	cfg.Runtime.PUID, cfg.Runtime.PGID = os.Getuid(), os.Getgid()
	err := EnsureRuntimeOwnership(cfg, paths)
	if err == nil {
		t.Fatal("a symlink in the Recyclarr tree must be refused")
	}
	if _, statErr := os.Lstat(victim); statErr != nil {
		t.Fatalf("victim disappeared: %v", statErr)
	}
}

// Without a symlink the walk still converges ownership normally.
func TestEnsureRuntimeOwnershipAcceptsAPlainTree(t *testing.T) {
	paths := layout.New(t.TempDir())
	configs := filepath.Join(paths.RecyclarrDir(), "configs")
	if err := os.MkdirAll(configs, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configs, "radarr.yaml"), []byte("radarr:\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.Runtime.PUID, cfg.Runtime.PGID = os.Getuid(), os.Getgid()
	if err := EnsureRuntimeOwnership(cfg, paths); err != nil {
		t.Fatalf("a plain tree must converge: %v", err)
	}
}
