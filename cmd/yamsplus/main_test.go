package main

import (
	"os"
	"path/filepath"
	"strings"
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

// A configuration without bind addresses used to index BindAddresses[0] and
// panic; it must now be rejected with a readable error.
func TestPluginAuditRejectsConfigWithoutBindAddress(t *testing.T) {
	root := t.TempDir()
	paths := layout.New(root)
	if err := os.MkdirAll(paths.ConfigDir(), 0o750); err != nil {
		t.Fatal(err)
	}
	raw := "schemaVersion: 1\nadminUsername: captain\nbindAddresses: []\ndownloads:\n  usenet:\n    host: news.example.test\n"
	if err := os.WriteFile(paths.ConfigFile(), []byte(raw), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := (secrets.Store{Dir: paths.SecretsDir()}).Write("jellyfin_access_token", "token"); err != nil {
		t.Fatal(err)
	}
	err := run([]string{"--root", root, "plugins", "audit"})
	if err == nil || !strings.Contains(err.Error(), "bindAddresses") {
		t.Fatalf("err = %v, want a bindAddresses validation error", err)
	}
}

// The production install uses root "/", where naive prefix matching used to
// reject every managed directory and break uninstall. The guard is checked
// without removing anything, so the test never touches a real install.
func TestResolveUnderAcceptsProductionRootAndRejectsEscapes(t *testing.T) {
	paths := layout.New(string(filepath.Separator))
	managed := filepath.Join(paths.ConfigDir(), "yamsplus.yaml")
	resolved, err := resolveUnder(paths.Root, managed)
	if err != nil {
		t.Fatalf("production root rejected a managed path: %v", err)
	}
	if resolved != filepath.Clean(managed) {
		t.Fatalf("resolved = %q, want %q", resolved, filepath.Clean(managed))
	}
	if _, err := resolveUnder(paths.Root, paths.Root); err == nil {
		t.Fatal("removing the root itself must be refused")
	}

	root := t.TempDir()
	if _, err := resolveUnder(root, filepath.Join(filepath.Dir(root), "elsewhere")); err == nil {
		t.Fatal("removing a sibling of the root must be refused")
	}
	if _, err := resolveUnder(root, filepath.Join(root, "..", "escape")); err == nil {
		t.Fatal("traversal out of the root must be refused")
	}
	if _, err := resolveUnder(root, filepath.Join(root, "etc", "yamsplus")); err != nil {
		t.Fatalf("a path below the root was refused: %v", err)
	}
}

// safeRemove deletes only inside the root and reports an escape instead.
func TestSafeRemoveDeletesInsideRootOnly(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "etc", "yamsplus")
	if err := os.MkdirAll(inside, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := safeRemove(root, inside); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(inside); !os.IsNotExist(err) {
		t.Fatalf("target was not removed: %v", err)
	}

	sibling := filepath.Join(t.TempDir(), "keep")
	if err := os.MkdirAll(sibling, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := safeRemove(root, sibling); err == nil {
		t.Fatal("removing outside the root must be refused")
	}
	if _, err := os.Stat(sibling); err != nil {
		t.Fatalf("a refused target was removed anyway: %v", err)
	}
}
