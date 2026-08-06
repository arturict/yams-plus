package apps

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/arturict/yams-plus/internal/config"
	"gopkg.in/yaml.v3"
)

type Bazarr struct {
	API    *HTTPClient
	APIKey string
}

func DiscoverBazarrAPIKey(configDir string, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	path := filepath.Join(configDir, "config", "config.yaml")
	for time.Now().Before(deadline) {
		raw, err := os.ReadFile(path)
		if err == nil {
			var document struct {
				Auth struct {
					APIKey string `yaml:"apikey"`
				} `yaml:"auth"`
			}
			if yaml.Unmarshal(raw, &document) == nil && strings.TrimSpace(document.Auth.APIKey) != "" {
				return strings.TrimSpace(document.Auth.APIKey), nil
			}
		}
		time.Sleep(time.Second)
	}
	return "", fmt.Errorf("Bazarr API key did not appear in %s", path)
}

func (b Bazarr) Converge(ctx context.Context, cfg config.Config, password, sonarrKey, radarrKey, providerUsername, providerPassword string) error {
	b.API.Headers.Set("X-API-KEY", b.APIKey)
	if err := b.API.Wait(ctx, "/api/system/status", 5*time.Minute); err != nil {
		return err
	}
	values := url.Values{
		"settings-auth-type":                     {"form"},
		"settings-auth-username":                 {cfg.AdminUsername},
		"settings-auth-password":                 {password},
		"settings-general-use_sonarr":            {boolString(cfg.Modules.Series)},
		"settings-general-use_radarr":            {boolString(cfg.Modules.Movies)},
		"settings-general-serie_default_enabled": {boolString(cfg.Modules.Series)},
		"settings-general-movie_default_enabled": {boolString(cfg.Modules.Movies)},
		"settings-general-serie_default_profile": {"1"},
		"settings-general-movie_default_profile": {"1"},
		"settings-general-enabled_providers":     {cfg.Subtitles.Provider},
		"settings-general-chmod":                 {"0660"},
	}
	if cfg.Modules.Series {
		values.Set("settings-sonarr-ip", "sonarr")
		values.Set("settings-sonarr-port", "8989")
		values.Set("settings-sonarr-base_url", "/")
		values.Set("settings-sonarr-ssl", "false")
		values.Set("settings-sonarr-apikey", sonarrKey)
	}
	if cfg.Modules.Movies {
		values.Set("settings-radarr-ip", "radarr")
		values.Set("settings-radarr-port", "7878")
		values.Set("settings-radarr-base_url", "/")
		values.Set("settings-radarr-ssl", "false")
		values.Set("settings-radarr-apikey", radarrKey)
	}
	if cfg.Subtitles.Provider == "opensubtitles.com" || cfg.Subtitles.Provider == "opensubtitlescom" {
		values["settings-general-enabled_providers"] = []string{"opensubtitlescom"}
		values.Set("settings-opensubtitlescom-username", providerUsername)
		values.Set("settings-opensubtitlescom-password", providerPassword)
	}
	values["languages-enabled"] = append([]string(nil), cfg.Subtitles.Languages...)
	profileRaw, err := json.Marshal(bazarrProfile(cfg))
	if err != nil {
		return err
	}
	values.Set("languages-profiles", string(profileRaw))
	if err := b.API.DoForm(ctx, http.MethodPost, "/api/system/settings", values, nil, http.StatusNoContent); err != nil {
		return err
	}
	return b.Verify(ctx, cfg)
}

func (b Bazarr) Verify(ctx context.Context, cfg config.Config) error {
	var settings map[string]any
	if err := b.API.DoJSON(ctx, http.MethodGet, "/api/system/settings", nil, &settings); err != nil {
		return err
	}
	general, _ := settings["general"].(map[string]any)
	if cfg.Modules.Series && !truthy(general["use_sonarr"]) {
		return fmt.Errorf("Bazarr did not retain the Sonarr connection")
	}
	if cfg.Modules.Movies && !truthy(general["use_radarr"]) {
		return fmt.Errorf("Bazarr did not retain the Radarr connection")
	}
	var profiles []map[string]any
	if err := b.API.DoJSON(ctx, http.MethodGet, "/api/system/languages/profiles", nil, &profiles); err != nil {
		return err
	}
	for _, profile := range profiles {
		if fmt.Sprint(profile["profileId"]) == "1" && fmt.Sprint(profile["name"]) == "YAMS+ subtitles" {
			return nil
		}
	}
	return fmt.Errorf("Bazarr language profile was not created")
}

func bazarrProfile(cfg config.Config) []map[string]any {
	items := make([]map[string]any, 0, len(cfg.Subtitles.Languages)*3)
	id := 1
	for _, language := range cfg.Subtitles.Languages {
		items = append(items, bazarrProfileItem(id, language, false, false))
		id++
		if cfg.Subtitles.Forced {
			items = append(items, bazarrProfileItem(id, language, true, false))
			id++
		}
		if cfg.Subtitles.HearingImpaired {
			items = append(items, bazarrProfileItem(id, language, false, true))
			id++
		}
	}
	return []map[string]any{{
		"profileId": 1, "name": "YAMS+ subtitles", "tag": "yamsplus",
		"items": items, "cutoff": 65535, "mustContain": []string{},
		"mustNotContain": []string{}, "originalFormat": false,
	}}
}

func bazarrProfileItem(id int, language string, forced, hi bool) map[string]any {
	return map[string]any{"id": id, "language": language, "audio_exclude": "False", "audio_only_include": "False", "forced": titleBool(forced), "hi": titleBool(hi)}
}

func titleBool(value bool) string {
	if value {
		return "True"
	}
	return "False"
}
func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}
func truthy(value any) bool {
	switch item := value.(type) {
	case bool:
		return item
	case string:
		return strings.EqualFold(item, "true") || item == "1"
	default:
		return false
	}
}
