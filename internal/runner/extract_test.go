package runner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/snow-ghost/mem/internal/agent"
	"github.com/snow-ghost/mem/internal/config"
	"github.com/snow-ghost/mem/internal/episode"
	"github.com/snow-ghost/mem/internal/store"
)

func setupTestStore(t *testing.T) (*store.MemoryStore, config.Config) {
	t.Helper()
	dir := t.TempDir()
	root := filepath.Join(dir, ".memory")
	s := store.New(root)
	if err := s.Init(); err != nil {
		t.Fatalf("init store: %v", err)
	}
	cfg := config.Config{
		MemPath:          root,
		SessionThreshold: 10,
		EpisodeThreshold: 100,
		PrinciplesMax:    100,
		EpisodesMax:      200,
		EpisodesKeep:     50,
	}
	return s, cfg
}

func TestRunExtract_GivenStubReturns2Episodes_WhenRun_Then2Appended(t *testing.T) {
	s, cfg := setupTestStore(t)
	stub := &agent.StubInvoker{
		Response: `[{"type":"decision","summary":"chose JSONL","tags":["arch"]},{"type":"error","summary":"race condition found","tags":["bug"]}]`,
	}

	result, err := RunExtract(cfg, s, stub, "abc123", "haiku", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.NewCount != 2 {
		t.Errorf("NewCount = %d, want 2", result.NewCount)
	}

	episodes, _ := episode.ReadAll(s.EpisodesPath())
	if len(episodes) != 2 {
		t.Errorf("stored episodes = %d, want 2", len(episodes))
	}
}

func TestRunExtract_GivenDuplicate_WhenRun_Then0NewAppended(t *testing.T) {
	s, cfg := setupTestStore(t)

	ep := episode.Episode{
		Ts: "2026-03-20T10:00:00Z", Session: "old", Type: "decision",
		Summary: "chose JSONL", Tags: []string{"arch"},
	}
	episode.Append(s.EpisodesPath(), s.LockPath(), ep)

	stub := &agent.StubInvoker{
		Response: `[{"type":"decision","summary":"chose JSONL","tags":["arch"]}]`,
	}

	result, err := RunExtract(cfg, s, stub, "new123", "haiku", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.NewCount != 0 {
		t.Errorf("NewCount = %d, want 0 (dedup)", result.NewCount)
	}
}

func TestRunExtract_GivenEmptyArray_WhenRun_ThenNoErrorAndNoEpisodes(t *testing.T) {
	s, cfg := setupTestStore(t)
	stub := &agent.StubInvoker{Response: `[]`}

	result, err := RunExtract(cfg, s, stub, "abc123", "haiku", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.NewCount != 0 {
		t.Errorf("NewCount = %d, want 0", result.NewCount)
	}
}

func TestRunExtract_GivenMalformedResponse_WhenRun_ThenError(t *testing.T) {
	s, cfg := setupTestStore(t)
	stub := &agent.StubInvoker{Response: `not json at all`}

	_, err := RunExtract(cfg, s, stub, "abc123", "haiku", false)
	if err == nil {
		t.Error("expected error for malformed response")
	}
}

func TestRunExtract_GivenDryRun_WhenRun_ThenNotWritten(t *testing.T) {
	s, cfg := setupTestStore(t)
	stub := &agent.StubInvoker{
		Response: `[{"type":"decision","summary":"test","tags":["t"]}]`,
	}

	result, err := RunExtract(cfg, s, stub, "abc123", "haiku", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.NewCount != 1 {
		t.Errorf("NewCount = %d, want 1", result.NewCount)
	}

	episodes, _ := episode.ReadAll(s.EpisodesPath())
	if len(episodes) != 0 {
		t.Errorf("stored episodes = %d, want 0 (dry run)", len(episodes))
	}
}

func TestThresholdCheck_GivenSessionAt10_WhenChecked_ThenReached(t *testing.T) {
	s, cfg := setupTestStore(t)
	cfg.SessionThreshold = 10
	os.WriteFile(s.SessionCountPath(), []byte("9"), 0644)

	stub := &agent.StubInvoker{
		Response: `[{"type":"decision","summary":"test","tags":["t"]}]`,
	}

	result, err := RunExtract(cfg, s, stub, "abc123", "haiku", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.ThresholdReached {
		t.Error("expected threshold reached at session count 10")
	}
}
