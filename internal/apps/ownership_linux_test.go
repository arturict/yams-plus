//go:build linux

package apps

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/arturict/yams-plus/internal/config"
	"github.com/arturict/yams-plus/internal/layout"
)

func TestWriteRecyclarrSecretsOwnsFileForContainerUser(t *testing.T) {
	cfg := config.Default()
	cfg.Runtime.PUID = os.Getuid()
	cfg.Runtime.PGID = os.Getgid()
	paths := layout.New(t.TempDir())
	converger := Converger{Config: cfg, Paths: paths}
	if err := converger.writeRecyclarrSecrets(map[string]*arrState{"radarr": {key: "test-key"}}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(paths.RecyclarrDir(), "secrets.yml"))
	if err != nil {
		t.Fatal(err)
	}
	stat := info.Sys().(*syscall.Stat_t)
	if int(stat.Uid) != cfg.Runtime.PUID || int(stat.Gid) != cfg.Runtime.PGID {
		t.Fatalf("ownership = %d:%d", stat.Uid, stat.Gid)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %o", info.Mode().Perm())
	}
}
