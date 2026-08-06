package main

import (
	"testing"

	"github.com/arturict/yams-plus/internal/config"
	"github.com/arturict/yams-plus/internal/engine"
	"github.com/arturict/yams-plus/internal/layout"
	"github.com/arturict/yams-plus/internal/secrets"
)

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
