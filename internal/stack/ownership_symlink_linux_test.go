//go:build linux

package stack

import (
	"os"
	"path/filepath"
	"strings"
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

// The media tree is mode 0770, owned by PUID and bind-mounted into Radarr,
// Sonarr and the download clients, so a container can replace a managed
// directory with a symlink. Every apply then ran os.Chmod and os.Chown on the
// link, which act on its target anywhere on the host.
func TestEnsureDirectoriesRefusesSymlinkOutOfManagedTree(t *testing.T) {
	paths := layout.New(t.TempDir())
	cfg := config.Default()
	cfg.Runtime.PUID, cfg.Runtime.PGID = os.Getuid(), os.Getgid()
	if err := EnsureRenderDirectories(cfg, paths); err != nil {
		t.Fatal(err)
	}
	victim := filepath.Join(t.TempDir(), "victim")
	if err := os.Mkdir(victim, 0o700); err != nil {
		t.Fatal(err)
	}
	movies := filepath.Join(paths.DataDir(cfg.DataRoot), "library", "movies")
	if err := os.Remove(movies); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(victim, movies); err != nil {
		t.Fatal(err)
	}

	for name, ensure := range map[string]func(config.Config, layout.Layout) error{
		"render": EnsureRenderDirectories,
		"apply":  EnsureDirectories,
	} {
		if err := ensure(cfg, paths); err == nil || !strings.Contains(err.Error(), "escapes") {
			t.Errorf("%s: a symlink leaving the media tree must be refused, got %v", name, err)
		}
		info, err := os.Stat(victim)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != 0o700 {
			t.Fatalf("%s: victim mode changed through the symlink: %o", name, got)
		}
	}
}

// A parent component is as dangerous as the leaf: MkdirAll and Chmod resolve
// library -> /elsewhere before they reach movies.
func TestEnsureDirectoriesRefusesSymlinkedParent(t *testing.T) {
	paths := layout.New(t.TempDir())
	cfg := config.Default()
	cfg.Runtime.PUID, cfg.Runtime.PGID = os.Getuid(), os.Getgid()
	if err := EnsureRenderDirectories(cfg, paths); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	library := filepath.Join(paths.DataDir(cfg.DataRoot), "library")
	if err := os.RemoveAll(library); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, library); err != nil {
		t.Fatal(err)
	}
	if err := EnsureDirectories(cfg, paths); err == nil || !strings.Contains(err.Error(), "escapes") {
		t.Fatalf("a symlinked parent leaving the media tree must be refused, got %v", err)
	}
	if _, err := os.Lstat(filepath.Join(outside, "movies")); !os.IsNotExist(err) {
		t.Fatalf("a directory was created outside the managed tree: %v", err)
	}
}
