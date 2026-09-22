package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/arturict/yams-plus/internal/apps"
	"github.com/arturict/yams-plus/internal/backup"
	"github.com/arturict/yams-plus/internal/config"
	"github.com/arturict/yams-plus/internal/docker"
	"github.com/arturict/yams-plus/internal/doctor"
	"github.com/arturict/yams-plus/internal/engine"
	"github.com/arturict/yams-plus/internal/layout"
	"github.com/arturict/yams-plus/internal/preflight"
	"github.com/arturict/yams-plus/internal/secrets"
	"github.com/arturict/yams-plus/internal/stack"
	"github.com/arturict/yams-plus/internal/state"
	"github.com/arturict/yams-plus/internal/wizard"
	"golang.org/x/term"
)

var version = "dev"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	root, args, err := globalRoot(args)
	if err != nil {
		return err
	}
	if len(args) == 0 {
		usage()
		return nil
	}
	paths := layout.New(root)
	ctx := context.Background()
	switch args[0] {
	case "help", "--help", "-h":
		usage()
		return nil
	case "version", "--version":
		fmt.Println(version)
		return nil
	case "install":
		return install(ctx, paths, args[1:])
	case "plan":
		return plan(paths, args[1:])
	case "apply":
		return apply(ctx, paths, args[1:])
	case "status":
		// The human summary of the same checks doctor runs; skipping Docker here
		// reported "healthy" with every container dead.
		return runDoctor(ctx, paths, false, true)
	case "doctor":
		fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
		asJSON := fs.Bool("json", false, "machine-readable output")
		filesOnly := fs.Bool("files-only", false, "skip Docker and HTTP checks")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		return runDoctor(ctx, paths, *asJSON, !*filesOnly)
	case "start", "stop", "restart":
		return lifecycle(ctx, paths, args[0])
	case "logs":
		if len(args) != 2 {
			return errors.New("usage: yamsplus logs SERVICE")
		}
		return composeOutput(ctx, paths, "logs", "--tail", "200", args[1])
	case "profiles":
		if len(args) != 2 || args[1] != "sync" {
			return errors.New("usage: yamsplus profiles sync")
		}
		return composeOutput(ctx, paths, "--profile", "tools", "run", "--rm", "recyclarr", "sync")
	case "plugins":
		if len(args) < 2 || args[1] != "audit" {
			return errors.New("usage: yamsplus plugins audit [--json]")
		}
		return pluginAudit(ctx, paths, args[2:])
	case "backup":
		return backupCommand(paths, args[1:])
	case "restore":
		return restoreCommand(paths, args[1:])
	case "update":
		return updateCommand(ctx, paths, args[1:])
	case "uninstall":
		return uninstallCommand(ctx, paths, args[1:])
	default:
		return fmt.Errorf("unknown command %q (run yamsplus help)", args[0])
	}
}

func pluginAudit(ctx context.Context, paths layout.Layout, args []string) error {
	fs := flag.NewFlagSet("plugins audit", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "machine-readable output")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := config.Load(paths.ConfigFile())
	if err != nil {
		return err
	}
	token, err := (secrets.Store{Dir: paths.SecretsDir()}).Read("jellyfin_access_token")
	if err != nil {
		return fmt.Errorf("jellyfin API token is unavailable; run apply first")
	}
	host, err := cfg.LocalHost()
	if err != nil {
		return err
	}
	api := apps.NewHTTPClient(fmt.Sprintf("http://%s:%d", host, cfg.Ports.Jellyfin))
	api.Headers.Set("X-Emby-Token", token)
	missing, err := (apps.Jellyfin{API: api}).AuditPlugins(ctx, cfg.Plugins.RequiredCompatible)
	if err != nil {
		return err
	}
	result := map[string]any{"status": "healthy", "required": cfg.Plugins.RequiredCompatible, "missing": missing}
	if len(missing) > 0 {
		result["status"] = "failed"
	}
	if *asJSON {
		raw, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(raw))
	} else if len(missing) == 0 {
		fmt.Printf("healthy: all %d required compatible plugins are active\n", len(cfg.Plugins.RequiredCompatible))
	} else {
		fmt.Printf("failed: missing or inactive plugins: %s\n", strings.Join(missing, ", "))
	}
	if len(missing) > 0 {
		return errors.New("plugin audit failed")
	}
	return nil
}

func globalRoot(args []string) (string, []string, error) {
	root := os.Getenv("YAMSPLUS_ROOT")
	if root == "" {
		root = "/"
	}
	if len(args) >= 2 && args[0] == "--root" {
		root = args[1]
		args = args[2:]
	} else if len(args) > 0 && strings.HasPrefix(args[0], "--root=") {
		root = strings.TrimPrefix(args[0], "--root=")
		args = args[1:]
	}
	if strings.TrimSpace(root) == "" {
		return "", nil, errors.New("root cannot be empty")
	}
	return root, args, nil
}

func install(ctx context.Context, paths layout.Layout, args []string) error {
	fs := flag.NewFlagSet("install", flag.ContinueOnError)
	configPath := fs.String("config", "", "existing desired-state YAML")
	dryRun := fs.Bool("dry-run", false, "show changes without writing or starting services")
	skipStart := fs.Bool("skip-start", false, "render files without host runtime checks or starting Docker")
	if err := fs.Parse(args); err != nil {
		return err
	}
	var cfg config.Config
	var password string
	values := map[string]string{}
	// The wizard and the prompts below collect the admin password and provider
	// credentials, none of which are kept unless the install gets far enough to
	// store them. Check the host first, so a missing Docker or a missing sudo
	// is reported before anything has been typed rather than after.
	if !*dryRun && !*skipStart {
		if err := requireRoot(paths); err != nil {
			return err
		}
		if err := preflight.Failed(preflight.Run(ctx, isProductionRoot(paths), nil, nil)); err != nil {
			return err
		}
	}
	if *configPath == "" {
		result, err := wizard.New(os.Stdin, os.Stdout).Run()
		if err != nil {
			return err
		}
		cfg, password, values = result.Config, result.AdminPassword, result.Secrets
	} else {
		var err error
		cfg, err = config.Load(*configPath)
		if err != nil {
			return err
		}
		password = os.Getenv("YAMSPLUS_ADMIN_PASSWORD")
		if password == "" && !*dryRun && !*skipStart && adminPasswordNeeded(paths, cfg) {
			password, err = secretPrompt("Admin password")
			if err != nil {
				return err
			}
		}
	}
	if *configPath != "" && !*dryRun && !*skipStart {
		missing, err := collectMissingSecrets(paths, cfg)
		if err != nil {
			return err
		}
		for name, value := range missing {
			values[name] = value
		}
	}
	eng := engine.Engine{Options: engine.Options{Paths: paths, DryRun: *dryRun, SkipPreflight: *skipStart, SkipStart: *skipStart, AdminPassword: password, SecretValues: values, Out: func(format string, values ...any) { fmt.Printf(format, values...) }}}
	return eng.Apply(ctx, cfg)
}

func plan(paths layout.Layout, args []string) error {
	fs := flag.NewFlagSet("plan", flag.ContinueOnError)
	configPath := fs.String("config", paths.ConfigFile(), "desired-state YAML")
	asJSON := fs.Bool("json", false, "machine-readable output")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	changes, err := (engine.Engine{Options: engine.Options{Paths: paths}}).Plan(cfg)
	if err != nil {
		return err
	}
	if *asJSON {
		raw, _ := json.MarshalIndent(changes, "", "  ")
		fmt.Println(string(raw))
		return nil
	}
	for _, change := range changes {
		fmt.Printf("%-10s %s\n", change.Action, change.Path)
	}
	return nil
}

func apply(ctx context.Context, paths layout.Layout, args []string) error {
	fs := flag.NewFlagSet("apply", flag.ContinueOnError)
	dryRun := fs.Bool("dry-run", false, "show changes only")
	skipStart := fs.Bool("skip-start", false, "render files without host runtime checks or starting containers")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if !*dryRun && !*skipStart {
		if err := requireRoot(paths); err != nil {
			return err
		}
	}
	cfg, err := config.Load(paths.ConfigFile())
	if err != nil {
		return err
	}
	password := os.Getenv("YAMSPLUS_ADMIN_PASSWORD")
	if !*dryRun && !*skipStart && adminPasswordNeeded(paths, cfg) {
		if password == "" {
			password, err = secretPrompt("Admin password")
			if err != nil {
				return err
			}
		}
	}
	values := map[string]string{}
	if !*dryRun && !*skipStart {
		values, err = collectMissingSecrets(paths, cfg)
		if err != nil {
			return err
		}
	}
	return (engine.Engine{Options: engine.Options{Paths: paths, DryRun: *dryRun, SkipPreflight: *skipStart, SkipStart: *skipStart, AdminPassword: password, SecretValues: values, Out: func(format string, values ...any) { fmt.Printf(format, values...) }}}).Apply(ctx, cfg)
}

func collectMissingSecrets(paths layout.Layout, cfg config.Config) (map[string]string, error) {
	values := map[string]string{}
	store := secrets.Store{Dir: paths.SecretsDir()}
	for _, requirement := range engine.RequiredSecrets(cfg) {
		if current, err := store.Read(requirement.Name); err == nil && current != "" {
			continue
		}
		envName := "YAMSPLUS_SECRET_" + secretEnvSuffix(requirement.Name)
		value := os.Getenv(envName)
		if value == "" {
			var err error
			value, err = secretPrompt(requirement.Prompt)
			if err != nil {
				return nil, err
			}
		}
		if strings.TrimSpace(value) == "" {
			return nil, fmt.Errorf("%s cannot be empty", requirement.Prompt)
		}
		values[requirement.Name] = value
	}
	return values, nil
}

func secretEnvSuffix(name string) string {
	return strings.ToUpper(strings.NewReplacer("-", "_", ".", "_").Replace(name))
}

func isProductionRoot(paths layout.Layout) bool {
	return paths.Root == "/" || paths.Root == string(filepath.Separator)
}

// requireRoot refuses a real install or apply without root before any prompt.
// Without it the run collected every answer and then failed creating
// /etc/yamsplus with "permission denied".
func requireRoot(paths layout.Layout) error {
	if !isProductionRoot(paths) || runtime.GOOS == "windows" || os.Geteuid() == 0 {
		return nil
	}
	return errors.New("this writes to /etc/yamsplus, /var/lib/yamsplus and /opt/yamsplus and manages containers; rerun it with sudo")
}

// adminPasswordNeeded reports whether an apply must ask for the shared admin
// password. YAMS Plus never stores it, and every service skips its login setup
// when it is empty so that a re-apply cannot overwrite a working password with
// nothing. That makes skipping the prompt safe only when every enabled service
// finished converging on an earlier run and none needs the password again.
// qBittorrent always does: its WebUI login is the password itself.
func adminPasswordNeeded(paths layout.Layout, cfg config.Config) bool {
	if cfg.Downloads.Mode == "torrent" || cfg.Downloads.Mode == "both" {
		return true
	}
	store := secrets.Store{Dir: paths.SecretsDir()}
	if !store.Exists("jellyfin_access_token") {
		return true
	}
	if cfg.Modules.Books && !store.Exists("shelfmark_session") {
		return true
	}
	previous, err := state.Load(paths.StateFile())
	if err != nil {
		return true
	}
	for _, service := range convergedServices(cfg) {
		if previous.Services[service] != "configured" {
			return true
		}
	}
	return false
}

// convergedServices lists the services apps.Converger records in state for cfg.
func convergedServices(cfg config.Config) []string {
	services := []string{"jellyfin", "seerr", "prowlarr"}
	if cfg.Modules.Movies {
		services = append(services, "radarr")
	}
	if cfg.Modules.Series {
		services = append(services, "sonarr")
	}
	if cfg.Modules.Subtitles {
		services = append(services, "bazarr")
	}
	if cfg.Downloads.Mode == "usenet" || cfg.Downloads.Mode == "both" {
		services = append(services, "sabnzbd")
	}
	if cfg.Modules.Books {
		services = append(services, "shelfmark", "audiobookshelf")
	}
	return services
}

func runDoctor(ctx context.Context, paths layout.Layout, asJSON, includeDocker bool) error {
	cfg, err := config.Load(paths.ConfigFile())
	if err != nil {
		return err
	}
	result := doctor.Run(ctx, cfg, paths, includeDocker)
	if asJSON {
		fmt.Println(doctor.JSON(result))
	} else {
		fmt.Print(doctor.Text(result))
	}
	if result.Status == "failed" {
		return errors.New("doctor found failed checks")
	}
	return nil
}

func composeClient(paths layout.Layout) docker.Client {
	return docker.Client{ComposeFile: paths.ComposeFile(), ProjectDir: paths.InstallDir(), Timeout: 20 * time.Minute}
}
func composeOutput(ctx context.Context, paths layout.Layout, args ...string) error {
	out, err := composeClient(paths).Run(ctx, args...)
	fmt.Print(out)
	return err
}
func lifecycle(ctx context.Context, paths layout.Layout, action string) error {
	if action == "start" {
		return composeOutput(ctx, paths, "up", "--detach")
	}
	if action == "stop" {
		return composeOutput(ctx, paths, "stop")
	}
	return composeOutput(ctx, paths, "restart")
}

func backupCommand(paths layout.Layout, args []string) error {
	fs := flag.NewFlagSet("backup", flag.ContinueOnError)
	output := fs.String("output", fmt.Sprintf("yamsplus-%s.tar.gz.age", time.Now().UTC().Format("20060102-150405")), "encrypted archive path")
	includeSecrets := fs.Bool("include-secrets", true, "include provider and API secrets")
	if err := fs.Parse(args); err != nil {
		return err
	}
	passphrase, err := secretPrompt("Backup passphrase")
	if err != nil {
		return err
	}
	if err := backup.Create(paths, *output, passphrase, *includeSecrets); err != nil {
		return err
	}
	fmt.Println("Created encrypted backup", *output)
	return nil
}

func restoreCommand(paths layout.Layout, args []string) error {
	fs := flag.NewFlagSet("restore", flag.ContinueOnError)
	yes := fs.Bool("yes", false, "confirm overwriting configuration")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("usage: yamsplus restore --yes BACKUP")
	}
	if !*yes {
		return errors.New("restore overwrites configuration; rerun with --yes after reviewing the target")
	}
	passphrase, err := secretPrompt("Backup passphrase")
	if err != nil {
		return err
	}
	return backup.Restore(paths, fs.Arg(0), passphrase)
}

func updateCommand(ctx context.Context, paths layout.Layout, args []string) error {
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	yes := fs.Bool("yes", false, "confirm pull and restart")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if !*yes {
		return errors.New("update requires --yes after reviewing stack.lock.yaml")
	}
	// The lock is embedded in the binary, so a newer binary carries newer
	// digests. Pulling against the compose file already on disk would pull the
	// digests the previous binary wrote and update nothing, even though the
	// operations guide says update uses the reviewed stack.lock.yaml.
	cfg, err := config.Load(paths.ConfigFile())
	if err != nil {
		return err
	}
	files, err := stack.Render(cfg, paths)
	if err != nil {
		return err
	}
	changes, err := stack.Plan(files)
	if err != nil {
		return err
	}
	stale := stack.Stale(paths, files)
	for _, path := range stale {
		changes = append(changes, stack.Change{Path: path, Action: "delete"})
	}
	for _, change := range changes {
		fmt.Printf("%-10s %s\n", change.Action, change.Path)
	}
	if err := stack.Write(files); err != nil {
		return err
	}
	if err := stack.Remove(stale); err != nil {
		return err
	}
	// update runs as root and has just rewritten the Recyclarr configuration,
	// which the Recyclarr container reads as PUID. Without this, profile syncs
	// failed until the next apply restored ownership.
	if err := stack.EnsureRuntimeOwnership(cfg, paths); err != nil {
		return err
	}
	if err := composeOutput(ctx, paths, "pull"); err != nil {
		return err
	}
	return composeOutput(ctx, paths, "up", "--detach", "--remove-orphans")
}

func uninstallCommand(ctx context.Context, paths layout.Layout, args []string) error {
	fs := flag.NewFlagSet("uninstall", flag.ContinueOnError)
	yes := fs.Bool("yes", false, "confirm removing containers and configuration")
	removeMedia := fs.Bool("remove-media", false, "also remove the configured media root")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if !*yes {
		return errors.New("uninstall requires --yes")
	}
	// Refuse before touching anything. Tearing the stack down first and then
	// declining left the host with no containers and a full configuration.
	if *removeMedia {
		return errors.New("media deletion requires the interactive safety workflow and is intentionally unavailable in the beta CLI")
	}
	if err := composeClient(paths).Down(ctx); err != nil {
		return err
	}
	for _, target := range []string{paths.ConfigDir(), paths.StateDir(), paths.InstallDir()} {
		if err := safeRemove(paths.Root, target); err != nil {
			return err
		}
	}
	fmt.Println("Removed YAMS Plus containers and configuration. Media was preserved.")
	return nil
}

func safeRemove(root, target string) error {
	targetAbs, err := resolveUnder(root, target)
	if err != nil {
		return err
	}
	return os.RemoveAll(targetAbs)
}

// resolveUnder returns target as an absolute path strictly below root, or an
// error. It touches no filesystem, so the uninstall guard is testable without
// removing anything. The production root is "/", where the separator must not
// be appended twice or every managed directory is refused.
func resolveUnder(root, target string) (string, error) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	prefix := rootAbs
	if !strings.HasSuffix(prefix, string(filepath.Separator)) {
		prefix += string(filepath.Separator)
	}
	if targetAbs == rootAbs || !strings.HasPrefix(targetAbs, prefix) {
		return "", fmt.Errorf("refusing to remove unsafe target %s", targetAbs)
	}
	return targetAbs, nil
}

func secretPrompt(label string) (string, error) {
	fmt.Print(label + ": ")
	if term.IsTerminal(int(os.Stdin.Fd())) {
		raw, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(raw)), nil
	}
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	return strings.TrimSpace(line), err
}

func usage() {
	fmt.Println(`YAMS Plus ` + version + `

Usage:
  yamsplus [--root PATH] COMMAND [OPTIONS]

Commands:
  install, plan, apply, status, doctor
  start, stop, restart, logs SERVICE
  profiles sync, plugins audit
  backup, restore, update, uninstall

The --root override exists for tests and isolated previews. Production defaults
to /etc/yamsplus, /var/lib/yamsplus, /opt/yamsplus, and /srv/yamsplus.`)
}
