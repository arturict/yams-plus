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

func TestAudiobookshelfConvergeInitializesAndCreatesLibraries(t *testing.T) {
	var initialized bool
	created := map[string]bool{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/status":
			_ = json.NewEncoder(w).Encode(map[string]any{"isInit": initialized})
		case "/init":
			initialized = true
		case "/login":
			_ = json.NewEncoder(w).Encode(map[string]any{"user": map[string]any{"accessToken": "token"}})
		case "/api/libraries":
			if r.Method == http.MethodGet {
				_ = json.NewEncoder(w).Encode(map[string]any{"libraries": []any{}})
				return
			}
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			created[body["name"].(string)] = true
		default:
			t.Fatalf("unexpected %s", r.URL.Path)
		}
	}))
	defer server.Close()
	token, err := (Audiobookshelf{API: NewHTTPClient(server.URL)}).Converge(context.Background(), "owner", "password123", "")
	if err != nil || token != "token" || !created["Audiobooks"] || !created["eBooks"] {
		t.Fatalf("token=%q created=%v err=%v", token, created, err)
	}
}

func TestShelfmarkConvergePersistsReusableSession(t *testing.T) {
	const session = "signed-automation-session"
	loginCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authenticated := false
		if cookie, err := r.Cookie("shelfmark_session"); err == nil && cookie.Value == session {
			authenticated = true
		}
		switch {
		case r.URL.Path == "/api/health":
			_ = json.NewEncoder(w).Encode(map[string]bool{"healthy": true})
		case r.URL.Path == "/api/auth/check":
			_ = json.NewEncoder(w).Encode(map[string]any{"authenticated": authenticated, "is_admin": authenticated})
		case r.URL.Path == "/api/auth/login":
			loginCount++
			http.SetCookie(w, &http.Cookie{Name: "shelfmark_session", Value: session, Path: "/"})
			_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
		case r.URL.Path == "/api/admin/users" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode([]map[string]any{{"username": "captain"}})
		case strings.HasPrefix(r.URL.Path, "/api/settings/"):
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	cfg := config.Default()
	cfg.AdminUsername = "captain"
	first, err := (Shelfmark{API: NewHTTPClient(server.URL)}).Converge(context.Background(), cfg, "long-enough-password", "", "prowlarr", "sab", "sabnzbd", "")
	if err != nil || first != session {
		t.Fatalf("session=%q err=%v", first, err)
	}
	second, err := (Shelfmark{API: NewHTTPClient(server.URL)}).Converge(context.Background(), cfg, "", first, "prowlarr", "sab", "sabnzbd", "")
	if err != nil || second != session || loginCount != 2 {
		t.Fatalf("session=%q loginCount=%d err=%v", second, loginCount, err)
	}
}

func TestShelfmarkRefreshesNoAuthSessionAfterEnablingBuiltinAuth(t *testing.T) {
	loginCount := 0
	builtin := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, _ := r.Cookie("shelfmark_session")
		session := ""
		if cookie != nil {
			session = cookie.Value
		}
		switch {
		case r.URL.Path == "/api/health":
			_ = json.NewEncoder(w).Encode(map[string]bool{"healthy": true})
		case r.URL.Path == "/api/auth/check":
			_ = json.NewEncoder(w).Encode(map[string]any{"authenticated": !builtin || session != "", "is_admin": !builtin || session == "admin-session"})
		case r.URL.Path == "/api/auth/login":
			loginCount++
			value := "no-auth-session"
			if builtin {
				value = "admin-session"
			}
			http.SetCookie(w, &http.Cookie{Name: "shelfmark_session", Value: value, Path: "/"})
			_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
		case r.URL.Path == "/api/admin/users" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode([]map[string]any{{"username": "captain", "role": "admin"}})
		case r.URL.Path == "/api/settings/security":
			builtin = true
			w.WriteHeader(http.StatusNoContent)
		case strings.HasPrefix(r.URL.Path, "/api/settings/"):
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	cfg := config.Default()
	cfg.AdminUsername = "captain"
	session, err := (Shelfmark{API: NewHTTPClient(server.URL)}).Converge(context.Background(), cfg, "long-enough-password", "", "prowlarr", "sab", "sabnzbd", "")
	if err != nil {
		t.Fatal(err)
	}
	if session != "admin-session" || loginCount != 2 {
		t.Fatalf("session=%q loginCount=%d", session, loginCount)
	}
}
