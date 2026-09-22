package apps

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/arturict/yams-plus/internal/config"
)

type Seerr struct{ API *HTTPClient }

const seerrDefaultPermissions = 32 | 128 | 1024 | 8192 | 16384 | 32768 | 262144 | 524288 | 2048 | 4096 | 65536 | 131072 | 2097152 | 4194304

// Bootstrap initializes or signs in to Seerr, applies the main and network
// settings and returns its API key. The boolean reports that Seerr must be
// restarted, because it reads csrfProtection only at startup.
func (s Seerr) Bootstrap(ctx context.Context, cfg config.Config, password, externalHost, existingAPIKey string) (string, bool, error) {
	if err := s.API.Wait(ctx, "/api/v1/status", 5*time.Minute); err != nil {
		return "", false, err
	}
	csrfActive, err := s.useCSRFToken(ctx)
	if err != nil {
		return "", false, err
	}
	var public struct {
		Initialized bool `json:"initialized"`
	}
	if err := s.API.DoJSON(ctx, http.MethodGet, "/api/v1/settings/public", nil, &public); err != nil {
		return "", false, err
	}
	if !public.Initialized {
		if password == "" {
			return "", false, fmt.Errorf("Seerr is uninitialized and the admin password is unavailable")
		}
		login := map[string]any{"username": cfg.AdminUsername, "password": password, "hostname": "jellyfin", "port": 8096, "useSsl": false, "urlBase": "", "email": "", "serverType": 2}
		if err := s.API.DoJSON(ctx, http.MethodPost, "/api/v1/auth/jellyfin", login, nil); err != nil {
			return "", false, err
		}
		if err := s.API.DoJSON(ctx, http.MethodPost, "/api/v1/settings/initialize", nil, &public); err != nil {
			return "", false, err
		}
	} else if existingAPIKey != "" {
		s.API.Headers.Set("X-Api-Key", existingAPIKey)
	} else if password != "" {
		// hostname configures the media server during first-run only. Sending it
		// again makes Seerr 3.4 reject an otherwise valid Jellyfin sign-in.
		login := map[string]any{"username": cfg.AdminUsername, "password": password}
		if err := s.API.DoJSON(ctx, http.MethodPost, "/api/v1/auth/jellyfin", login, nil); err != nil {
			return "", false, err
		}
	}
	var main map[string]any
	if err := s.API.DoJSON(ctx, http.MethodGet, "/api/v1/settings/main", nil, &main); err != nil {
		return "", false, err
	}
	apiKey := fmt.Sprint(main["apiKey"])
	delete(main, "apiKey") // read-only in Seerr's versioned OpenAPI contract
	main["appLanguage"], main["applicationTitle"], main["applicationUrl"] = "en", "Seerr", fmt.Sprintf("http://%s:%d", externalHost, cfg.Ports.Seerr)
	main["defaultPermissions"], main["localLogin"], main["mediaServerType"] = seerrDefaultPermissions, true, 2
	main["partialRequestsEnabled"], main["versionCheck"] = true, true
	if err := s.API.DoJSON(ctx, http.MethodPost, "/api/v1/settings/main", main, &main); err != nil {
		return "", false, err
	}
	// csrfProtection stays off. Seerr describes it as "set external API access
	// to read-only (requires HTTPS)": it demands a token on every change, API-key
	// requests included, and issues that token in Secure cookies that a browser
	// on a plain-HTTP LAN or Tailscale address never sends back. Earlier
	// releases turned it on, which broke every later apply.
	var network map[string]any
	if err := s.API.DoJSON(ctx, http.MethodGet, "/api/v1/settings/network", nil, &network); err != nil {
		return "", false, err
	}
	network["csrfProtection"] = false
	network["trustProxy"] = false
	if err := s.API.DoJSON(ctx, http.MethodPost, "/api/v1/settings/network", network, &network); err != nil {
		return "", false, err
	}
	if existingAPIKey != "" && (apiKey == "" || apiKey == "<nil>") {
		apiKey = existingAPIKey
	}
	if apiKey == "" || apiKey == "<nil>" {
		return "", false, fmt.Errorf("Seerr returned an empty API key")
	}
	return apiKey, csrfActive, s.enableLibraries(ctx)
}

// useCSRFToken makes every later request carry Seerr's CSRF token when
// csrfProtection is active, and reports whether it is. Seerr issues the token
// on every response but the secret cookie only to a request that lacks one, so
// after an earlier request the secret may already be in the cookie jar rather
// than in this response. Both are Secure cookies, which the jar does not send
// to every plain-HTTP address, so they are attached by hand.
func (s Seerr) useCSRFToken(ctx context.Context) (bool, error) {
	cookies, err := s.API.ResponseCookies(ctx, "/api/v1/status")
	if err != nil {
		return false, err
	}
	if base, err := url.Parse(s.API.BaseURL); err == nil {
		cookies = append(cookies, s.API.Client.Jar.Cookies(base)...)
	}
	var secret, token string
	for _, cookie := range cookies {
		switch {
		case cookie.Name == "_csrf" && secret == "":
			secret = cookie.Value
		case cookie.Name == "XSRF-TOKEN" && token == "":
			token = cookie.Value
		}
	}
	if secret == "" || token == "" {
		return false, nil
	}
	s.API.Headers.Set("Cookie", (&http.Cookie{Name: "_csrf", Value: secret}).String())
	s.API.Headers.Set("X-XSRF-TOKEN", token)
	return true, nil
}

func (s Seerr) enableLibraries(ctx context.Context) error {
	var libraries []map[string]any
	if err := s.API.DoJSON(ctx, http.MethodGet, "/api/v1/settings/jellyfin/library?sync=true", nil, &libraries); err != nil {
		return err
	}
	var ids []string
	for _, library := range libraries {
		id := fmt.Sprint(library["id"])
		if id != "" && id != "<nil>" {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	query := url.Values{"enable": {strings.Join(ids, ",")}}
	return s.API.DoJSON(ctx, http.MethodGet, "/api/v1/settings/jellyfin/library?"+query.Encode(), nil, &libraries)
}

func (s Seerr) EnsureServarr(ctx context.Context, cfg config.Config, kind string, profiles, roots []map[string]any, apiKey string) error {
	port, directory := 7878, "/data/library/movies"
	path := "/api/v1/settings/radarr"
	enabled := cfg.Modules.Movies
	wantedProfiles := cfg.Quality.Movies.Profiles
	if kind == "sonarr" {
		port, directory, path, enabled, wantedProfiles = 8989, "/data/library/series", "/api/v1/settings/sonarr", cfg.Modules.Series, cfg.Quality.Series.Profiles
	}
	if !enabled {
		return nil
	}
	profile := findProfile(profiles, "YAMS+ "+cfg.Quality.Movies.DefaultProfile)
	if kind == "sonarr" {
		profile = findProfile(profiles, "YAMS+ "+cfg.Quality.Series.DefaultProfile)
	}
	if profile == nil {
		return fmt.Errorf("%s default quality profile is missing after Recyclarr sync", kind)
	}
	root := directory
	for _, item := range roots {
		if strings.EqualFold(fmt.Sprint(item["path"]), directory) {
			root = fmt.Sprint(item["path"])
			break
		}
	}
	serviceName := "YAMS+ Radarr"
	if kind == "sonarr" {
		serviceName = "YAMS+ Sonarr"
	}
	body := map[string]any{"name": serviceName, "hostname": kind, "port": port, "apiKey": apiKey, "useSsl": false, "baseUrl": "", "activeProfileId": profile["id"], "activeProfileName": profile["name"], "activeDirectory": root, "is4k": false, "isDefault": true, "externalUrl": ""}
	if kind == "radarr" {
		body["minimumAvailability"] = "released"
	} else {
		body["enableSeasonFolders"] = true
		body["activeLanguageProfileId"] = 1
	}
	var current []map[string]any
	if err := s.API.DoJSON(ctx, http.MethodGet, path, nil, &current); err != nil {
		return err
	}
	upsert := func(candidate map[string]any) error {
		for _, item := range current {
			if fmt.Sprint(item["name"]) == fmt.Sprint(candidate["name"]) {
				body := cloneMap(candidate)
				delete(body, "id") // path identifies the service; body id is read-only
				return s.API.DoJSON(ctx, http.MethodPut, path+"/"+fmt.Sprint(item["id"]), body, nil)
			}
		}
		return s.API.DoJSON(ctx, http.MethodPost, path, candidate, nil)
	}
	if err := upsert(body); err != nil {
		return err
	}
	// A second logical service points at the same Arr instance. This preserves
	// one physical copy while exposing Seerr's explicit 4K request path.
	if slices.Contains(wantedProfiles, "2160p") {
		fourK := findProfile(profiles, "YAMS+ 2160p")
		if fourK == nil {
			return fmt.Errorf("%s 4K profile is missing after Recyclarr sync", kind)
		}
		fourKBody := cloneMap(body)
		fourKBody["name"], fourKBody["activeProfileId"], fourKBody["activeProfileName"], fourKBody["is4k"], fourKBody["isDefault"] = serviceName+" 4K", fourK["id"], fourK["name"], true, false
		delete(fourKBody, "id")
		return upsert(fourKBody)
	}
	return nil
}

func findProfile(profiles []map[string]any, name string) map[string]any {
	for _, profile := range profiles {
		if strings.EqualFold(fmt.Sprint(profile["name"]), name) {
			return profile
		}
	}
	return nil
}
