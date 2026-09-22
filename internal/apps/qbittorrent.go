package apps

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// QBittorrent configures qBittorrent only through its documented WebUI API.
// The login password is supplied by the installer (existing installs) or read
// from the container's one-time startup log by the convergence orchestrator.
type QBittorrent struct {
	API *HTTPClient
}

var temporaryPasswordPattern = regexp.MustCompile(`(?i)temporary password[^:\r\n]*:\s*([^\s]+)`)

func TemporaryQBittorrentPassword(logs string) (string, error) {
	match := temporaryPasswordPattern.FindStringSubmatch(logs)
	if len(match) != 2 || strings.TrimSpace(match[1]) == "" {
		return "", errors.New("qBittorrent temporary WebUI password was not found in container logs")
	}
	return strings.TrimSpace(match[1]), nil
}

func (q QBittorrent) Wait(ctx context.Context) error {
	return q.API.Wait(ctx, "/api/v2/app/version", 5*time.Minute)
}

// qbittorrentWebUIPort is the port the WebUI listens on inside its container
// (WEBUI_PORT in compose.yaml.tmpl). The port published on the host is
// configurable and may differ.
const qbittorrentWebUIPort = 8081

// NewQBittorrent returns a client for the WebUI published at baseURL. Until the
// first Converge turns host-header validation off, qBittorrent refuses every
// request whose Host port is not its own WebUI port, so a published port other
// than 8081 made the first login fail with 401. The Host header therefore
// always names the container port.
func NewQBittorrent(baseURL string) QBittorrent {
	api := NewHTTPClient(baseURL)
	if parsed, err := url.Parse(api.BaseURL); err == nil {
		api.Host = net.JoinHostPort(parsed.Hostname(), strconv.Itoa(qbittorrentWebUIPort))
	}
	return QBittorrent{API: api}
}

func (q QBittorrent) Login(ctx context.Context, username, password string) error {
	response, err := q.API.DoFormText(ctx, http.MethodPost, "/api/v2/auth/login", url.Values{
		"username": {username}, "password": {password},
	})
	if err != nil {
		return err
	}
	// qBittorrent 5.2 answers success with 204 and no body and a wrong
	// password with 401; earlier releases answer 200 with "Ok." or "Fails.".
	switch strings.ToLower(strings.TrimSpace(response)) {
	case "", "ok.":
		return nil
	}
	return fmt.Errorf("qBittorrent rejected WebUI credentials")
}

func (q QBittorrent) Converge(ctx context.Context, username, password string) error {
	prefs := map[string]any{
		"save_path":                              "/data/downloads/torrents/complete",
		"temp_path":                              "/data/downloads/torrents/incomplete",
		"temp_path_enabled":                      true,
		"create_subfolder_enabled":               true,
		"web_ui_csrf_protection_enabled":         true,
		"web_ui_clickjacking_protection_enabled": true,
		"web_ui_host_header_validation_enabled":  false,
		"web_ui_upnp":                            false,
		"upnp":                                   false,
		"dht":                                    true,
		"pex":                                    true,
		"lsd":                                    false,
	}
	// Only assert the WebUI credentials when the password is actually known.
	// A credential-free re-apply used to send an empty password, which cleared
	// the qBittorrent WebUI password entirely.
	if password != "" {
		prefs["web_ui_username"] = username
		prefs["web_ui_password"] = password
	}
	raw, err := json.Marshal(prefs)
	if err != nil {
		return err
	}
	if err := q.API.DoForm(ctx, http.MethodPost, "/api/v2/app/setPreferences", url.Values{"json": {string(raw)}}, nil); err != nil {
		return err
	}
	categories := map[string]string{
		"radarr":     "/data/downloads/torrents/complete/movies",
		"sonarr":     "/data/downloads/torrents/complete/series",
		"books":      "/data/downloads/torrents/complete/books",
		"audiobooks": "/data/downloads/torrents/complete/audiobooks",
	}
	var existing map[string]struct {
		Name     string `json:"name"`
		SavePath string `json:"savePath"`
	}
	if err := q.API.DoJSON(ctx, http.MethodGet, "/api/v2/torrents/categories", nil, &existing); err != nil {
		return err
	}
	for name, path := range categories {
		endpoint := "/api/v2/torrents/createCategory"
		if _, ok := existing[name]; ok {
			endpoint = "/api/v2/torrents/editCategory"
		}
		if err := q.API.DoForm(ctx, http.MethodPost, endpoint, url.Values{"category": {name}, "savePath": {path}}, nil); err != nil {
			return err
		}
	}
	return nil
}
