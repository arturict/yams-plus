package apps

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/arturict/yams-plus/internal/config"
)

type Shelfmark struct{ API *HTTPClient }

func (s Shelfmark) Converge(ctx context.Context, cfg config.Config, password, existingSession, prowlarrKey, sabKey, sabHost, qbitHost string) (string, error) {
	if err := s.API.Wait(ctx, "/api/health", 5*time.Minute); err != nil {
		return "", err
	}
	if existingSession != "" {
		if err := s.API.SetCookie("shelfmark_session", existingSession); err != nil {
			return "", err
		}
	}
	type authStatus struct {
		Authenticated bool `json:"authenticated"`
		IsAdmin       bool `json:"is_admin"`
	}
	checkAuth := func() (authStatus, error) {
		var auth authStatus
		err := s.API.DoJSON(ctx, http.MethodGet, "/api/auth/check", nil, &auth)
		return auth, err
	}
	login := func() error {
		if err := s.API.ClearCookie("shelfmark_session"); err != nil {
			return err
		}
		return s.API.DoJSON(ctx, http.MethodPost, "/api/auth/login", map[string]any{"username": cfg.AdminUsername, "password": password, "remember_me": true}, nil)
	}
	auth, err := checkAuth()
	if err != nil {
		return "", err
	}
	if existingSession == "" || !auth.Authenticated || !auth.IsAdmin {
		if password == "" {
			return "", fmt.Errorf("Shelfmark automation session is unavailable; rerun apply once with YAMSPLUS_ADMIN_PASSWORD set")
		}
		// In no-auth mode Shelfmark accepts this login and creates an admin
		// session. On reruns it authenticates the local administrator.
		if err := login(); err != nil {
			return "", err
		}
	}
	var users []map[string]any
	if err := s.API.DoJSON(ctx, http.MethodGet, "/api/admin/users", nil, &users); err != nil {
		return "", err
	}
	found := false
	for _, user := range users {
		if strings.EqualFold(fmt.Sprint(user["username"]), cfg.AdminUsername) {
			found = true
			break
		}
	}
	if !found {
		if err := s.API.DoJSON(ctx, http.MethodPost, "/api/admin/users", map[string]any{"username": cfg.AdminUsername, "password": password, "role": "admin"}, nil); err != nil {
			return "", err
		}
	}
	updates := []struct {
		tab  string
		body map[string]any
	}{
		{"downloads", map[string]any{"BOOKS_OUTPUT_MODE": "folder", "DESTINATION": "/data/library/books", "AUDIOBOOK_DESTINATION": "/data/library/audiobooks", "FILE_ORGANIZATION": "rename", "HARDLINK_TORRENTS": false}},
		{"prowlarr_config", map[string]any{"PROWLARR_ENABLED": true, "PROWLARR_URL": "http://prowlarr:9696", "PROWLARR_API_KEY": prowlarrKey, "PROWLARR_INDEXERS": []string{}, "PROWLARR_AUTO_EXPAND": true}},
	}
	clients := map[string]any{}
	if sabKey != "" {
		clients["PROWLARR_USENET_CLIENT"] = "sabnzbd"
		clients["SABNZBD_URL"] = "http://" + sabHost + ":8080"
		clients["SABNZBD_API_KEY"] = sabKey
		clients["SABNZBD_CATEGORY"] = "books"
		clients["SABNZBD_CATEGORY_AUDIOBOOK"] = "audiobooks"
		clients["PROWLARR_USENET_ACTION"] = "move"
	}
	if qbitHost != "" {
		clients["PROWLARR_TORRENT_CLIENT"] = "qbittorrent"
		clients["QBITTORRENT_URL"] = "http://" + qbitHost + ":8081"
		clients["QBITTORRENT_USERNAME"] = cfg.AdminUsername
		clients["QBITTORRENT_PASSWORD"] = password
		clients["QBITTORRENT_CATEGORY"] = "books"
		clients["QBITTORRENT_CATEGORY_AUDIOBOOK"] = "audiobooks"
		clients["PROWLARR_TORRENT_ACTION"] = "keep"
	}
	updates = append(updates, struct {
		tab  string
		body map[string]any
	}{"prowlarr_clients", clients})
	for _, update := range updates {
		if err := s.API.DoJSON(ctx, http.MethodPut, "/api/settings/"+update.tab, update.body, nil); err != nil {
			return "", fmt.Errorf("configure Shelfmark %s: %w", update.tab, err)
		}
	}
	// Authentication is enabled last so an interrupted first run remains
	// recoverable without touching Shelfmark's SQLite user database.
	if err := s.API.DoJSON(ctx, http.MethodPut, "/api/settings/security", map[string]any{"AUTH_METHOD": "builtin"}, nil); err != nil {
		return "", err
	}
	// A session created while Shelfmark is in no-auth mode does not contain an
	// administrator role. Shelfmark can briefly report the previous auth mode
	// after the settings write, so always refresh the session when the bootstrap
	// password is available instead of trusting that transitional response. A
	// new cookie jar also prevents the no-auth and built-in sessions from being
	// sent together under the same cookie name.
	if password != "" {
		s.API = NewHTTPClient(s.API.BaseURL)
		if err := login(); err != nil {
			return "", err
		}
	}
	auth, err = checkAuth()
	if err != nil {
		return "", err
	}
	if !auth.Authenticated || !auth.IsAdmin {
		if password == "" {
			return "", fmt.Errorf("Shelfmark administrator session needs a one-time refresh; rerun apply with YAMSPLUS_ADMIN_PASSWORD set")
		}
		if err := login(); err != nil {
			return "", err
		}
		auth, err = checkAuth()
		if err != nil {
			return "", err
		}
		if !auth.Authenticated || !auth.IsAdmin {
			return "", fmt.Errorf("Shelfmark did not create an administrator automation session")
		}
	}
	candidates := s.API.CookieValues("shelfmark_session")
	if existingSession != "" {
		candidates = append(candidates, existingSession)
	}
	seen := map[string]bool{}
	for _, candidate := range candidates {
		if candidate == "" || seen[candidate] {
			continue
		}
		seen[candidate] = true
		verifier := NewHTTPClient(s.API.BaseURL)
		if err := verifier.SetCookie("shelfmark_session", candidate); err != nil {
			return "", err
		}
		var verified authStatus
		if err := verifier.DoJSON(ctx, http.MethodGet, "/api/auth/check", nil, &verified); err != nil {
			continue
		}
		if verified.Authenticated && verified.IsAdmin {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("Shelfmark did not return a reusable administrator automation session")
}

type Audiobookshelf struct{ API *HTTPClient }

func (a Audiobookshelf) Converge(ctx context.Context, username, password, existingToken string) (string, error) {
	if err := a.API.Wait(ctx, "/status", 5*time.Minute); err != nil {
		return "", err
	}
	var status struct {
		IsInit bool `json:"isInit"`
	}
	if err := a.API.DoJSON(ctx, http.MethodGet, "/status", nil, &status); err != nil {
		return "", err
	}
	if !status.IsInit {
		if err := a.API.DoJSON(ctx, http.MethodPost, "/init", map[string]any{"newRoot": map[string]any{"username": username, "password": password}}, nil, http.StatusOK); err != nil {
			return "", err
		}
	}
	token := existingToken
	if token == "" {
		var login struct {
			User struct {
				AccessToken string `json:"accessToken"`
			} `json:"user"`
		}
		if err := a.API.DoJSON(ctx, http.MethodPost, "/login", map[string]any{"username": username, "password": password}, &login); err != nil {
			return "", err
		}
		token = login.User.AccessToken
	}
	if token == "" {
		return "", fmt.Errorf("Audiobookshelf did not return an access token")
	}
	a.API.Headers.Set("Authorization", "Bearer "+token)
	var libraries struct {
		Libraries []map[string]any `json:"libraries"`
	}
	if err := a.API.DoJSON(ctx, http.MethodGet, "/api/libraries", nil, &libraries); err != nil {
		return "", err
	}
	ensure := func(name, path string) error {
		for _, library := range libraries.Libraries {
			if strings.EqualFold(fmt.Sprint(library["name"]), name) {
				return nil
			}
		}
		body := map[string]any{"name": name, "mediaType": "book", "provider": "google", "folders": []map[string]any{{"fullPath": path}}}
		return a.API.DoJSON(ctx, http.MethodPost, "/api/libraries", body, nil)
	}
	if err := ensure("Audiobooks", "/audiobooks"); err != nil {
		return "", err
	}
	if err := ensure("eBooks", "/ebooks"); err != nil {
		return "", err
	}
	return token, nil
}
