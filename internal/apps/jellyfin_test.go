package apps

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/arturict/yams-plus/internal/config"
)

func TestJellyfinBootstrapAndLibraries(t *testing.T) {
	initialized := false
	firstUserInitialized := false
	libraries := []map[string]string{}
	installed := []map[string]string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/System/Info/Public":
			json.NewEncoder(w).Encode(map[string]bool{"StartupWizardCompleted": initialized})
		case r.URL.Path == "/Startup/User" && r.Method == http.MethodGet:
			firstUserInitialized = true
			json.NewEncoder(w).Encode(map[string]string{"Name": ""})
		case r.URL.Path == "/Startup/User" && r.Method == http.MethodPost:
			if !firstUserInitialized {
				http.Error(w, "first user was not initialized", http.StatusNotFound)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		case strings.HasPrefix(r.URL.Path, "/Startup/"):
			if r.URL.Path == "/Startup/Complete" {
				initialized = true
			}
			w.WriteHeader(http.StatusNoContent)
		case r.URL.Path == "/Users/AuthenticateByName":
			json.NewEncoder(w).Encode(map[string]string{"AccessToken": "token"})
		case r.URL.Path == "/Library/VirtualFolders" && r.Method == http.MethodGet:
			json.NewEncoder(w).Encode(libraries)
		case r.URL.Path == "/Library/VirtualFolders" && r.Method == http.MethodPost:
			libraries = append(libraries, map[string]string{"Name": r.URL.Query().Get("name")})
			w.WriteHeader(http.StatusNoContent)
		case r.URL.Path == "/Repositories" && r.Method == http.MethodGet:
			json.NewEncoder(w).Encode([]any{})
		case r.URL.Path == "/Repositories" && r.Method == http.MethodPost:
			w.WriteHeader(http.StatusNoContent)
		case r.URL.Path == "/Plugins":
			json.NewEncoder(w).Encode(installed)
		case r.URL.Path == "/Packages":
			json.NewEncoder(w).Encode([]map[string]string{
				{"name": "AudioDB"}, {"name": "Custom Tabs"}, {"name": "Editor's Choice"},
				{"name": "File Transformation"}, {"name": "Intro Skipper"}, {"name": "Jellyfin Enhanced"},
				{"name": "MusicBrainz"}, {"name": "OMDb"}, {"name": "Playback Reporting"},
				{"name": "Plugin Pages"}, {"name": "Reports"}, {"name": "Studio Images"}, {"name": "TMDb"},
			})
		case strings.HasPrefix(r.URL.Path, "/Packages/Installed/"):
			name, _ := urlPathUnescape(strings.TrimPrefix(r.URL.Path, "/Packages/Installed/"))
			installed = append(installed, map[string]string{"Name": name})
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, r.URL.Path, http.StatusNotFound)
		}
	}))
	defer server.Close()
	cfg := config.Default()
	cfg.AdminUsername = "captain"
	cfg.Downloads.Usenet.Host = "news.example.test"
	result, err := (Jellyfin{API: NewHTTPClient(server.URL)}).Converge(context.Background(), cfg, "this-is-a-long-password", "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Token != "token" || len(libraries) != 2 {
		t.Fatalf("result=%#v libraries=%#v", result, libraries)
	}
	if len(result.InstalledPackages) == 0 {
		t.Fatal("expected plugin installs")
	}
}

func urlPathUnescape(value string) (string, error) { return value, nil }

func TestDoStartupJSONRetriesTransientConnectionFailure(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			conn, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				t.Fatal(err)
			}
			_ = conn.Close()
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	err := (Jellyfin{API: NewHTTPClient(server.URL)}).doStartupJSON(context.Background(), http.MethodPost, "/Startup/Configuration", map[string]string{"UICulture": "en-US"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if attempts != 2 {
		t.Fatalf("expected two attempts, got %d", attempts)
	}
}

func TestConfigurePlaybackRetention(t *testing.T) {
	months := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/System/Configuration/playback_reporting" {
			http.NotFound(w, r)
			return
		}
		if r.Method == http.MethodPost {
			var body struct {
				MaxDataAge int `json:"MaxDataAge"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			months = body.MaxDataAge
			w.WriteHeader(http.StatusNoContent)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"MaxDataAge": months, "BackupPath": "", "MaxBackupFiles": 5})
	}))
	defer server.Close()

	if err := (Jellyfin{API: NewHTTPClient(server.URL)}).ConfigurePlaybackRetention(context.Background(), 90); err != nil {
		t.Fatal(err)
	}
	if months != 3 {
		t.Fatalf("expected three months, got %d", months)
	}
	if err := (Jellyfin{API: NewHTTPClient(server.URL)}).ConfigurePlaybackRetention(context.Background(), 91); err == nil {
		t.Fatal("expected an error for retention that cannot be represented by the plugin")
	}
}
