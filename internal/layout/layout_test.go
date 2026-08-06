package layout

import (
	"path/filepath"
	"testing"
)

func TestLayoutUsesIsolatedRoot(t *testing.T) {
	l := New(filepath.Join("tmp", "sandbox"))
	want, err := filepath.Abs(filepath.Join("tmp", "sandbox", "etc", "yamsplus", "yamsplus.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if got := l.ConfigFile(); got != want {
		t.Fatalf("ConfigFile() = %q, want %q", got, want)
	}
}

func TestLayoutMakesRelativeTestRootAbsolute(t *testing.T) {
	l := New(".yamsplus-test/example")
	if !filepath.IsAbs(l.Root) {
		t.Fatalf("root is not absolute: %s", l.Root)
	}
}
