//go:build !windows

package main

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/arturict/yams-plus/internal/config"
	"github.com/arturict/yams-plus/internal/layout"
)

// update rewrites the Recyclarr configuration as root, and the Recyclarr
// container reads it as PUID. Changing ownership needs root, so this only
// runs as root; run it with sudo after touching update.
func TestUpdateHandsRecyclarrConfigurationToPUID(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("changing ownership needs root")
	}
	root := t.TempDir()
	paths := layout.New(root)
	configPath := filepath.Join(t.TempDir(), "desired.yaml")
	cfg := config.Default()
	cfg.AdminUsername = "captain"
	cfg.Downloads.Usenet.Host = "news.example.test"
	cfg.Runtime.PUID, cfg.Runtime.PGID = 65534, 65534
	if err := config.Save(configPath, cfg); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"--root", root, "install", "--config", configPath, "--skip-start"}); err != nil {
		t.Fatal(err)
	}
	// A newer lock rewrites these files; a missing one stands in for that.
	radarr := filepath.Join(paths.RecyclarrDir(), "configs", "radarr.yaml")
	if err := os.Remove(radarr); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", "")
	if err := run([]string{"--root", root, "update", "--yes"}); err == nil {
		t.Fatal("expected update to fail once it reaches Docker")
	}
	info, err := os.Stat(radarr)
	if err != nil {
		t.Fatal(err)
	}
	if stat := info.Sys().(*syscall.Stat_t); stat.Uid != 65534 || stat.Gid != 65534 {
		t.Fatalf("%s is owned by %d:%d, want 65534:65534", radarr, stat.Uid, stat.Gid)
	}
}
