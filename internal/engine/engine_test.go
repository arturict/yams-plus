package engine

import (
	"context"
	"os"
	"testing"

	"github.com/arturict/yams-plus/internal/config"
	"github.com/arturict/yams-plus/internal/layout"
	"github.com/arturict/yams-plus/internal/secrets"
	"github.com/arturict/yams-plus/internal/state"
)

func engineTestConfig() config.Config {
	cfg := config.Default()
	cfg.AdminUsername = "captain"
	cfg.Downloads.Usenet.Host = "news.example.test"
	return cfg
}

func TestValidateRequiredSecretsRejectsEmptyValues(t *testing.T) {
	cfg := engineTestConfig()
	store := secrets.Store{Dir: t.TempDir()}
	for _, requirement := range RequiredSecrets(cfg) {
		if err := store.Write(requirement.Name, ""); err != nil {
			t.Fatal(err)
		}
	}
	if err := validateRequiredSecrets(cfg, store); err == nil {
		t.Fatal("expected empty required secrets to fail validation")
	}
	for _, requirement := range RequiredSecrets(cfg) {
		if err := store.Write(requirement.Name, "configured"); err != nil {
			t.Fatal(err)
		}
	}
	if err := validateRequiredSecrets(cfg, store); err != nil {
		t.Fatal(err)
	}
}

func TestApplySkipStartConvergesFilesAndState(t *testing.T) {
	paths := layout.New(t.TempDir())
	eng := Engine{Options: Options{Paths: paths, SkipPreflight: true, SkipStart: true}}
	if err := eng.Apply(context.Background(), engineTestConfig()); err != nil {
		t.Fatal(err)
	}
	current, err := state.Load(paths.StateFile())
	if err != nil {
		t.Fatal(err)
	}
	if current.Phase != "rendered" {
		t.Fatalf("phase=%q", current.Phase)
	}
	changes, err := eng.Plan(engineTestConfig())
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range changes {
		if change.Action != "unchanged" {
			t.Errorf("%s = %s", change.Path, change.Action)
		}
	}
}

func TestDryRunDoesNotWrite(t *testing.T) {
	paths := layout.New(t.TempDir())
	eng := Engine{Options: Options{Paths: paths, SkipPreflight: true, DryRun: true}}
	if err := eng.Apply(context.Background(), engineTestConfig()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(paths.ConfigFile()); !os.IsNotExist(err) {
		t.Fatalf("dry run wrote config: %v", err)
	}
}
