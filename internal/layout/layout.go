package layout

import (
	"path/filepath"
	"strings"
)

// Layout centralizes every host path so tests can safely operate below a
// temporary root instead of touching the real machine.
type Layout struct {
	Root string
}

func New(root string) Layout {
	if strings.TrimSpace(root) == "" {
		root = "/"
	}
	cleaned := filepath.Clean(root)
	if cleaned != "/" && cleaned != string(filepath.Separator) && !filepath.IsAbs(cleaned) {
		if absolute, err := filepath.Abs(cleaned); err == nil {
			cleaned = absolute
		}
	}
	return Layout{Root: cleaned}
}

func (l Layout) under(path string) string {
	path = strings.TrimLeft(filepath.Clean(path), `/\`)
	if l.Root == string(filepath.Separator) || l.Root == "/" {
		return filepath.Join(string(filepath.Separator), path)
	}
	return filepath.Join(l.Root, path)
}

func (l Layout) ConfigDir() string  { return l.under("etc/yamsplus") }
func (l Layout) ConfigFile() string { return filepath.Join(l.ConfigDir(), "yamsplus.yaml") }
func (l Layout) SecretsDir() string { return filepath.Join(l.ConfigDir(), "secrets") }
func (l Layout) StateDir() string   { return l.under("var/lib/yamsplus") }
func (l Layout) StateFile() string  { return filepath.Join(l.StateDir(), "state.json") }
func (l Layout) AppsDir() string    { return filepath.Join(l.StateDir(), "apps") }
func (l Layout) InstallDir() string { return l.under("opt/yamsplus") }
func (l Layout) ComposeFile() string {
	return filepath.Join(l.InstallDir(), "compose.yaml")
}
func (l Layout) LockFile() string { return filepath.Join(l.InstallDir(), "stack.lock.yaml") }
func (l Layout) RecyclarrDir() string {
	return filepath.Join(l.InstallDir(), "recyclarr")
}
func (l Layout) DataDir(configured string) string {
	if strings.TrimSpace(configured) == "" {
		return l.under("srv/yamsplus")
	}
	if (filepath.IsAbs(configured) || strings.HasPrefix(configured, "/")) && l.Root != "/" && l.Root != string(filepath.Separator) {
		return l.under(configured)
	}
	return filepath.Clean(configured)
}
