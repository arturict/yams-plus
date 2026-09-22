package apps

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/arturict/yams-plus/internal/config"
)

func TestSeerrSetsAutomaticApprovalDefaults(t *testing.T) {
	initialized := false
	postedPermissions := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/status":
			json.NewEncoder(w).Encode(map[string]any{"version": "3.4.1"})
		case r.URL.Path == "/api/v1/settings/public" || r.URL.Path == "/api/v1/settings/initialize":
			if r.URL.Path == "/api/v1/settings/initialize" {
				initialized = true
			}
			json.NewEncoder(w).Encode(map[string]bool{"initialized": initialized})
		case r.URL.Path == "/api/v1/auth/jellyfin":
			json.NewEncoder(w).Encode(map[string]any{"id": 1})
		case r.URL.Path == "/api/v1/settings/main" && r.Method == http.MethodGet:
			json.NewEncoder(w).Encode(map[string]any{"apiKey": "seerr-key"})
		case r.URL.Path == "/api/v1/settings/main" && r.Method == http.MethodPost:
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			if _, exists := body["apiKey"]; exists {
				http.Error(w, "apiKey is read-only", http.StatusBadRequest)
				return
			}
			postedPermissions = int(body["defaultPermissions"].(float64))
			json.NewEncoder(w).Encode(body)
		case r.URL.Path == "/api/v1/settings/network" && r.Method == http.MethodGet:
			json.NewEncoder(w).Encode(map[string]any{})
		case r.URL.Path == "/api/v1/settings/network" && r.Method == http.MethodPost:
			json.NewEncoder(w).Encode(map[string]any{})
		case r.URL.Path == "/api/v1/settings/jellyfin/library":
			json.NewEncoder(w).Encode([]any{})
		default:
			http.Error(w, r.URL.Path, http.StatusNotFound)
		}
	}))
	defer server.Close()
	cfg := config.Default()
	cfg.AdminUsername = "captain"
	cfg.Downloads.Usenet.Host = "news.example.test"
	key, restart, err := (Seerr{API: NewHTTPClient(server.URL)}).Bootstrap(context.Background(), cfg, "this-is-a-long-password", "127.0.0.1", "")
	if err != nil {
		t.Fatal(err)
	}
	if restart {
		t.Fatal("no restart is needed when CSRF protection was never on")
	}
	if key != "seerr-key" || postedPermissions != seerrDefaultPermissions {
		t.Fatalf("key=%s permissions=%d", key, postedPermissions)
	}
}

// Earlier releases turned on Seerr's csrfProtection, which Seerr describes as
// "set external API access to read-only (requires HTTPS)". With it on, every
// POST, API-key requests included, needs the token from two Secure cookies
// that Go's cookie jar never sends over plain HTTP, so every apply after the
// first failed with 403. Bootstrap must carry the token, turn the setting off
// and ask for the restart that makes that take effect.
func TestSeerrBootstrapTurnsOffCSRFProtection(t *testing.T) {
	var postedNetwork map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Like Seerr's csurf middleware: the secret cookie is issued only to a
		// request that does not carry one yet, the token on every response.
		if _, err := r.Cookie("_csrf"); err != nil {
			http.SetCookie(w, &http.Cookie{Name: "_csrf", Value: "secret", Path: "/", Secure: true, HttpOnly: true})
		}
		http.SetCookie(w, &http.Cookie{Name: "XSRF-TOKEN", Value: "token", Path: "/", Secure: true})
		if r.Method == http.MethodPost {
			cookie, err := r.Cookie("_csrf")
			if err != nil || cookie.Value != "secret" || r.Header.Get("X-XSRF-TOKEN") != "token" {
				http.Error(w, `{"message":"invalid csrf token"}`, http.StatusForbidden)
				return
			}
		}
		switch {
		case r.URL.Path == "/api/v1/status":
			json.NewEncoder(w).Encode(map[string]any{"version": "3.4.1"})
		case r.URL.Path == "/api/v1/settings/public":
			json.NewEncoder(w).Encode(map[string]bool{"initialized": true})
		case r.URL.Path == "/api/v1/settings/main" && r.Method == http.MethodGet:
			json.NewEncoder(w).Encode(map[string]any{"apiKey": "seerr-key"})
		case r.URL.Path == "/api/v1/settings/main":
			json.NewEncoder(w).Encode(map[string]any{})
		case r.URL.Path == "/api/v1/settings/network" && r.Method == http.MethodGet:
			json.NewEncoder(w).Encode(map[string]any{"csrfProtection": true, "trustProxy": false})
		case r.URL.Path == "/api/v1/settings/network":
			json.NewDecoder(r.Body).Decode(&postedNetwork)
			json.NewEncoder(w).Encode(postedNetwork)
		case r.URL.Path == "/api/v1/settings/jellyfin/library":
			json.NewEncoder(w).Encode([]any{})
		default:
			http.Error(w, r.URL.Path, http.StatusNotFound)
		}
	}))
	defer server.Close()
	cfg := config.Default()
	cfg.AdminUsername = "captain"
	key, restart, err := (Seerr{API: NewHTTPClient(server.URL)}).Bootstrap(context.Background(), cfg, "", "127.0.0.1", "seerr-key")
	if err != nil {
		t.Fatal(err)
	}
	if key != "seerr-key" {
		t.Fatalf("key = %q", key)
	}
	if postedNetwork["csrfProtection"] != false {
		t.Fatalf("csrfProtection was not turned off: %v", postedNetwork)
	}
	if !restart {
		t.Fatal("Seerr reads csrfProtection at startup, so turning it off needs a restart")
	}
}
