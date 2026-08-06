//go:build linux

package stack

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/arturict/yams-plus/internal/layout"
)

func TestEnsureDirectoriesOwnsWritableTrees(t *testing.T) {
	cfg := testConfig()
	cfg.Runtime.PUID = os.Getuid()
	cfg.Runtime.PGID = os.Getgid()
	paths := layout.New(t.TempDir())
	if err := EnsureDirectories(cfg, paths); err != nil {
		t.Fatal(err)
	}
	dataRoot := paths.DataDir(cfg.DataRoot)
	for _, path := range []string{
		paths.AppsDir(), dataRoot,
		filepath.Join(dataRoot, "downloads"),
		filepath.Join(dataRoot, "downloads", "usenet"),
		filepath.Join(dataRoot, "library"),
	} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		stat := info.Sys().(*syscall.Stat_t)
		if int(stat.Uid) != cfg.Runtime.PUID || int(stat.Gid) != cfg.Runtime.PGID {
			t.Fatalf("%s ownership = %d:%d", path, stat.Uid, stat.Gid)
		}
		if within(path, dataRoot) && info.Mode().Perm() != 0o770 {
			t.Fatalf("%s mode = %o", path, info.Mode().Perm())
		}
	}
}

func TestEnsureRuntimeOwnershipOwnsGeneratedRecyclarrTree(t *testing.T) {
	cfg := testConfig()
	cfg.Runtime.PUID = os.Getuid()
	cfg.Runtime.PGID = os.Getgid()
	paths := layout.New(t.TempDir())
	if err := EnsureDirectories(cfg, paths); err != nil {
		t.Fatal(err)
	}
	files, err := Render(cfg, paths)
	if err != nil {
		t.Fatal(err)
	}
	if err := Write(files); err != nil {
		t.Fatal(err)
	}
	if err := EnsureRuntimeOwnership(cfg, paths); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{paths.RecyclarrDir(), filepath.Join(paths.RecyclarrDir(), "configs"), filepath.Join(paths.RecyclarrDir(), "configs", "radarr.yaml")} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		stat := info.Sys().(*syscall.Stat_t)
		if int(stat.Uid) != cfg.Runtime.PUID || int(stat.Gid) != cfg.Runtime.PGID {
			t.Fatalf("%s ownership = %d:%d", path, stat.Uid, stat.Gid)
		}
	}
}
