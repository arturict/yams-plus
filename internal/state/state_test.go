package state

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	before := time.Now().UTC().Add(-time.Second)
	saved := State{
		Phase:        "rendered",
		ConfigDigest: Digest([]byte("desired state")),
		Services:     map[string]string{"jellyfin": "configured"},
		Actions:      []string{"add at least one legal indexer in Prowlarr"},
	}
	if err := Save(path, saved); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.SchemaVersion != 1 {
		t.Fatalf("schemaVersion = %d, want 1", loaded.SchemaVersion)
	}
	if loaded.UpdatedAt.Before(before) || loaded.UpdatedAt.After(time.Now().UTC().Add(time.Second)) {
		t.Fatalf("updatedAt = %v is not stamped at save time", loaded.UpdatedAt)
	}
	if loaded.Phase != saved.Phase || loaded.ConfigDigest != saved.ConfigDigest {
		t.Fatalf("loaded = %+v, want phase and digest of %+v", loaded, saved)
	}
	if loaded.Services["jellyfin"] != "configured" || len(loaded.Actions) != 1 {
		t.Fatalf("services/actions did not round trip: %+v", loaded)
	}
}

// The state file records service inventory next to the secrets directory, so it
// must never be world readable.
func TestSaveUsesRestrictedFileMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX file modes are not enforced on Windows")
	}
	path := filepath.Join(t.TempDir(), "state.json")
	if err := Save(path, State{Phase: "rendered"}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o640 {
		t.Fatalf("mode = %o, want 640", info.Mode().Perm())
	}
}

func TestSaveOverwritesExistingState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := Save(path, State{Phase: "rendered", Actions: []string{"pending"}}); err != nil {
		t.Fatal(err)
	}
	if err := Save(path, State{Phase: "configured"}); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Phase != "configured" || len(loaded.Actions) != 0 {
		t.Fatalf("loaded = %+v, want the second save without stale actions", loaded)
	}
}

func TestLoadReportsMissingFile(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "absent.json")); !os.IsNotExist(err) {
		t.Fatalf("err = %v, want not-exist", err)
	}
}
