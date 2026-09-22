package apps

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEnsureDownloadClientUsesSchema(t *testing.T) {
	created := map[string]any{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v3/downloadclient/schema":
			json.NewEncoder(w).Encode([]any{map[string]any{"implementation": "Sabnzbd", "fields": []any{map[string]any{"name": "host"}, map[string]any{"name": "port"}, map[string]any{"name": "apiKey"}, map[string]any{"name": "movieCategory"}}}})
		case r.URL.Path == "/api/v3/downloadclient" && r.Method == http.MethodGet:
			json.NewEncoder(w).Encode([]any{})
		case r.URL.Path == "/api/v3/downloadclient" && r.Method == http.MethodPost:
			json.NewDecoder(r.Body).Decode(&created)
			w.WriteHeader(http.StatusCreated)
		default:
			http.Error(w, r.URL.Path, http.StatusNotFound)
		}
	}))
	defer server.Close()
	arr := Servarr{Name: "Radarr", API: NewHTTPClient(server.URL)}
	err := arr.EnsureDownloadClient(context.Background(), DownloadClientSpec{Name: "YAMS+ SABnzbd", Implementation: "Sabnzbd", Protocol: "usenet", Priority: 1, Fields: map[string]any{"host": "sabnzbd", "port": 8080, "apiKey": "key", "movieCategory": "radarr"}})
	if err != nil {
		t.Fatal(err)
	}
	if created["name"] != "YAMS+ SABnzbd" {
		t.Fatalf("created=%#v", created)
	}
}

// The README promises the installed stack carries no telemetry, but the arr
// applications enable anonymous usage and error reporting by default and
// nothing was turning it off. Verified against a live Radarr before fixing:
// GET /api/v3/config/host reported analyticsEnabled=true.
func TestEnsureAnalyticsDisabled(t *testing.T) {
	var put map[string]any
	var gets, puts int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			gets++
			enabled := put == nil
			_ = json.NewEncoder(w).Encode(map[string]any{
				"analyticsEnabled": enabled,
				"instanceName":     "Radarr",
			})
			return
		}
		puts++
		_ = json.NewDecoder(r.Body).Decode(&put)
		_ = json.NewEncoder(w).Encode(put)
	}))
	defer server.Close()

	client := Servarr{Name: "Radarr", API: NewHTTPClient(server.URL)}
	if err := client.EnsureAnalyticsDisabled(context.Background()); err != nil {
		t.Fatal(err)
	}
	if puts != 1 {
		t.Fatalf("puts = %d, want 1", puts)
	}
	if put["analyticsEnabled"] != false {
		t.Fatalf("analyticsEnabled = %v, want false", put["analyticsEnabled"])
	}
	// Unrelated fields must survive the round trip.
	if put["instanceName"] != "Radarr" {
		t.Fatalf("instanceName was dropped: %v", put)
	}
	// Already disabled: no second write.
	if err := client.EnsureAnalyticsDisabled(context.Background()); err != nil {
		t.Fatal(err)
	}
	if puts != 1 {
		t.Fatalf("puts = %d after a converged run, want 1", puts)
	}
}
