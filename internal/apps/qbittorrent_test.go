package apps

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTemporaryQBittorrentPassword(t *testing.T) {
	got, err := TemporaryQBittorrentPassword("The WebUI administrator username is: admin\nA temporary password is provided for this session: F9_hello")
	if err != nil || got != "F9_hello" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestQBittorrentConvergeIsUpsertSafe(t *testing.T) {
	var created, edited int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/auth/login":
			_, _ = w.Write([]byte("Ok."))
		case "/api/v2/app/setPreferences":
			_ = r.ParseForm()
			if !strings.Contains(r.Form.Get("json"), "/data/downloads/torrents/complete") {
				t.Error("missing download path")
			}
		case "/api/v2/torrents/categories":
			_ = json.NewEncoder(w).Encode(map[string]any{"radarr": map[string]any{"name": "radarr", "savePath": "/old"}})
		case "/api/v2/torrents/createCategory":
			created++
		case "/api/v2/torrents/editCategory":
			edited++
		default:
			t.Fatalf("unexpected %s", r.URL.Path)
		}
	}))
	defer server.Close()
	q := QBittorrent{API: NewHTTPClient(server.URL)}
	if err := q.Login(context.Background(), "admin", "password"); err != nil {
		t.Fatal(err)
	}
	if err := q.Converge(context.Background(), "owner", "new-password"); err != nil {
		t.Fatal(err)
	}
	if created != 3 || edited != 1 {
		t.Fatalf("created=%d edited=%d", created, edited)
	}
}
