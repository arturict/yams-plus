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
	key, err := (Seerr{API: NewHTTPClient(server.URL)}).Bootstrap(context.Background(), cfg, "this-is-a-long-password", "127.0.0.1", "")
	if err != nil {
		t.Fatal(err)
	}
	if key != "seerr-key" || postedPermissions != seerrDefaultPermissions {
		t.Fatalf("key=%s permissions=%d", key, postedPermissions)
	}
}
