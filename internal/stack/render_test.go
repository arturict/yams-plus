package stack

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arturict/yams-plus/internal/config"
	"github.com/arturict/yams-plus/internal/layout"
	"gopkg.in/yaml.v3"
)

func testConfig() config.Config {
	cfg := config.Default()
	cfg.AdminUsername = "captain"
	cfg.Downloads.Usenet.Host = "news.example.test"
	return cfg
}

func TestRenderModuleAndNetworkMatrix(t *testing.T) {
	tests := []struct {
		name       string
		configure  func(*config.Config)
		want       []string
		doNotWant  []string
		vpnQBitNet bool
	}{
		{
			name: "movies-only-usenet",
			configure: func(cfg *config.Config) {
				cfg.Modules = config.Modules{Movies: true}
			},
			want: []string{"jellyfin", "seerr", "prowlarr", "radarr", "sabnzbd"}, doNotWant: []string{"sonarr", "bazarr", "qbittorrent", "gluetun", "shelfmark", "audiobookshelf"},
		},
		{
			name: "series-only-torrent-vpn",
			configure: func(cfg *config.Config) {
				cfg.Modules = config.Modules{Series: true, Subtitles: true}
				cfg.Downloads.Mode = "torrent"
				cfg.Downloads.Torrent.UseVPN = true
			},
			want: []string{"jellyfin", "seerr", "prowlarr", "sonarr", "bazarr", "qbittorrent", "gluetun"}, doNotWant: []string{"radarr", "sabnzbd", "shelfmark", "audiobookshelf"}, vpnQBitNet: true,
		},
		{
			name: "both-with-books",
			configure: func(cfg *config.Config) {
				cfg.Modules.Books = true
				cfg.Downloads.Mode = "both"
				cfg.Downloads.Usenet.UseVPN = true
				cfg.Downloads.Torrent.UseVPN = true
			},
			want:       []string{"jellyfin", "seerr", "prowlarr", "radarr", "sonarr", "bazarr", "sabnzbd", "qbittorrent", "gluetun", "shelfmark", "audiobookshelf"},
			vpnQBitNet: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := testConfig()
			tt.configure(&cfg)
			files, err := Render(cfg, layout.New(t.TempDir()))
			if err != nil {
				t.Fatal(err)
			}
			var compose struct {
				Services map[string]struct {
					NetworkMode string   `yaml:"network_mode"`
					Ports       []string `yaml:"ports"`
				} `yaml:"services"`
			}
			if err := yaml.Unmarshal(files[0].Content, &compose); err != nil {
				t.Fatal(err)
			}
			for _, service := range tt.want {
				if _, ok := compose.Services[service]; !ok {
					t.Errorf("missing service %s", service)
				}
			}
			for _, service := range tt.doNotWant {
				if _, ok := compose.Services[service]; ok {
					t.Errorf("unexpected service %s", service)
				}
			}
			if tt.vpnQBitNet {
				qbit := compose.Services["qbittorrent"]
				if qbit.NetworkMode != "service:gluetun" || len(qbit.Ports) != 0 {
					t.Errorf("qBittorrent must share Gluetun networking without direct ports: %#v", qbit)
				}
			}
		})
	}
}

func TestRenderUsesDigestsAndPrivateBinding(t *testing.T) {
	files, err := Render(testConfig(), layout.New(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	compose := string(files[0].Content)
	if strings.Contains(compose, ":latest") {
		t.Fatal("compose contains latest tag")
	}
	if !strings.Contains(compose, "@sha256:") {
		t.Fatal("compose does not contain digest pins")
	}
	if !strings.Contains(compose, "127.0.0.1:8096:8096") {
		t.Fatal("Jellyfin is not privately bound")
	}
	if strings.Contains(compose, "qbittorrent:") {
		t.Fatal("qBittorrent rendered for usenet-only configuration")
	}
}

func TestWriteIsIdempotent(t *testing.T) {
	root := t.TempDir()
	paths := layout.New(root)
	files, err := Render(testConfig(), paths)
	if err != nil {
		t.Fatal(err)
	}
	if err := Write(files); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(paths.ComposeFile())
	if err != nil {
		t.Fatal(err)
	}
	if err := Write(files); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(paths.ComposeFile())
	if err != nil {
		t.Fatal(err)
	}
	if !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("unchanged compose was rewritten")
	}
	changes, err := Plan(files)
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range changes {
		if change.Action != "unchanged" {
			t.Fatalf("%s = %s", filepath.Base(change.Path), change.Action)
		}
	}
}

func TestWithinRejectsSiblingPrefix(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "srv", "yamsplus")
	if !within(filepath.Join(root, "library", "movies"), root) {
		t.Fatal("expected descendant to be within root")
	}
	if within(root+"-other", root) {
		t.Fatal("sibling sharing a prefix must not be within root")
	}
}

// Disabling a module has to remove the file it rendered. Leaving radarr.yaml
// behind kept Recyclarr syncing a profile to an application the stack no
// longer runs, and plan never reported it.
func TestStaleReportsFilesThatLeftTheDesiredState(t *testing.T) {
	paths := layout.New(t.TempDir())
	cfg := config.Default()
	cfg.AdminUsername = "captain"
	cfg.Downloads.Usenet.Host = "news.example.test"
	cfg.Modules.Movies = true
	cfg.Modules.Series = true

	files, err := Render(cfg, paths)
	if err != nil {
		t.Fatal(err)
	}
	if err := Write(files); err != nil {
		t.Fatal(err)
	}
	radarr := filepath.Join(paths.RecyclarrDir(), "configs", "radarr.yaml")
	if _, err := os.Stat(radarr); err != nil {
		t.Fatalf("radarr.yaml was not rendered: %v", err)
	}
	if stale := Stale(paths, files); len(stale) != 0 {
		t.Fatalf("nothing should be stale yet, got %v", stale)
	}

	cfg.Modules.Movies = false
	files, err = Render(cfg, paths)
	if err != nil {
		t.Fatal(err)
	}
	stale := Stale(paths, files)
	if len(stale) != 1 || stale[0] != radarr {
		t.Fatalf("stale = %v, want just %s", stale, radarr)
	}
	if err := Write(files); err != nil {
		t.Fatal(err)
	}
	if err := Remove(stale); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(radarr); !os.IsNotExist(err) {
		t.Fatalf("radarr.yaml survived a disabled movies module: %v", err)
	}
	// Sonarr is still enabled and must be untouched.
	if _, err := os.Stat(filepath.Join(paths.RecyclarrDir(), "configs", "sonarr.yaml")); err != nil {
		t.Fatalf("sonarr.yaml was removed as collateral: %v", err)
	}
	// And the next round has nothing left to do.
	if stale := Stale(paths, files); len(stale) != 0 {
		t.Fatalf("removal did not converge, still stale: %v", stale)
	}
}

// container_name and the network name were fixed to "yamsplus", so a second
// install with its own projectName collided with the first on every container.
func TestRenderNamesContainersAndNetworkAfterTheProject(t *testing.T) {
	cfg := testConfig()
	cfg.ProjectName = "media2"
	files, err := Render(cfg, layout.New(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	compose := string(files[0].Content)
	if strings.Contains(compose, "container_name: yamsplus-") || strings.Contains(compose, "name: yamsplus\n") {
		t.Fatalf("a fixed yamsplus name survived in compose.yaml:\n%s", compose)
	}
	for _, want := range []string{"name: media2\n", "container_name: media2-jellyfin\n", "container_name: media2-recyclarr\n"} {
		if !strings.Contains(compose, want) {
			t.Fatalf("compose.yaml lacks %q", want)
		}
	}
}
