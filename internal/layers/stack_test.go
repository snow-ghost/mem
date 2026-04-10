package layers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/snow-ghost/mem/internal/config"
	"github.com/snow-ghost/mem/internal/db"
)

func setupTestDB(t *testing.T) (*db.DB, config.Config) {
	t.Helper()
	dir := t.TempDir()
	d, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	db.InitSchema(d)
	cfg := config.Config{PalacePath: dir, DbFile: "test.db"}
	return d, cfg
}

func TestWakeUp_GivenIdentityAndData_WhenCalled_ThenContainsBoth(t *testing.T) {
	d, cfg := setupTestDB(t)
	defer d.Close()

	os.WriteFile(cfg.IdentityPath(), []byte("I am Atlas, assistant for Alice."), 0644)

	d.Exec("INSERT INTO wings (name, type) VALUES ('myapp', 'project')")
	d.Exec("INSERT INTO rooms (name, wing_id) VALUES ('auth', 1)")
	d.Exec("INSERT INTO drawers (content, content_hash, wing_id, room_id, hall, source_file, source_type) VALUES ('test content', 'hash1', 1, 1, 'facts', 'test.go', 'file')")

	result := WakeUp(d, cfg, "")
	if !strings.Contains(result, "IDENTITY") {
		t.Error("missing L0 identity")
	}
	if !strings.Contains(result, "CRITICAL FACTS") {
		t.Error("missing L1 facts")
	}
	if !strings.Contains(result, "MYAPP") {
		t.Error("missing wing name")
	}
}

func TestWakeUp_GivenNoIdentity_WhenCalled_ThenL1Only(t *testing.T) {
	d, cfg := setupTestDB(t)
	defer d.Close()

	result := WakeUp(d, cfg, "")
	if strings.Contains(result, "IDENTITY") {
		t.Error("should not have L0 when identity.txt missing")
	}
	if !strings.Contains(result, "L1") {
		t.Error("should still have L1")
	}
}

func TestLoadIdentity_GivenFile_WhenLoaded_ThenContentReturned(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "identity.txt")
	os.WriteFile(path, []byte("I am a helpful assistant"), 0644)

	result := LoadIdentity(path)
	if !strings.Contains(result, "helpful assistant") {
		t.Errorf("got %q, want identity content", result)
	}
}

func TestLoadIdentity_GivenMissing_WhenLoaded_ThenEmpty(t *testing.T) {
	result := LoadIdentity("/nonexistent/identity.txt")
	if result != "" {
		t.Errorf("got %q, want empty", result)
	}
}
