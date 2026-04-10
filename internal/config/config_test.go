package config

import (
	"os"
	"strings"
	"testing"
)

func TestLoad_GivenNoEnv_WhenLoaded_ThenDefaultPath(t *testing.T) {
	os.Unsetenv("MEM_PALACE")
	cfg := Load()
	if !strings.HasSuffix(cfg.PalacePath, ".mempalace") {
		t.Errorf("PalacePath = %q, want suffix .mempalace", cfg.PalacePath)
	}
	if cfg.DbFile != "palace.db" {
		t.Errorf("DbFile = %q, want palace.db", cfg.DbFile)
	}
}

func TestLoad_GivenEnv_WhenLoaded_ThenCustomPath(t *testing.T) {
	t.Setenv("MEM_PALACE", "/tmp/custom-palace")
	cfg := Load()
	if cfg.PalacePath != "/tmp/custom-palace" {
		t.Errorf("PalacePath = %q, want /tmp/custom-palace", cfg.PalacePath)
	}
}

func TestDbPath_GivenConfig_WhenCalled_ThenCorrectPath(t *testing.T) {
	cfg := Config{PalacePath: "/tmp/palace", DbFile: "palace.db"}
	if cfg.DbPath() != "/tmp/palace/palace.db" {
		t.Errorf("DbPath = %q", cfg.DbPath())
	}
}
