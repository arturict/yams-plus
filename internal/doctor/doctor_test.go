package doctor

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/arturict/yams-plus/internal/config"
	"github.com/arturict/yams-plus/internal/layout"
	"github.com/arturict/yams-plus/internal/secrets"
)

func TestRunFilesOnlyUsesStableActionRequiredState(t *testing.T) {
	paths := layout.New(t.TempDir())
	for _, path := range []string{paths.ConfigFile(), paths.ComposeFile(), paths.LockFile()} {
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("test"), 0o640); err != nil {
			t.Fatal(err)
		}
	}
	result := Run(context.Background(), config.Default(), paths, false)
	if result.Status != "action-required" || result.Summary != "4 checks, 0 failed, 1 require action" {
		t.Fatalf("%s", JSON(result))
	}
	var decoded Result
	if err := json.Unmarshal([]byte(JSON(result)), &decoded); err != nil || decoded.Status != result.Status {
		t.Fatalf("doctor JSON is not stable: %v", err)
	}
}

func TestProwlarrIndexerCheckPersistsSuccessfulLiveCheck(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Api-Key") != "local-key" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{{"name": "authorised-test-indexer"}})
	}))
	defer server.Close()
	port := server.Listener.Addr().(*net.TCPAddr).Port

	paths := layout.New(t.TempDir())
	store := secrets.Store{Dir: paths.SecretsDir()}
	if err := store.Write("prowlarr_api_key", "local-key"); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.BindAddresses = []string{"127.0.0.1"}
	cfg.Ports.Prowlarr = port
	check := prowlarrIndexerCheck(context.Background(), cfg, store)
	if check.Status != "healthy" || !store.Exists("prowlarr_indexers_ready") {
		t.Fatalf("check=%#v marker=%v", check, store.Exists("prowlarr_indexers_ready"))
	}
}

// A config without bind addresses must be reported, never panic the CLI.
func TestChecksReportMissingBindAddress(t *testing.T) {
	cfg := config.Default()
	cfg.BindAddresses = nil
	checks := endpointChecks(context.Background(), cfg)
	if len(checks) != 1 || checks[0].Status != "failed" {
		t.Fatalf("checks=%#v", checks)
	}
	check := prowlarrIndexerCheck(context.Background(), cfg, secrets.Store{Dir: t.TempDir()})
	if check.Status != "failed" {
		t.Fatalf("check=%#v", check)
	}
}

// doctor --json is consumed by scripts, so the check list must not depend on
// Go map iteration order.
func TestEndpointChecksAreOrderedDeterministically(t *testing.T) {
	cfg := config.Default()
	cfg.Modules.Books = true
	cfg.Downloads.Mode = "both"
	first := endpointChecks(context.Background(), cfg)
	if len(first) < 2 {
		t.Fatalf("expected several endpoint checks, got %d", len(first))
	}
	names := make([]string, len(first))
	for i, check := range first {
		names[i] = check.Name
	}
	if !sort.StringsAreSorted(names) {
		t.Fatalf("checks are not in a stable sorted order: %v", names)
	}
	for i := 0; i < 8; i++ {
		again := endpointChecks(context.Background(), cfg)
		for j := range again {
			if again[j].Name != names[j] {
				t.Fatalf("run %d produced %v, want %v", i, again, names)
			}
		}
	}
}
