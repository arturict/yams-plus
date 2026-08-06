package stack

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"text/template"

	"github.com/arturict/yams-plus/internal/config"
	"github.com/arturict/yams-plus/internal/layout"
	stackassets "github.com/arturict/yams-plus/stack"
	"gopkg.in/yaml.v3"
)

type Lock struct {
	SchemaVersion int                    `yaml:"schemaVersion"`
	ResolvedAt    string                 `yaml:"resolvedAt"`
	Architecture  string                 `yaml:"architecture"`
	Services      map[string]LockService `yaml:"services"`
}

type LockService struct {
	Version string `yaml:"version"`
	Image   string `yaml:"image"`
}

type RenderedFile struct {
	Path    string
	Mode    os.FileMode
	Content []byte
}

type Change struct {
	Path   string `json:"path"`
	Action string `json:"action"`
	Digest string `json:"digest"`
}

type renderContext struct {
	config.Config
	RecyclarrDir string
}

func LoadEmbeddedLock() (Lock, []byte, error) {
	raw, err := stackassets.Files.ReadFile("stack.lock.yaml")
	if err != nil {
		return Lock{}, nil, err
	}
	var lock Lock
	if err := yaml.Unmarshal(raw, &lock); err != nil {
		return Lock{}, nil, fmt.Errorf("decode stack lock: %w", err)
	}
	if lock.SchemaVersion != 1 || lock.Architecture != "amd64" {
		return Lock{}, nil, fmt.Errorf("unsupported stack lock")
	}
	for name, service := range lock.Services {
		if !strings.Contains(service.Image, "@sha256:") {
			return Lock{}, nil, fmt.Errorf("service %s is not digest pinned", name)
		}
	}
	return lock, raw, nil
}

func Render(cfg config.Config, paths layout.Layout) ([]RenderedFile, error) {
	lock, lockRaw, err := LoadEmbeddedLock()
	if err != nil {
		return nil, err
	}
	image := func(name string) (string, error) {
		service, ok := lock.Services[name]
		if !ok {
			return "", fmt.Errorf("image %q missing from lock", name)
		}
		return service.Image, nil
	}
	dataRoot := paths.DataDir(cfg.DataRoot)
	funcs := template.FuncMap{
		"image":      image,
		"quote":      strconv.Quote,
		"appPath":    func(name string) string { return filepath.ToSlash(filepath.Join(paths.AppsDir(), name)) },
		"dataPath":   func(name string) string { return filepath.ToSlash(filepath.Join(dataRoot, name)) },
		"secretPath": func(name string) string { return filepath.ToSlash(filepath.Join(paths.SecretsDir(), name)) },
		"yesno": func(v bool) string {
			if v {
				return "on"
			}
			return "off"
		},
		"hasProfile": func(values []string, value string) bool { return slices.Contains(values, value) },
		"vpnEnabled": func() bool {
			return cfg.Downloads.Usenet.UseVPN || ((cfg.Downloads.Mode == "torrent" || cfg.Downloads.Mode == "both") && cfg.Downloads.Torrent.UseVPN)
		},
	}
	ctx := renderContext{Config: cfg, RecyclarrDir: filepath.ToSlash(paths.RecyclarrDir())}
	renderTemplate := func(asset, destination string) (RenderedFile, error) {
		raw, err := stackassets.Files.ReadFile(asset)
		if err != nil {
			return RenderedFile{}, err
		}
		tpl, err := template.New(filepath.Base(asset)).Funcs(funcs).Option("missingkey=error").Parse(string(raw))
		if err != nil {
			return RenderedFile{}, fmt.Errorf("parse %s: %w", asset, err)
		}
		var out bytes.Buffer
		if err := tpl.Execute(&out, ctx); err != nil {
			return RenderedFile{}, fmt.Errorf("render %s: %w", asset, err)
		}
		return RenderedFile{Path: destination, Mode: 0o640, Content: out.Bytes()}, nil
	}
	compose, err := renderTemplate("compose.yaml.tmpl", paths.ComposeFile())
	if err != nil {
		return nil, err
	}
	files := []RenderedFile{
		compose,
		{Path: paths.LockFile(), Mode: 0o640, Content: lockRaw},
	}
	plugins, err := stackassets.Files.ReadFile("plugins.yaml")
	if err != nil {
		return nil, err
	}
	files = append(files, RenderedFile{Path: filepath.Join(paths.InstallDir(), "plugins.yaml"), Mode: 0o640, Content: plugins})
	if cfg.Modules.Movies {
		file, err := renderTemplate("recyclarr/radarr.yaml.tmpl", filepath.Join(paths.RecyclarrDir(), "configs", "radarr.yaml"))
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	if cfg.Modules.Series {
		file, err := renderTemplate("recyclarr/sonarr.yaml.tmpl", filepath.Join(paths.RecyclarrDir(), "configs", "sonarr.yaml"))
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	return files, nil
}

func Plan(files []RenderedFile) ([]Change, error) {
	changes := make([]Change, 0, len(files))
	for _, file := range files {
		digest := sha256.Sum256(file.Content)
		change := Change{Path: file.Path, Digest: hex.EncodeToString(digest[:])}
		existing, err := os.ReadFile(file.Path)
		switch {
		case os.IsNotExist(err):
			change.Action = "create"
		case err != nil:
			return nil, fmt.Errorf("read %s: %w", file.Path, err)
		case bytes.Equal(existing, file.Content):
			change.Action = "unchanged"
		default:
			change.Action = "update"
		}
		changes = append(changes, change)
	}
	return changes, nil
}

func Write(files []RenderedFile) error {
	for _, file := range files {
		if err := os.MkdirAll(filepath.Dir(file.Path), 0o750); err != nil {
			return err
		}
		existing, err := os.ReadFile(file.Path)
		if err == nil && bytes.Equal(existing, file.Content) {
			continue
		}
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		tmp, err := os.CreateTemp(filepath.Dir(file.Path), ".yamsplus-*")
		if err != nil {
			return err
		}
		tmpName := tmp.Name()
		cleanup := func() { _ = tmp.Close(); _ = os.Remove(tmpName) }
		if err := tmp.Chmod(file.Mode); err != nil {
			cleanup()
			return err
		}
		if _, err := tmp.Write(file.Content); err != nil {
			cleanup()
			return err
		}
		if err := tmp.Sync(); err != nil {
			cleanup()
			return err
		}
		if err := tmp.Close(); err != nil {
			cleanup()
			return err
		}
		if err := os.Rename(tmpName, file.Path); err != nil {
			cleanup()
			return err
		}
	}
	return nil
}

func RequiredDirectories(cfg config.Config, paths layout.Layout) []string {
	data := paths.DataDir(cfg.DataRoot)
	dirs := []string{paths.ConfigDir(), paths.SecretsDir(), paths.StateDir(), paths.AppsDir(), paths.InstallDir(), paths.RecyclarrDir(),
		data, filepath.Join(data, "downloads"), filepath.Join(data, "downloads", "usenet"),
		filepath.Join(data, "downloads", "usenet", "incomplete"), filepath.Join(data, "downloads", "usenet", "complete"),
		filepath.Join(data, "downloads", "torrents"), filepath.Join(data, "library"),
		filepath.Join(data, "library", "movies"), filepath.Join(data, "library", "series")}
	if cfg.Modules.Books {
		dirs = append(dirs, filepath.Join(data, "library", "books"), filepath.Join(data, "library", "audiobooks"))
	}
	for _, app := range []string{"jellyfin", "seerr", "prowlarr"} {
		dirs = append(dirs, filepath.Join(paths.AppsDir(), app))
	}
	if cfg.Modules.Movies {
		dirs = append(dirs, filepath.Join(paths.AppsDir(), "radarr"))
	}
	if cfg.Modules.Series {
		dirs = append(dirs, filepath.Join(paths.AppsDir(), "sonarr"))
	}
	if cfg.Modules.Subtitles {
		dirs = append(dirs, filepath.Join(paths.AppsDir(), "bazarr"))
	}
	if cfg.Downloads.Mode == "usenet" || cfg.Downloads.Mode == "both" {
		dirs = append(dirs, filepath.Join(paths.AppsDir(), "sabnzbd"))
	}
	if cfg.Downloads.Mode == "torrent" || cfg.Downloads.Mode == "both" {
		dirs = append(dirs, filepath.Join(paths.AppsDir(), "qbittorrent"))
	}
	if cfg.Modules.Books {
		dirs = append(dirs, filepath.Join(paths.AppsDir(), "shelfmark"), filepath.Join(paths.AppsDir(), "audiobookshelf", "config"), filepath.Join(paths.AppsDir(), "audiobookshelf", "metadata"))
	}
	return dirs
}

func EnsureDirectories(cfg config.Config, paths layout.Layout) error {
	appsRoot := filepath.Clean(paths.AppsDir())
	dataRoot := filepath.Clean(paths.DataDir(cfg.DataRoot))
	recyclarrRoot := filepath.Clean(paths.RecyclarrDir())
	for _, dir := range RequiredDirectories(cfg, paths) {
		mode := os.FileMode(0o750)
		if strings.Contains(filepath.ToSlash(dir), "/srv/") || strings.Contains(filepath.ToSlash(dir), "/library") || strings.Contains(filepath.ToSlash(dir), "/downloads") {
			mode = 0o770
		}
		if err := os.MkdirAll(dir, mode); err != nil {
			return fmt.Errorf("create %s: %w", dir, err)
		}
		if err := os.Chmod(dir, mode); err != nil {
			return fmt.Errorf("set permissions on %s: %w", dir, err)
		}
		if runtime.GOOS != "windows" && (within(dir, appsRoot) || within(dir, dataRoot) || within(dir, recyclarrRoot)) {
			if err := os.Chown(dir, cfg.Runtime.PUID, cfg.Runtime.PGID); err != nil {
				return fmt.Errorf("set ownership on %s: %w", dir, err)
			}
		}
	}
	return nil
}

// EnsureRuntimeOwnership makes generated Recyclarr configuration readable and
// its state directory writable by the unprivileged UID used in the container.
func EnsureRuntimeOwnership(cfg config.Config, paths layout.Layout) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	return filepath.Walk(paths.RecyclarrDir(), func(path string, _ os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if err := os.Chown(path, cfg.Runtime.PUID, cfg.Runtime.PGID); err != nil {
			return fmt.Errorf("set ownership on %s: %w", path, err)
		}
		return nil
	})
}

func within(path, root string) bool {
	rel, err := filepath.Rel(root, filepath.Clean(path))
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
