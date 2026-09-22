package apps

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
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

// The same rule as qBittorrent: a re-apply without the admin password must not
// reset Bazarr's form authentication to an empty password.
func TestBazarrConvergeOmitsAuthWhenPasswordUnknown(t *testing.T) {
	for _, tc := range []struct {
		name     string
		password string
		wantAuth bool
	}{
		{"unknown password leaves auth alone", "", false},
		{"known password asserts auth", "s3cret", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var form url.Values
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.URL.Path == "/api/system/status":
					_, _ = w.Write([]byte(`{"data":{}}`))
				case r.URL.Path == "/api/system/settings" && r.Method == http.MethodPost:
					_ = r.ParseForm()
					form = r.Form
					w.WriteHeader(http.StatusNoContent)
				case r.URL.Path == "/api/system/settings":
					// Verify reads the settings back.
					_, _ = w.Write([]byte(`{"general":{"use_sonarr":true,"use_radarr":true},"analytics":{"enabled":false}}`))
				case r.URL.Path == "/api/system/languages/profiles":
					_, _ = w.Write([]byte(`[{"profileId":1,"name":"YAMS+ subtitles"}]`))
				default:
					w.WriteHeader(http.StatusNoContent)
				}
			}))
			defer server.Close()

			cfg := config.Default()
			cfg.AdminUsername = "captain"
			cfg.Modules.Series, cfg.Modules.Movies = true, true
			err := (Bazarr{API: NewHTTPClient(server.URL), APIKey: "key"}).
				Converge(context.Background(), cfg, tc.password, "sonarr-key", "radarr-key", "user", "pass")
			if err != nil {
				t.Fatal(err)
			}
			_, hasType := form["settings-auth-type"]
			_, hasPass := form["settings-auth-password"]
			if hasType != tc.wantAuth || hasPass != tc.wantAuth {
				t.Fatalf("auth-type=%v auth-password=%v, want both %v", hasType, hasPass, tc.wantAuth)
			}
			if tc.wantAuth && form.Get("settings-auth-password") != tc.password {
				t.Fatalf("password = %q, want %q", form.Get("settings-auth-password"), tc.password)
			}
			// Non-credential settings are asserted either way.
			if form.Get("settings-sonarr-apikey") != "sonarr-key" {
				t.Fatalf("sonarr key was not configured: %v", form)
			}
			if form.Get("settings-analytics-enabled") != "false" {
				t.Fatalf("Bazarr analytics were not turned off: %v", form)
			}
		})
	}
}

// Bazarr defaults analytics.enabled to true. If a settings write does not
// stick, Verify must say so instead of reporting a converged Bazarr.
func TestBazarrVerifyRejectsEnabledAnalytics(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/system/settings":
			_, _ = w.Write([]byte(`{"general":{"use_sonarr":true,"use_radarr":true},"analytics":{"enabled":true}}`))
		case "/api/system/languages/profiles":
			_, _ = w.Write([]byte(`[{"profileId":1,"name":"YAMS+ subtitles"}]`))
		}
	}))
	defer server.Close()
	cfg := config.Default()
	cfg.Modules.Series, cfg.Modules.Movies = true, true
	err := (Bazarr{API: NewHTTPClient(server.URL), APIKey: "key"}).Verify(context.Background(), cfg)
	if err == nil || !strings.Contains(err.Error(), "analytics") {
		t.Fatalf("expected Verify to report enabled analytics, got %v", err)
	}
}
