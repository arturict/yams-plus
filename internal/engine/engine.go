package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/arturict/yams-plus/internal/apps"
	"github.com/arturict/yams-plus/internal/config"
	"github.com/arturict/yams-plus/internal/docker"
	"github.com/arturict/yams-plus/internal/layout"
	"github.com/arturict/yams-plus/internal/preflight"
	"github.com/arturict/yams-plus/internal/secrets"
	"github.com/arturict/yams-plus/internal/stack"
	"github.com/arturict/yams-plus/internal/state"
	"gopkg.in/yaml.v3"
)

type Options struct {
	Paths         layout.Layout
	DryRun        bool
	SkipPreflight bool
	SkipStart     bool
	AdminPassword string
	SecretValues  map[string]string
	Out           func(string, ...any)
}

type Engine struct{ Options Options }

func (e Engine) Plan(cfg config.Config) ([]stack.Change, error) {
	files, err := stack.Render(cfg, e.Options.Paths)
	if err != nil {
		return nil, err
	}
	return stack.Plan(files)
}

func (e Engine) Apply(ctx context.Context, cfg config.Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	out := e.Options.Out
	if out == nil {
		out = func(string, ...any) {}
	}
	production := e.Options.Paths.Root == "/" || e.Options.Paths.Root == string(filepath.Separator)
	_, configErr := os.Stat(e.Options.Paths.ConfigFile())
	freshInstall := os.IsNotExist(configErr)
	if configErr != nil && !freshInstall {
		return configErr
	}
	if !e.Options.SkipPreflight {
		var ports []preflight.PortBinding
		if production && !e.Options.DryRun && !e.Options.SkipStart && freshInstall {
			ports = enabledPortBindings(cfg)
		}
		checks := preflight.Run(ctx, production && !e.Options.DryRun, cfg.BindAddresses, ports)
		for _, check := range checks {
			out("%-16s %-20s %s\n", check.Status, check.Name, check.Message)
		}
		if err := preflight.Failed(checks); err != nil && !e.Options.DryRun {
			return err
		}
		if !freshInstall {
			out("%-16s %-20s %s\n", "healthy", "existing-state", "existing YAMS Plus configuration will be converged")
		} else {
			out("%-16s %-20s %s\n", "healthy", "existing-state", "fresh installation")
		}
	}
	files, err := stack.Render(cfg, e.Options.Paths)
	if err != nil {
		return err
	}
	changes, err := stack.Plan(files)
	if err != nil {
		return err
	}
	for _, change := range changes {
		out("%-10s %s\n", change.Action, change.Path)
	}
	if e.Options.DryRun {
		return nil
	}
	ensureDirectories := stack.EnsureDirectories
	if e.Options.SkipStart {
		ensureDirectories = stack.EnsureRenderDirectories
	}
	if err := ensureDirectories(cfg, e.Options.Paths); err != nil {
		return err
	}
	if err := config.Save(e.Options.Paths.ConfigFile(), cfg); err != nil {
		return err
	}
	store := secrets.Store{Dir: e.Options.Paths.SecretsDir()}
	for name, value := range e.Options.SecretValues {
		if err := store.Write(name, value); err != nil {
			return err
		}
	}
	if !e.Options.SkipStart {
		if err := validateRequiredSecrets(cfg, store); err != nil {
			return err
		}
	}
	if err := stack.Write(files); err != nil {
		return err
	}
	configRaw, _ := yaml.Marshal(cfg)
	current := state.State{Phase: "rendered", ConfigDigest: state.Digest(configRaw), Services: map[string]string{}, Actions: []string{"add at least one legal indexer in Prowlarr"}}
	if err := state.Save(e.Options.Paths.StateFile(), current); err != nil {
		return err
	}
	if e.Options.SkipStart {
		return nil
	}
	if err := stack.EnsureRuntimeOwnership(cfg, e.Options.Paths); err != nil {
		return err
	}
	client := docker.Client{ComposeFile: e.Options.Paths.ComposeFile(), ProjectDir: e.Options.Paths.InstallDir(), Timeout: 20 * time.Minute}
	if err := client.Validate(ctx); err != nil {
		return err
	}
	out("Starting the selected services. First pull may take a while.\n")
	if err := client.Up(ctx); err != nil {
		return err
	}
	current.Phase = "containers-started"
	if err := state.Save(e.Options.Paths.StateFile(), current); err != nil {
		return err
	}
	// Application API convergence is deliberately checkpointed after the
	// containers so a failed service can resume without rebuilding host paths.
	converger := apps.Converger{Config: cfg, Paths: e.Options.Paths, Compose: client, AdminPassword: e.Options.AdminPassword, Out: out}
	result, err := converger.Run(ctx)
	if err != nil {
		current.Phase = "application-convergence-failed"
		current.Actions = []string{err.Error()}
		_ = state.Save(e.Options.Paths.StateFile(), current)
		return err
	}
	current.Services, current.Actions = result.Services, result.Actions
	if len(result.Actions) > 0 {
		current.Phase = "awaiting-indexer"
	} else {
		current.Phase = "configured"
	}
	return state.Save(e.Options.Paths.StateFile(), current)
}

func enabledPortBindings(cfg config.Config) []preflight.PortBinding {
	ports := []preflight.PortBinding{
		{Name: "jellyfin", Port: cfg.Ports.Jellyfin},
		{Name: "jellyfin-discovery", Port: cfg.Ports.JellyfinDiscovery, Protocol: "udp"},
		{Name: "seerr", Port: cfg.Ports.Seerr},
		{Name: "prowlarr", Port: cfg.Ports.Prowlarr},
	}
	if cfg.Modules.Movies {
		ports = append(ports, preflight.PortBinding{Name: "radarr", Port: cfg.Ports.Radarr})
	}
	if cfg.Modules.Series {
		ports = append(ports, preflight.PortBinding{Name: "sonarr", Port: cfg.Ports.Sonarr})
	}
	if cfg.Modules.Subtitles {
		ports = append(ports, preflight.PortBinding{Name: "bazarr", Port: cfg.Ports.Bazarr})
	}
	if cfg.Downloads.Mode == "usenet" || cfg.Downloads.Mode == "both" {
		ports = append(ports, preflight.PortBinding{Name: "sabnzbd", Port: cfg.Ports.SABnzbd})
	}
	if cfg.Downloads.Mode == "torrent" || cfg.Downloads.Mode == "both" {
		ports = append(ports, preflight.PortBinding{Name: "qbittorrent", Port: cfg.Ports.QBittorrent})
	}
	if cfg.Modules.Books {
		ports = append(ports,
			preflight.PortBinding{Name: "shelfmark", Port: cfg.Ports.Shelfmark},
			preflight.PortBinding{Name: "audiobookshelf", Port: cfg.Ports.Audiobookshelf},
		)
	}
	return ports
}

func validateRequiredSecrets(cfg config.Config, store secrets.Store) error {
	var missing []string
	for _, requirement := range RequiredSecrets(cfg) {
		value, err := store.Read(requirement.Name)
		if err != nil || value == "" {
			missing = append(missing, requirement.Name)
		}
	}
	if len(missing) > 0 {
		raw, _ := json.Marshal(missing)
		return fmt.Errorf("missing or empty required secret files: %s", raw)
	}
	return nil
}

type SecretRequirement struct {
	Name   string
	Prompt string
}

func RequiredSecrets(cfg config.Config) []SecretRequirement {
	var required []SecretRequirement
	need := func(name, prompt string) {
		if name != "" {
			required = append(required, SecretRequirement{Name: name, Prompt: prompt})
		}
	}
	if cfg.Downloads.Mode == "usenet" || cfg.Downloads.Mode == "both" {
		need(cfg.Downloads.Usenet.UsernameSecret, "Usenet provider username")
		need(cfg.Downloads.Usenet.PasswordSecret, "Usenet provider password")
	}
	if cfg.Downloads.Usenet.UseVPN || ((cfg.Downloads.Mode == "torrent" || cfg.Downloads.Mode == "both") && cfg.Downloads.Torrent.UseVPN) {
		if cfg.Downloads.VPN.Type == "wireguard" {
			need(cfg.Downloads.VPN.PrivateKeySecret, "WireGuard private key")
		} else {
			need(cfg.Downloads.VPN.UsernameSecret, "OpenVPN username")
			need(cfg.Downloads.VPN.PasswordSecret, "OpenVPN password")
		}
	}
	if cfg.Modules.Subtitles {
		need(cfg.Subtitles.UsernameSecret, "Subtitle provider username")
		need(cfg.Subtitles.PasswordSecret, "Subtitle provider password")
	}
	return required
}

func LoadOrDefault(path string) (config.Config, error) {
	if path == "" {
		return config.Config{}, fmt.Errorf("config path is required")
	}
	cfg, err := config.Load(path)
	if os.IsNotExist(err) {
		return config.Default(), nil
	}
	return cfg, err
}
