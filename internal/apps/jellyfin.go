package apps

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/arturict/yams-plus/internal/config"
	stackassets "github.com/arturict/yams-plus/stack"
	"gopkg.in/yaml.v3"
)

type Jellyfin struct{ API *HTTPClient }

type JellyfinResult struct {
	Token             string
	InstalledPackages []string
	RestartRequired   bool
}

type pluginCatalog struct {
	Repositories []struct {
		Name string `yaml:"name"`
		URL  string `yaml:"url"`
	} `yaml:"repositories"`
}

func (j Jellyfin) Converge(ctx context.Context, cfg config.Config, password, existingToken string) (JellyfinResult, error) {
	if err := j.API.Wait(ctx, "/System/Info/Public", 5*time.Minute); err != nil {
		return JellyfinResult{}, err
	}
	var public struct {
		StartupWizardCompleted bool `json:"StartupWizardCompleted"`
	}
	if err := j.API.DoJSON(ctx, http.MethodGet, "/System/Info/Public", nil, &public); err != nil {
		return JellyfinResult{}, err
	}
	if !public.StartupWizardCompleted {
		if len(password) < 14 {
			return JellyfinResult{}, fmt.Errorf("Jellyfin is uninitialized and the admin password is unavailable")
		}
		if err := j.doStartupJSON(ctx, http.MethodPost, "/Startup/Configuration", map[string]any{"UICulture": "en-US", "MetadataCountryCode": "US", "PreferredMetadataLanguage": "en"}, nil); err != nil {
			return JellyfinResult{}, err
		}
		// Jellyfin 10.11 initializes its placeholder administrator lazily. The
		// documented GET must run before POST or a pristine server returns 404.
		var firstUser struct {
			Name string `json:"Name"`
		}
		if err := j.doStartupJSON(ctx, http.MethodGet, "/Startup/User", nil, &firstUser); err != nil {
			return JellyfinResult{}, err
		}
		if err := j.doStartupJSON(ctx, http.MethodPost, "/Startup/User", map[string]any{"Name": cfg.AdminUsername, "Password": password}, nil); err != nil {
			return JellyfinResult{}, err
		}
		if err := j.doStartupJSON(ctx, http.MethodPost, "/Startup/RemoteAccess", map[string]any{"EnableRemoteAccess": true, "EnableAutomaticPortMapping": false}, nil); err != nil {
			return JellyfinResult{}, err
		}
		if err := j.doStartupJSON(ctx, http.MethodPost, "/Startup/Complete", nil, nil); err != nil {
			return JellyfinResult{}, err
		}
	}
	token := existingToken
	if token == "" {
		if password == "" {
			return JellyfinResult{}, fmt.Errorf("admin password is required to obtain a Jellyfin session token")
		}
		var auth struct {
			AccessToken string `json:"AccessToken"`
		}
		j.API.Headers.Set("Authorization", `MediaBrowser Client="YAMS Plus", Device="CLI", DeviceId="yamsplus", Version="0.1.0"`)
		if err := j.API.DoJSON(ctx, http.MethodPost, "/Users/AuthenticateByName", map[string]string{"Username": cfg.AdminUsername, "Pw": password}, &auth); err != nil {
			return JellyfinResult{}, err
		}
		token = auth.AccessToken
	}
	if token == "" {
		return JellyfinResult{}, fmt.Errorf("Jellyfin returned an empty access token")
	}
	j.API.Headers.Set("X-Emby-Token", token)
	if err := j.ensureLibraries(ctx, cfg); err != nil {
		return JellyfinResult{}, err
	}
	result := JellyfinResult{Token: token}
	restart, packages, err := j.ensurePlugins(ctx, cfg.Plugins.RequiredCompatible)
	if err != nil {
		return JellyfinResult{}, err
	}
	result.RestartRequired, result.InstalledPackages = restart, packages
	return result, nil
}

func (j Jellyfin) doStartupJSON(ctx context.Context, method, path string, body, output any) error {
	deadline := time.Now().Add(5 * time.Minute)
	for {
		err := j.API.DoJSON(ctx, method, path, body, output)
		if err == nil {
			return nil
		}
		var httpErr *HTTPError
		retryable := !errors.As(err, &httpErr)
		if httpErr != nil {
			retryable = httpErr.Status == http.StatusBadGateway || httpErr.Status == http.StatusServiceUnavailable || httpErr.Status == http.StatusGatewayTimeout
		}
		if !retryable || time.Now().After(deadline) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

func (j Jellyfin) ensureLibraries(ctx context.Context, cfg config.Config) error {
	var current []struct {
		Name string `json:"Name"`
	}
	if err := j.API.DoJSON(ctx, http.MethodGet, "/Library/VirtualFolders", nil, &current); err != nil {
		return err
	}
	exists := func(name string) bool {
		for _, item := range current {
			if strings.EqualFold(item.Name, name) {
				return true
			}
		}
		return false
	}
	ensure := func(name, kind, path string) error {
		if exists(name) {
			return nil
		}
		query := url.Values{"name": {name}, "collectionType": {kind}, "paths": {path}, "refreshLibrary": {"false"}}
		return j.API.DoJSON(ctx, http.MethodPost, "/Library/VirtualFolders?"+query.Encode(), map[string]any{"LibraryOptions": map[string]any{}}, nil)
	}
	if cfg.Modules.Movies {
		if err := ensure("Movies", "movies", "/data/library/movies"); err != nil {
			return err
		}
	}
	if cfg.Modules.Series {
		if err := ensure("Series", "tvshows", "/data/library/series"); err != nil {
			return err
		}
	}
	if cfg.Modules.Books {
		if err := ensure("Audiobooks", "books", "/data/library/audiobooks"); err != nil {
			return err
		}
	}
	return nil
}

func (j Jellyfin) ensurePlugins(ctx context.Context, required []string) (bool, []string, error) {
	raw, err := stackassets.Files.ReadFile("plugins.yaml")
	if err != nil {
		return false, nil, err
	}
	var catalog pluginCatalog
	if err := yaml.Unmarshal(raw, &catalog); err != nil {
		return false, nil, err
	}
	var repositories []map[string]any
	if err := j.API.DoJSON(ctx, http.MethodGet, "/Repositories", nil, &repositories); err != nil {
		return false, nil, err
	}
	for _, wanted := range catalog.Repositories {
		found := false
		for _, repo := range repositories {
			if strings.EqualFold(fmt.Sprint(repo["Url"]), wanted.URL) || strings.EqualFold(fmt.Sprint(repo["url"]), wanted.URL) {
				found = true
				break
			}
		}
		if !found {
			repositories = append(repositories, map[string]any{"Name": wanted.Name, "Url": wanted.URL, "Enabled": true})
		}
	}
	if err := j.API.DoJSON(ctx, http.MethodPost, "/Repositories", repositories, nil); err != nil {
		return false, nil, err
	}
	var installed []struct {
		Name   string `json:"Name"`
		Status string `json:"Status"`
	}
	if err := j.API.DoJSON(ctx, http.MethodGet, "/Plugins", nil, &installed); err != nil {
		return false, nil, err
	}
	installedNames := make([]string, 0, len(installed))
	restartRequired := false
	for _, plugin := range installed {
		installedNames = append(installedNames, plugin.Name)
		if strings.EqualFold(plugin.Status, "Restart") && containsFold(required, plugin.Name) {
			restartRequired = true
		}
	}
	var available []struct {
		Name string `json:"name"`
	}
	if err := j.API.DoJSON(ctx, http.MethodGet, "/Packages", nil, &available); err != nil {
		return false, nil, err
	}
	var queued []string
	for _, name := range required {
		if containsFold(installedNames, name) {
			continue
		}
		packageName := ""
		for _, candidate := range available {
			if containsFold([]string{candidate.Name}, name) {
				packageName = candidate.Name
				break
			}
		}
		if packageName == "" {
			return false, queued, fmt.Errorf("required Jellyfin plugin %s has no compatible package in the configured manifests", name)
		}
		path := "/Packages/Installed/" + url.PathEscape(packageName)
		if err := j.API.DoJSON(ctx, http.MethodPost, path, nil, nil); err != nil {
			return false, queued, fmt.Errorf("required Jellyfin plugin %s is not installable for the locked server version: %w", name, err)
		}
		queued = append(queued, name)
	}
	return restartRequired || len(queued) > 0, queued, nil
}

func containsFold(values []string, wanted string) bool {
	normalize := func(value string) string {
		return strings.Map(func(r rune) rune {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				return unicode.ToLower(r)
			}
			return -1
		}, value)
	}
	return slices.ContainsFunc(values, func(value string) bool { return normalize(value) == normalize(wanted) })
}

func (j Jellyfin) AuditPlugins(ctx context.Context, required []string) ([]string, error) {
	var installed []struct {
		Name    string `json:"Name"`
		Status  string `json:"Status"`
		Id      string `json:"Id"`
		Version string `json:"Version"`
	}
	if err := j.API.DoJSON(ctx, http.MethodGet, "/Plugins", nil, &installed); err != nil {
		return nil, err
	}
	var missing []string
	for _, name := range required {
		found := false
		for _, plugin := range installed {
			if containsFold([]string{plugin.Name}, name) && !strings.EqualFold(plugin.Status, "Malfunctioned") {
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, name)
		}
	}
	return missing, nil
}

func (j Jellyfin) ConfigureEnhanced(ctx context.Context, seerrAPIKey string, cfg config.Config) error {
	var installed []struct {
		Name string `json:"Name"`
		Id   string `json:"Id"`
	}
	if err := j.API.DoJSON(ctx, http.MethodGet, "/Plugins", nil, &installed); err != nil {
		return err
	}
	id := ""
	for _, plugin := range installed {
		if containsFold([]string{plugin.Name}, "Jellyfin Enhanced") {
			id = plugin.Id
			break
		}
	}
	if id == "" {
		return fmt.Errorf("Jellyfin Enhanced is not installed")
	}
	var current map[string]any
	if err := j.API.DoJSON(ctx, http.MethodGet, "/Plugins/"+id+"/Configuration", nil, &current); err != nil {
		return err
	}
	updates := map[string]any{
		"JellyseerrEnabled": true, "JellyseerrShowSearchResults": true, "JellyseerrShowReportButton": true,
		"JellyseerrEnable4KRequests":   cfg.Modules.Movies && slices.Contains(cfg.Quality.Movies.Profiles, "2160p"),
		"JellyseerrEnable4KTvRequests": cfg.Modules.Series && slices.Contains(cfg.Quality.Series.Profiles, "2160p"),
		"JellyseerrShowAdvanced":       true, "JellyseerrShowSimilar": true, "JellyseerrShowRecommended": true,
		"JellyseerrUrls": "http://seerr:5055", "JellyseerrApiKey": seerrAPIKey, "JellyseerrAutoImportUsers": true,
		"ArrLinksEnabled": true, "RadarrUrl": "http://radarr:7878", "SonarrUrl": "http://sonarr:8989", "BazarrUrl": "http://bazarr:6767",
		"DownloadsPageEnabled": true, "DownloadsUsePluginPages": true, "DownloadsUseCustomTabs": false,
		"DownloadsPagePollingEnabled": true, "DownloadsPollIntervalSeconds": 30, "ShowDownloadsInRequests": true,
		"ThemeSelectorEnabled": true, "AutoSkipIntro": true, "AutoSkipOutro": true, "QualityTagsEnabled": true,
	}
	for key, value := range updates {
		current[key] = value
	}
	return j.API.DoJSON(ctx, http.MethodPost, "/Plugins/"+id+"/Configuration", current, nil)
}

func (j Jellyfin) ConfigurePlaybackRetention(ctx context.Context, days int) error {
	if days <= 0 || days%30 != 0 {
		return fmt.Errorf("playback retention must be a positive multiple of 30 days")
	}
	const path = "/System/Configuration/playback_reporting"
	var current map[string]any
	if err := j.API.DoJSON(ctx, http.MethodGet, path, nil, &current); err != nil {
		return err
	}
	current["MaxDataAge"] = days / 30
	if err := j.API.DoJSON(ctx, http.MethodPost, path, current, nil); err != nil {
		return err
	}
	var verified struct {
		MaxDataAge int `json:"MaxDataAge"`
	}
	if err := j.API.DoJSON(ctx, http.MethodGet, path, nil, &verified); err != nil {
		return err
	}
	if verified.MaxDataAge != days/30 {
		return fmt.Errorf("playback reporting retention verification failed: got %d month(s)", verified.MaxDataAge)
	}
	return nil
}
