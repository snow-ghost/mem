package config

import (
	"os"
	"testing"
)

func TestLoad_GivenNoEnvVars_WhenLoaded_ThenDefaultsApplied(t *testing.T) {
	clearEnv(t)
	cfg := Load()

	if cfg.MemPath != ".memory" {
		t.Errorf("MemPath = %q, want %q", cfg.MemPath, ".memory")
	}
	if cfg.SessionThreshold != 10 {
		t.Errorf("SessionThreshold = %d, want 10", cfg.SessionThreshold)
	}
	if cfg.EpisodeThreshold != 100 {
		t.Errorf("EpisodeThreshold = %d, want 100", cfg.EpisodeThreshold)
	}
	if cfg.PrinciplesMax != 100 {
		t.Errorf("PrinciplesMax = %d, want 100", cfg.PrinciplesMax)
	}
	if cfg.EpisodesMax != 200 {
		t.Errorf("EpisodesMax = %d, want 200", cfg.EpisodesMax)
	}
	if cfg.EpisodesKeep != 50 {
		t.Errorf("EpisodesKeep = %d, want 50", cfg.EpisodesKeep)
	}
	if cfg.AgentID == "" {
		t.Errorf("AgentID should be auto-generated, got empty")
	}
}

func TestLoad_GivenCustomThreshold_WhenLoaded_ThenOverrideApplied(t *testing.T) {
	clearEnv(t)
	t.Setenv("MEM_SESSION_THRESHOLD", "20")
	cfg := Load()

	if cfg.SessionThreshold != 20 {
		t.Errorf("SessionThreshold = %d, want 20", cfg.SessionThreshold)
	}
}

func TestLoad_GivenAgentID_WhenLoaded_ThenAgentIDSet(t *testing.T) {
	clearEnv(t)
	t.Setenv("MEM_AGENT_ID", "agent-1")
	cfg := Load()

	if cfg.AgentID != "agent-1" {
		t.Errorf("AgentID = %q, want %q", cfg.AgentID, "agent-1")
	}
}

func TestLoad_GivenInvalidInt_WhenLoaded_ThenDefaultUsed(t *testing.T) {
	clearEnv(t)
	t.Setenv("MEM_SESSION_THRESHOLD", "notanumber")
	cfg := Load()

	if cfg.SessionThreshold != 10 {
		t.Errorf("SessionThreshold = %d, want 10 (default)", cfg.SessionThreshold)
	}
}

func clearEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"MEM_PATH", "MEM_SESSION_THRESHOLD", "MEM_EPISODE_THRESHOLD",
		"MEM_PRINCIPLES_MAX", "MEM_EPISODES_MAX", "MEM_EPISODES_KEEP",
		"MEM_AGENT_ID",
	} {
		if v, ok := os.LookupEnv(key); ok {
			t.Setenv(key, v) // will restore after test
		}
		os.Unsetenv(key)
	}
}
