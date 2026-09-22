package apps

import (
	"context"
	"encoding/json"
	"net"
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

// A credential-free re-apply must not clear the WebUI password. Convergence
// used to send web_ui_password:"" whenever apply had no admin password, which
// left qBittorrent open while doctor stayed green.
func TestQBittorrentConvergeOmitsCredentialsWhenPasswordUnknown(t *testing.T) {
	for _, tc := range []struct {
		name     string
		password string
		wantKeys bool
	}{
		{"unknown password leaves credentials alone", "", false},
		{"known password asserts credentials", "s3cret", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var prefs map[string]any
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/v2/app/setPreferences":
					_ = r.ParseForm()
					if err := json.Unmarshal([]byte(r.Form.Get("json")), &prefs); err != nil {
						t.Fatal(err)
					}
				case "/api/v2/torrents/categories":
					_, _ = w.Write([]byte(`{}`))
				}
			}))
			defer server.Close()

			api := NewHTTPClient(server.URL)
			if err := (QBittorrent{API: api}).Converge(context.Background(), "captain", tc.password); err != nil {
				t.Fatal(err)
			}
			_, hasUser := prefs["web_ui_username"]
			_, hasPass := prefs["web_ui_password"]
			if hasUser != tc.wantKeys || hasPass != tc.wantKeys {
				t.Fatalf("web_ui_username=%v web_ui_password=%v, want both %v", hasUser, hasPass, tc.wantKeys)
			}
			if tc.wantKeys && prefs["web_ui_password"] != tc.password {
				t.Fatalf("password = %v, want %q", prefs["web_ui_password"], tc.password)
			}
			// The non-credential preferences are always asserted.
			if prefs["web_ui_csrf_protection_enabled"] != true {
				t.Fatalf("csrf protection was not set: %v", prefs)
			}
		})
	}
}

// qBittorrent 5.2 answers a successful login with 204 and no body, and while
// host-header validation is still on (it is until the first Converge) it
// refuses any request whose Host port differs from its own WebUI port. The
// published port is configurable, so the login must name the container port.
func TestQBittorrentLoginAgainstARemappedPort(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, port, _ := net.SplitHostPort(r.Host); port != "8081" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("Unauthorized"))
			return
		}
		_ = r.ParseForm()
		if r.Form.Get("username") != "admin" || r.Form.Get("password") != "temporary" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("Unauthorized"))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	qbit := NewQBittorrent(server.URL)
	if err := qbit.Login(context.Background(), "admin", "temporary"); err != nil {
		t.Fatalf("login through a remapped port must succeed: %v", err)
	}
	if err := qbit.Login(context.Background(), "admin", "wrong"); err == nil {
		t.Fatal("a wrong password must be rejected")
	}
}

// Releases before 5.2 answer 200 with "Ok." or "Fails.".
func TestQBittorrentLoginReadsTheOlderTextAnswers(t *testing.T) {
	for body, wantErr := range map[string]bool{"Ok.": false, "Fails.": true} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(body))
		}))
		err := NewQBittorrent(server.URL).Login(context.Background(), "admin", "secret")
		server.Close()
		if (err != nil) != wantErr {
			t.Fatalf("body %q: err = %v, want error %v", body, err, wantErr)
		}
	}
}
