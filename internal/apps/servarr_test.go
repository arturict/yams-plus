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
