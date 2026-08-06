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
