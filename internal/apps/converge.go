package apps

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/arturict/yams-plus/internal/config"
	"github.com/arturict/yams-plus/internal/docker"
	"github.com/arturict/yams-plus/internal/layout"
	"github.com/arturict/yams-plus/internal/secrets"
)

type Converger struct {
	Config        config.Config
	Paths         layout.Layout
	Compose       docker.Client
	AdminPassword string
	Out           func(string, ...any)
}

type ConvergeResult struct {
	Services map[string]string
	Actions  []string
}

type arrState struct {
	client   Servarr
	key      string
	profiles []map[string]any
	roots    []map[string]any
}

func (c Converger) Run(ctx context.Context) (ConvergeResult, error) {
	out := c.Out
	if out == nil {
		out = func(string, ...any) {}
	}
	store := secrets.Store{Dir: c.Paths.SecretsDir()}
	result := ConvergeResult{Services: map[string]string{}, Actions: []string{"add at least one legal indexer in Prowlarr"}}
	readOptional := func(name string) string { value, _ := store.Read(name); return value }
	host, err := c.Config.LocalHost()
	if err != nil {
		return result, err
	}

	out("Configuring Jellyfin through its API...\n")
	jellyfinAPI := NewHTTPClient(fmt.Sprintf("http://%s:%d", host, c.Config.Ports.Jellyfin))
	jellyfin := Jellyfin{API: jellyfinAPI}
	jellyResult, err := jellyfin.Converge(ctx, c.Config, c.AdminPassword, readOptional("jellyfin_access_token"))
	if err != nil {
		return result, err
	}
	if err := store.Write("jellyfin_access_token", jellyResult.Token); err != nil {
		return result, err
	}
	result.Services["jellyfin"] = "configured"

	arrs := map[string]*arrState{}
	configureArr := func(name string, port int, apiVersion, root string) error {
		api := NewHTTPClient(fmt.Sprintf("http://%s:%d", host, port))
		client := Servarr{Name: name, API: api, APIVersion: apiVersion}
		if err := client.Wait(ctx); err != nil {
			return err
		}
		key, err := DiscoverAPIKey(filepath.Join(c.Paths.AppsDir(), name), 3*time.Minute)
		if err != nil {
			return err
		}
		client.Authenticate(key)
		if err := client.EnsureHostAuth(ctx, c.Config.AdminUsername, c.AdminPassword); err != nil {
			return err
		}
		if root != "" {
			if err := client.EnsureRootFolder(ctx, root); err != nil {
				return err
			}
		}
		if err := store.Write(name+"_api_key", key); err != nil {
			return err
		}
		arrs[name] = &arrState{client: client, key: key}
		result.Services[name] = "configured"
		return nil
	}
	out("Securing and connecting the Arr applications...\n")
	if err := configureArr("prowlarr", c.Config.Ports.Prowlarr, "v1", ""); err != nil {
		return result, err
	}
	if c.Config.Modules.Movies {
		if err := configureArr("radarr", c.Config.Ports.Radarr, "v3", "/data/library/movies"); err != nil {
			return result, err
		}
	}
	if c.Config.Modules.Series {
		if err := configureArr("sonarr", c.Config.Ports.Sonarr, "v3", "/data/library/series"); err != nil {
			return result, err
		}
	}

	var sabKey, sabHost string
	if c.Config.Downloads.Mode == "usenet" || c.Config.Downloads.Mode == "both" {
		out("Configuring SABnzbd with TLS-only provider settings...\n")
		sabAPI := NewHTTPClient(fmt.Sprintf("http://%s:%d", host, c.Config.Ports.SABnzbd))
		sab := SABnzbd{API: sabAPI}
		if err := sab.Wait(ctx); err != nil {
			return result, err
		}
		sabKey, err = DiscoverINIValue(filepath.Join(c.Paths.AppsDir(), "sabnzbd"), "api_key", 3*time.Minute)
		if err != nil {
			return result, err
		}
		sab.APIKey = sabKey
		providerUser, err := store.Read(c.Config.Downloads.Usenet.UsernameSecret)
		if err != nil {
			return result, err
		}
		providerPassword, err := store.Read(c.Config.Downloads.Usenet.PasswordSecret)
		if err != nil {
			return result, err
		}
		if err := sab.Converge(ctx, c.Config, c.AdminPassword, providerUser, providerPassword); err != nil {
			return result, err
		}
		if err := store.Write("sabnzbd_api_key", sabKey); err != nil {
			return result, err
		}
		result.Services["sabnzbd"] = "configured"
		sabHost = "sabnzbd"
		if c.Config.Downloads.Usenet.UseVPN {
			sabHost = "gluetun"
		}
	}

	var qbitHost string
	if c.Config.Downloads.Mode == "torrent" || c.Config.Downloads.Mode == "both" {
		out("Configuring qBittorrent and its download categories...\n")
		qbitAPI := NewHTTPClient(fmt.Sprintf("http://%s:%d", host, c.Config.Ports.QBittorrent))
		qbit := QBittorrent{API: qbitAPI}
		if err := qbit.Wait(ctx); err != nil {
			return result, err
		}
		loginErr := qbit.Login(ctx, c.Config.AdminUsername, c.AdminPassword)
		if loginErr != nil {
			logs, err := c.Compose.Logs(ctx, "qbittorrent")
			if err != nil {
				return result, fmt.Errorf("qBittorrent login failed and startup password could not be read: %w", loginErr)
			}
			temporaryPassword, err := TemporaryQBittorrentPassword(logs)
			if err != nil {
				return result, fmt.Errorf("qBittorrent login failed: %w; %v", loginErr, err)
			}
			if err := qbit.Login(ctx, "admin", temporaryPassword); err != nil {
				return result, err
			}
		}
		if err := qbit.Converge(ctx, c.Config.AdminUsername, c.AdminPassword); err != nil {
			return result, err
		}
		qbitHost = "qbittorrent"
		if c.Config.Downloads.Torrent.UseVPN {
			qbitHost = "gluetun"
		}
		result.Services["qbittorrent"] = "configured"
	}

	for name, arr := range arrs {
		if name == "prowlarr" {
			continue
		}
		category := name
		categoryField := "movieCategory"
		if name == "sonarr" {
			categoryField = "tvCategory"
		}
		if sabKey != "" {
			spec := DownloadClientSpec{Name: "YAMS+ SABnzbd", Implementation: "Sabnzbd", Protocol: "usenet", Priority: 1, Fields: map[string]any{"host": sabHost, "port": 8080, "useSsl": false, "apiKey": sabKey, categoryField: category}}
			if err := arr.client.EnsureDownloadClient(ctx, spec); err != nil {
				return result, err
			}
		}
		if qbitHost != "" {
			spec := DownloadClientSpec{Name: "YAMS+ qBittorrent", Implementation: "QBittorrent", Protocol: "torrent", Priority: 2, Fields: map[string]any{"host": qbitHost, "port": 8081, "useSsl": false, "username": c.Config.AdminUsername, "password": c.AdminPassword, categoryField: category}}
			if err := arr.client.EnsureDownloadClient(ctx, spec); err != nil {
				return result, err
			}
		}
		if c.Config.Downloads.Mode == "both" {
			if err := arr.client.EnsureUsenetFallback(ctx); err != nil {
				return result, err
			}
		}
	}

	if c.Config.Modules.Subtitles {
		out("Connecting Bazarr and creating the subtitle language profile...\n")
		bazarrKey, err := DiscoverBazarrAPIKey(filepath.Join(c.Paths.AppsDir(), "bazarr"), 3*time.Minute)
		if err != nil {
			return result, err
		}
		providerUsername, err := store.Read(c.Config.Subtitles.UsernameSecret)
		if err != nil {
			return result, err
		}
		providerPassword, err := store.Read(c.Config.Subtitles.PasswordSecret)
		if err != nil {
			return result, err
		}
		sonarrKey, radarrKey := "", ""
		if arrs["sonarr"] != nil {
			sonarrKey = arrs["sonarr"].key
		}
		if arrs["radarr"] != nil {
			radarrKey = arrs["radarr"].key
		}
		bazarr := Bazarr{API: NewHTTPClient(fmt.Sprintf("http://%s:%d", host, c.Config.Ports.Bazarr)), APIKey: bazarrKey}
		if err := bazarr.Converge(ctx, c.Config, c.AdminPassword, sonarrKey, radarrKey, providerUsername, providerPassword); err != nil {
			return result, err
		}
		if err := store.Write("bazarr_api_key", bazarrKey); err != nil {
			return result, err
		}
		result.Services["bazarr"] = "configured"
	}

	if c.Config.Modules.Books {
		out("Configuring the Books beta module (Shelfmark + Audiobookshelf)...\n")
		shelfmarkURL := fmt.Sprintf("http://%s:%d", host, c.Config.Ports.Shelfmark)
		existingShelfmarkSession := readOptional("shelfmark_session")
		shelfmark := Shelfmark{API: NewHTTPClient(shelfmarkURL)}
		shelfmarkSession, err := shelfmark.Converge(ctx, c.Config, c.AdminPassword, existingShelfmarkSession, arrs["prowlarr"].key, sabKey, sabHost, qbitHost)
		if err != nil {
			return result, err
		}
		if existingShelfmarkSession == "" {
			out("Restarting Shelfmark once to activate built-in authentication...\n")
			if _, err := c.Compose.Run(ctx, "restart", "shelfmark"); err != nil {
				return result, err
			}
			shelfmark = Shelfmark{API: NewHTTPClient(shelfmarkURL)}
			shelfmarkSession, err = shelfmark.Converge(ctx, c.Config, c.AdminPassword, shelfmarkSession, arrs["prowlarr"].key, sabKey, sabHost, qbitHost)
			if err != nil {
				return result, err
			}
		}
		if err := store.Write("shelfmark_session", shelfmarkSession); err != nil {
			return result, err
		}
		abs := Audiobookshelf{API: NewHTTPClient(fmt.Sprintf("http://%s:%d", host, c.Config.Ports.Audiobookshelf))}
		token, err := abs.Converge(ctx, c.Config.AdminUsername, c.AdminPassword, readOptional("audiobookshelf_access_token"))
		if err != nil {
			return result, err
		}
		if err := store.Write("audiobookshelf_access_token", token); err != nil {
			return result, err
		}
		result.Services["shelfmark"] = "configured"
		result.Services["audiobookshelf"] = "configured"
	}

	out("Syncing the selected Recyclarr/TRaSH profiles...\n")
	if err := c.writeRecyclarrSecrets(arrs); err != nil {
		return result, err
	}
	if _, err := c.Compose.Run(ctx, "--profile", "tools", "run", "--rm", "recyclarr", "sync"); err != nil {
		return result, err
	}
	for name, arr := range arrs {
		if name == "prowlarr" {
			continue
		}
		arr.profiles, err = arr.client.QualityProfiles(ctx)
		if err != nil {
			return result, err
		}
		if err := arr.client.API.DoJSON(ctx, "GET", arr.client.prefix()+"/rootfolder", nil, &arr.roots); err != nil {
			return result, err
		}
	}

	prowlarr := arrs["prowlarr"]
	if radarr := arrs["radarr"]; radarr != nil {
		if err := prowlarr.client.EnsureApplication(ctx, "Radarr", "YAMS+ Radarr", "http://radarr:7878", radarr.key); err != nil {
			return result, err
		}
	}
	if sonarr := arrs["sonarr"]; sonarr != nil {
		if err := prowlarr.client.EnsureApplication(ctx, "Sonarr", "YAMS+ Sonarr", "http://sonarr:8989", sonarr.key); err != nil {
			return result, err
		}
	}

	out("Connecting Seerr and enabling automatic requests...\n")
	seerrAPI := NewHTTPClient(fmt.Sprintf("http://%s:%d", host, c.Config.Ports.Seerr))
	seerr := Seerr{API: seerrAPI}
	seerrKey, err := seerr.Bootstrap(ctx, c.Config, c.AdminPassword, host, readOptional("seerr_api_key"))
	if err != nil {
		return result, err
	}
	seerrAPI.Headers.Set("X-Api-Key", seerrKey)
	if err := store.Write("seerr_api_key", seerrKey); err != nil {
		return result, err
	}
	if radarr := arrs["radarr"]; radarr != nil {
		if err := seerr.EnsureServarr(ctx, c.Config, "radarr", radarr.profiles, radarr.roots, radarr.key); err != nil {
			return result, err
		}
	}
	if sonarr := arrs["sonarr"]; sonarr != nil {
		if err := seerr.EnsureServarr(ctx, c.Config, "sonarr", sonarr.profiles, sonarr.roots, sonarr.key); err != nil {
			return result, err
		}
	}
	result.Services["seerr"] = "configured"

	if jellyResult.RestartRequired {
		out("Restarting Jellyfin once to activate the plugin pack...\n")
		if _, err := c.Compose.Run(ctx, "restart", "jellyfin"); err != nil {
			return result, err
		}
		if err := jellyfinAPI.WaitStatus(ctx, "/Plugins", 5*time.Minute, 200); err != nil {
			return result, err
		}
	}
	if err := jellyfin.ConfigurePlaybackRetention(ctx, c.Config.Plugins.PlaybackRetentionDays); err != nil {
		return result, err
	}
	if err := jellyfin.ConfigureEnhanced(ctx, seerrKey, c.Config); err != nil {
		return result, err
	}
	missing, err := jellyfin.AuditPlugins(ctx, c.Config.Plugins.RequiredCompatible)
	if err != nil {
		return result, err
	}
	if len(missing) > 0 {
		return result, fmt.Errorf("required Jellyfin plugins are missing or malfunctioned after restart: %v", missing)
	}
	count, err := prowlarr.client.IndexerCount(ctx)
	if err != nil {
		return result, err
	}
	if count > 0 {
		if err := store.Write("prowlarr_indexers_ready", fmt.Sprint(count)); err != nil {
			return result, err
		}
		result.Actions = nil
	}
	return result, nil
}

func (c Converger) writeRecyclarrSecrets(arrs map[string]*arrState) error {
	var out strings.Builder
	for _, name := range []string{"radarr", "sonarr"} {
		if arr := arrs[name]; arr != nil {
			fmt.Fprintf(&out, "%s_api_key: %s\n", name, strconv.Quote(arr.key))
		}
	}
	if err := os.MkdirAll(c.Paths.RecyclarrDir(), 0o750); err != nil {
		return err
	}
	path := filepath.Join(c.Paths.RecyclarrDir(), "secrets.yml")
	if err := os.WriteFile(path, []byte(out.String()), 0o600); err != nil {
		return err
	}
	if runtime.GOOS != "windows" {
		if err := os.Chown(path, c.Config.Runtime.PUID, c.Config.Runtime.PGID); err != nil {
			return fmt.Errorf("set ownership on %s: %w", path, err)
		}
	}
	return nil
}
