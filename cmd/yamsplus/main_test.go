package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/arturict/yams-plus/internal/config"
	"github.com/arturict/yams-plus/internal/engine"
	"github.com/arturict/yams-plus/internal/layout"
	"github.com/arturict/yams-plus/internal/secrets"
)

func TestInstallSkipStartDoesNotRequireDocker(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(t.TempDir(), "desired.yaml")
	cfg := config.Default()
	cfg.AdminUsername = "captain"
	cfg.Downloads.Usenet.Host = "news.example.test"
	if err := config.Save(configPath, cfg); err != nil {
		t.Fatal(err)
	}

	// A render-only installation must work on a machine that does not yet have
	// Docker. Runtime validation still runs for a real installation.
	t.Setenv("PATH", "")
	if err := run([]string{"--root", root, "install", "--config", configPath, "--skip-start"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(layout.New(root).ComposeFile()); err != nil {
		t.Fatalf("rendered compose file: %v", err)
	}
}

func TestCollectMissingSecretsUsesEnvironmentAndPreservesStore(t *testing.T) {
	cfg := config.Default()
	cfg.AdminUsername = "captain"
	cfg.Downloads.Usenet.Host = "news.example.test"
	paths := layout.New(t.TempDir())
	requirements := engine.RequiredSecrets(cfg)
	if len(requirements) == 0 {
		t.Fatal("expected required secrets")
	}
	stored := requirements[0]
	if err := (secrets.Store{Dir: paths.SecretsDir()}).Write(stored.Name, "already-configured"); err != nil {
		t.Fatal(err)
	}
	for _, requirement := range requirements[1:] {
		t.Setenv("YAMSPLUS_SECRET_"+secretEnvSuffix(requirement.Name), "from-environment")
	}
	values, err := collectMissingSecrets(paths, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := values[stored.Name]; ok {
		t.Fatal("stored secret should not be collected again")
	}
	if len(values) != len(requirements)-1 {
		t.Fatalf("collected %d secrets, want %d", len(values), len(requirements)-1)
	}
}
