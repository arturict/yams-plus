package apps

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/arturict/yams-plus/internal/config"
)

type SABnzbd struct {
	API    *HTTPClient
	APIKey string
}

func DiscoverINIValue(configDir, key string, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	path := filepath.Join(configDir, "sabnzbd.ini")
	for time.Now().Before(deadline) {
		file, err := os.Open(path)
		if err == nil {
			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				parts := strings.SplitN(scanner.Text(), "=", 2)
				if len(parts) == 2 && strings.EqualFold(strings.TrimSpace(parts[0]), key) {
					_ = file.Close()
					return strings.TrimSpace(parts[1]), nil
				}
			}
			_ = file.Close()
		}
		time.Sleep(time.Second)
	}
	return "", fmt.Errorf("%s did not appear in %s", key, path)
}

func (s SABnzbd) Wait(ctx context.Context) error {
	return s.API.Wait(ctx, "/api?mode=version&output=json", 5*time.Minute)
}

func (s SABnzbd) Converge(ctx context.Context, cfg config.Config, adminPassword, providerUsername, providerPassword string) error {
	base := url.Values{"mode": {"set_config"}, "output": {"json"}, "apikey": {s.APIKey}}
	setMisc := func(keyword, value string) error {
		values := cloneValues(base)
		values.Set("section", "misc")
		values.Set("keyword", keyword)
		values.Set("value", value)
		return s.API.DoForm(ctx, http.MethodPost, "/api", values, nil)
	}
	misc := map[string]string{"complete_dir": "/data/downloads/usenet/complete", "download_dir": "/data/downloads/usenet/incomplete", "inet_exposure": "0", "permissions": "770"}
	if adminPassword != "" {
		misc["username"], misc["password"] = cfg.AdminUsername, adminPassword
	}
	for keyword, value := range misc {
		if err := setMisc(keyword, value); err != nil {
			return err
		}
	}
	server := cloneValues(base)
	server.Set("section", "servers")
	server.Set("name", "YAMS Plus "+cfg.Downloads.Usenet.Provider)
	server.Set("host", cfg.Downloads.Usenet.Host)
	server.Set("port", fmt.Sprint(cfg.Downloads.Usenet.Port))
	server.Set("username", providerUsername)
	server.Set("password", providerPassword)
	server.Set("connections", fmt.Sprint(cfg.Downloads.Usenet.Connections))
	server.Set("ssl", "1")
	server.Set("ssl_verify", "2")
	server.Set("enable", "1")
	server.Set("priority", "0")
	if err := s.API.DoForm(ctx, http.MethodPost, "/api", server, nil); err != nil {
		return err
	}
	categories := map[string]string{"radarr": "movies", "sonarr": "series"}
	if cfg.Modules.Books {
		categories["books"] = "books"
		categories["audiobooks"] = "audiobooks"
	}
	for name, dir := range categories {
		values := cloneValues(base)
		values.Set("section", "categories")
		values.Set("name", name)
		values.Set("dir", dir)
		values.Set("pp", "3")
		if err := s.API.DoForm(ctx, http.MethodPost, "/api", values, nil); err != nil {
			return err
		}
	}
	return nil
}

func cloneValues(input url.Values) url.Values {
	out := make(url.Values, len(input))
	for key, values := range input {
		out[key] = append([]string(nil), values...)
	}
	return out
}
