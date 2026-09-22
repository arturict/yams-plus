package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arturict/yams-plus/internal/config"
	"github.com/arturict/yams-plus/internal/engine"
	"github.com/arturict/yams-plus/internal/layout"
	"github.com/arturict/yams-plus/internal/secrets"
	"github.com/arturict/yams-plus/internal/state"
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

// uninstall --remove-media is refused in the beta. It has to refuse before
// running compose down, or the host is left with no containers and a full
// configuration directory.
func TestUninstallRefusesRemoveMediaBeforeTouchingAnything(t *testing.T) {
	root := t.TempDir()
	paths := layout.New(root)
	if err := os.MkdirAll(paths.ConfigDir(), 0o750); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(paths.ConfigDir(), "yamsplus.yaml")
	if err := os.WriteFile(marker, []byte("schemaVersion: 1\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	// No compose file exists, so a run that reached compose down would fail with
	// a docker error rather than the refusal below.
	err := run([]string{"--root", root, "uninstall", "--yes", "--remove-media"})
	if err == nil || !strings.Contains(err.Error(), "media deletion requires") {
		t.Fatalf("err = %v, want the media-deletion refusal", err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("configuration was touched despite the refusal: %v", err)
	}
}

// update must re-render from the lock embedded in the running binary. Pulling
// against the compose file already on disk re-pulled the previous binary's
// digests and updated nothing, while the operations guide says update uses the
// reviewed stack.lock.yaml.
func TestUpdateRewritesComposeFromTheEmbeddedLock(t *testing.T) {
	root := t.TempDir()
	paths := layout.New(root)
	configPath := filepath.Join(t.TempDir(), "desired.yaml")
	cfg := config.Default()
	cfg.AdminUsername = "captain"
	cfg.Downloads.Usenet.Host = "news.example.test"
	if err := config.Save(configPath, cfg); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"--root", root, "install", "--config", configPath, "--skip-start"}); err != nil {
		t.Fatal(err)
	}
	current, err := os.ReadFile(paths.ComposeFile())
	if err != nil {
		t.Fatal(err)
	}

	// Stand in for a host installed by an older binary: rewind the rendered
	// compose file so it no longer matches the embedded lock.
	stale := bytes.Replace(current, []byte("image: "), []byte("image: stale-"), 1)
	if bytes.Equal(stale, current) {
		t.Fatal("could not construct a stale compose file")
	}
	if err := os.WriteFile(paths.ComposeFile(), stale, 0o640); err != nil {
		t.Fatal(err)
	}

	// Stop the command at the Docker boundary: the file phase is what this test
	// covers, and a real "compose up" would start the whole stack.
	t.Setenv("PATH", "")
	if err := run([]string{"--root", root, "update", "--yes"}); err == nil {
		t.Fatal("expected update to fail once it reaches Docker")
	}

	after, err := os.ReadFile(paths.ComposeFile())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, current) {
		t.Fatal("update did not restore the compose file from the embedded lock")
	}
}

// A later apply may skip the admin password only when nothing it converges
// needs it. The password is never stored, so qBittorrent needs it on every
// login, and a service converging for the first time needs it to set up its
// own login; without it that service came up with no authentication at all.
func TestAdminPasswordNeeded(t *testing.T) {
	configured := func(names ...string) map[string]string {
		services := map[string]string{}
		for _, name := range names {
			services[name] = "configured"
		}
		return services
	}
	usenet := config.Default()
	torrent := config.Default()
	torrent.Downloads.Mode = "torrent"
	both := config.Default()
	both.Downloads.Mode = "both"
	withBooks := config.Default()
	withBooks.Modules.Books = true
	full := configured("jellyfin", "seerr", "prowlarr", "radarr", "sonarr", "bazarr", "sabnzbd")

	for _, tc := range []struct {
		name     string
		cfg      config.Config
		tokens   []string
		services map[string]string
		want     bool
	}{
		{"fresh install", usenet, nil, nil, true},
		{"every service already configured", usenet, []string{"jellyfin_access_token"}, full, false},
		{"torrent always logs in to qBittorrent", torrent, []string{"jellyfin_access_token"}, configured("jellyfin", "seerr", "prowlarr", "radarr", "sonarr", "bazarr", "qbittorrent"), true},
		{"both always logs in to qBittorrent", both, []string{"jellyfin_access_token"}, configured("jellyfin", "seerr", "prowlarr", "radarr", "sonarr", "bazarr", "sabnzbd", "qbittorrent"), true},
		{"a module added since the last apply", usenet, []string{"jellyfin_access_token"}, configured("jellyfin", "seerr", "prowlarr", "radarr", "sonarr", "sabnzbd"), true},
		{"last convergence failed", usenet, []string{"jellyfin_access_token"}, nil, true},
		{"books without a Shelfmark session", withBooks, []string{"jellyfin_access_token"}, configured("jellyfin", "seerr", "prowlarr", "radarr", "sonarr", "bazarr", "sabnzbd", "shelfmark", "audiobookshelf"), true},
		{"books fully configured", withBooks, []string{"jellyfin_access_token", "shelfmark_session"}, configured("jellyfin", "seerr", "prowlarr", "radarr", "sonarr", "bazarr", "sabnzbd", "shelfmark", "audiobookshelf"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			paths := layout.New(t.TempDir())
			store := secrets.Store{Dir: paths.SecretsDir()}
			for _, token := range tc.tokens {
				if err := store.Write(token, "placeholder"); err != nil {
					t.Fatal(err)
				}
			}
			if tc.services != nil {
				if err := os.MkdirAll(paths.StateDir(), 0o750); err != nil {
					t.Fatal(err)
				}
				if err := state.Save(paths.StateFile(), state.State{Phase: "configured", Services: tc.services}); err != nil {
					t.Fatal(err)
				}
			}
			if got := adminPasswordNeeded(paths, tc.cfg); got != tc.want {
				t.Fatalf("adminPasswordNeeded = %v, want %v", got, tc.want)
			}
		})
	}
}

// The wizard collects the admin password and every provider credential, and
// none of it survives a failed run. A host problem that does not depend on the
// answers must stop the install before the first question.
func TestInstallChecksTheHostBeforeTheWizard(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	_ = writer.Close()
	stdin := os.Stdin
	os.Stdin = reader
	t.Cleanup(func() { os.Stdin = stdin; _ = reader.Close() })

	t.Setenv("PATH", "")
	err = run([]string{"--root", t.TempDir(), "install"})
	if err == nil || !strings.Contains(err.Error(), "preflight failed") || !strings.Contains(err.Error(), "docker") {
		t.Fatalf("expected the missing Docker to stop the install before the wizard, got %v", err)
	}
}

// Backups include the applications' SQLite databases, which change while the
// containers run. A backup of an installed stack must stop the services first,
// and must refuse rather than silently archive them running when it cannot.
func TestBackupRefusesToArchiveServicesItCannotStop(t *testing.T) {
	withStdin := func(t *testing.T, input string) {
		reader, writer, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := writer.WriteString(input); err != nil {
			t.Fatal(err)
		}
		_ = writer.Close()
		stdin := os.Stdin
		os.Stdin = reader
		t.Cleanup(func() { os.Stdin = stdin; _ = reader.Close() })
	}
	root := t.TempDir()
	configPath := filepath.Join(t.TempDir(), "desired.yaml")
	cfg := config.Default()
	cfg.AdminUsername = "captain"
	cfg.Downloads.Usenet.Host = "news.example.test"
	if err := config.Save(configPath, cfg); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"--root", root, "install", "--config", configPath, "--skip-start"}); err != nil {
		t.Fatal(err)
	}
	// Without Docker the running services cannot be listed or stopped.
	t.Setenv("PATH", "")

	archive := filepath.Join(t.TempDir(), "backup.tar.gz.age")
	withStdin(t, "correct horse battery staple\n")
	err := run([]string{"--root", root, "backup", "--output", archive})
	if err == nil || !strings.Contains(err.Error(), "--live") {
		t.Fatalf("expected the backup to refuse and name --live, got %v", err)
	}
	if _, err := os.Stat(archive); !os.IsNotExist(err) {
		t.Fatalf("a refused backup left an archive behind: %v", err)
	}

	withStdin(t, "correct horse battery staple\n")
	if err := run([]string{"--root", root, "backup", "--live", "--output", archive}); err != nil {
		t.Fatalf("--live must archive without Docker: %v", err)
	}
	if _, err := os.Stat(archive); err != nil {
		t.Fatal(err)
	}
}
