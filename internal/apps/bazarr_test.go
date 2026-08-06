package apps

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/arturict/yams-plus/internal/config"
)

func TestDiscoverBazarrAPIKey(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config", "config.yaml"), []byte("auth:\n  apikey: secret-key\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := DiscoverBazarrAPIKey(dir, time.Second)
	if err != nil || got != "secret-key" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestBazarrProfileIncludesPreferences(t *testing.T) {
	cfg := config.Default()
	cfg.Subtitles.Languages = []string{"de", "en"}
	cfg.Subtitles.Forced = true
	cfg.Subtitles.HearingImpaired = true
	profile := bazarrProfile(cfg)
	items := profile[0]["items"].([]map[string]any)
	if len(items) != 6 || items[1]["forced"] != "True" || items[2]["hi"] != "True" {
		t.Fatalf("unexpected profile: %#v", profile)
	}
}
