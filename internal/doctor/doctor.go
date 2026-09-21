package doctor

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/arturict/yams-plus/internal/config"
	"github.com/arturict/yams-plus/internal/docker"
	"github.com/arturict/yams-plus/internal/layout"
	"github.com/arturict/yams-plus/internal/secrets"
)

type Result struct {
	Status  string  `json:"status"`
	Checks  []Check `json:"checks"`
	Summary string  `json:"summary"`
}

type Check struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

func Run(ctx context.Context, cfg config.Config, paths layout.Layout, includeDocker bool) Result {
	result := Result{Status: "healthy"}
	fileChecks := []struct{ name, path string }{{"config", paths.ConfigFile()}, {"compose", paths.ComposeFile()}, {"stack-lock", paths.LockFile()}}
	for _, item := range fileChecks {
		_, err := os.Stat(item.path)
		status, message := "healthy", item.path
		if err != nil {
			status, message = "failed", err.Error()
		}
		result.Checks = append(result.Checks, Check{Name: item.name, Status: status, Message: message})
	}
	if includeDocker {
		client := docker.Client{ComposeFile: paths.ComposeFile(), ProjectDir: paths.InstallDir(), Timeout: 30 * time.Second}
		containers, err := client.PS(ctx)
		if err != nil {
			result.Checks = append(result.Checks, Check{Name: "containers", Status: "failed", Message: err.Error()})
		} else if len(containers) == 0 {
			result.Checks = append(result.Checks, Check{Name: "containers", Status: "failed", Message: "no containers are running"})
		} else {
			for _, container := range containers {
				status := "healthy"
				if container.State != "running" || container.Health == "unhealthy" {
					status = "failed"
				}
				message := container.State
				if container.Health != "" {
					message += "/" + container.Health
				}
				result.Checks = append(result.Checks, Check{Name: "container-" + container.Service, Status: status, Message: message})
			}
		}
		result.Checks = append(result.Checks, endpointChecks(ctx, cfg)...)
	}
	store := secrets.Store{Dir: paths.SecretsDir()}
	if includeDocker {
		result.Checks = append(result.Checks, prowlarrIndexerCheck(ctx, cfg, store))
	} else if store.Exists("prowlarr_indexers_ready") {
		result.Checks = append(result.Checks, Check{Name: "prowlarr-indexers", Status: "healthy", Message: "last live check found at least one indexer"})
	} else {
		result.Checks = append(result.Checks, Check{Name: "prowlarr-indexers", Status: "action-required", Message: "add at least one legal indexer in Prowlarr, then run doctor again"})
	}
	failed, action := 0, 0
	for _, check := range result.Checks {
		if check.Status == "failed" {
			failed++
		}
		if check.Status == "action-required" {
			action++
		}
	}
	switch {
	case failed > 0:
		result.Status = "failed"
	case action > 0:
		result.Status = "action-required"
	}
	result.Summary = fmt.Sprintf("%d checks, %d failed, %d require action", len(result.Checks), failed, action)
	return result
}

func prowlarrIndexerCheck(ctx context.Context, cfg config.Config, store secrets.Store) Check {
	check := Check{Name: "prowlarr-indexers", Status: "action-required", Message: "add at least one legal indexer in Prowlarr, then run doctor again"}
	key, err := store.Read("prowlarr_api_key")
	if err != nil {
		check.Status, check.Message = "failed", "Prowlarr API key is unavailable"
		return check
	}
	host, err := cfg.LocalHost()
	if err != nil {
		check.Status, check.Message = "failed", err.Error()
		return check
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://%s:%d/api/v1/indexer", host, cfg.Ports.Prowlarr), nil)
	req.Header.Set("X-Api-Key", key)
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		check.Status, check.Message = "failed", err.Error()
		return check
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		check.Status, check.Message = "failed", resp.Status
		return check
	}
	var indexers []map[string]any
	if json.NewDecoder(resp.Body).Decode(&indexers) != nil {
		check.Status, check.Message = "failed", "invalid Prowlarr indexer response"
		return check
	}
	if len(indexers) == 0 {
		// The marker is what doctor --files-only trusts when it cannot reach
		// Prowlarr. Leaving it behind made that report healthy forever once the
		// indexers were removed again.
		_ = store.Remove("prowlarr_indexers_ready")
		return check
	}
	check.Status, check.Message = "healthy", fmt.Sprintf("%d indexer(s) configured", len(indexers))
	_ = store.Write("prowlarr_indexers_ready", fmt.Sprint(len(indexers)))
	return check
}

func endpointChecks(ctx context.Context, cfg config.Config) []Check {
	host, err := cfg.LocalHost()
	if err != nil {
		return []Check{{Name: "endpoints", Status: "failed", Message: err.Error()}}
	}
	targets := map[string]int{"jellyfin": cfg.Ports.Jellyfin, "seerr": cfg.Ports.Seerr, "prowlarr": cfg.Ports.Prowlarr}
	if cfg.Modules.Movies {
		targets["radarr"] = cfg.Ports.Radarr
	}
	if cfg.Modules.Series {
		targets["sonarr"] = cfg.Ports.Sonarr
	}
	if cfg.Modules.Subtitles {
		targets["bazarr"] = cfg.Ports.Bazarr
	}
	if cfg.Downloads.Mode == "usenet" || cfg.Downloads.Mode == "both" {
		targets["sabnzbd"] = cfg.Ports.SABnzbd
	}
	if cfg.Downloads.Mode == "torrent" || cfg.Downloads.Mode == "both" {
		targets["qbittorrent"] = cfg.Ports.QBittorrent
	}
	if cfg.Modules.Books {
		targets["shelfmark"] = cfg.Ports.Shelfmark
		targets["audiobookshelf"] = cfg.Ports.Audiobookshelf
	}
	client := &http.Client{Timeout: 3 * time.Second}
	var checks []Check
	// Ranging over the map directly made doctor --json emit its checks in a
	// different order on every run, which no consumer can diff.
	names := make([]string, 0, len(targets))
	for name := range targets {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		port := targets[name]
		url := fmt.Sprintf("http://%s:%d", host, port)
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		resp, err := client.Do(req)
		status, message := "healthy", "reachable"
		if err != nil {
			status, message = "failed", err.Error()
		} else {
			_ = resp.Body.Close()
			if resp.StatusCode >= 500 {
				status, message = "failed", resp.Status
			} else {
				message = resp.Status
			}
		}
		checks = append(checks, Check{Name: "endpoint-" + name, Status: status, Message: message})
	}
	return checks
}

func JSON(result Result) string { raw, _ := json.MarshalIndent(result, "", "  "); return string(raw) }

func Text(result Result) string {
	var out strings.Builder
	fmt.Fprintf(&out, "YAMS Plus doctor: %s\n", result.Status)
	for _, check := range result.Checks {
		fmt.Fprintf(&out, "%-16s %-15s %s\n", check.Status, check.Name, check.Message)
	}
	fmt.Fprintln(&out, result.Summary)
	return out.String()
}
