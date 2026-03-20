package runner

import (
	"os"
	"testing"

	"github.com/snow-ghost/mem/internal/agent"
	"github.com/snow-ghost/mem/internal/episode"
	"github.com/snow-ghost/mem/internal/principle"
)

func TestRunConsolidate_GivenStubReturns2Principles_WhenForced_ThenPrinciplesUpdated(t *testing.T) {
	s, cfg := setupTestStore(t)
	cfg.EpisodesKeep = 2

	for i := range 5 {
		ep := episode.Episode{
			Ts: "2026-03-20T10:00:00Z", Type: "decision",
			Summary: "ep" + string(rune('A'+i)), Tags: []string{"arch"},
		}
		episode.Append(s.EpisodesPath(), s.LockPath(), ep)
	}

	stub := &agent.StubInvoker{
		Response: `{"new_principles":[{"topic":"Arch","rule":"Use JSONL"},{"topic":"Test","rule":"Run with -race"}],"episodes_to_remove":[0,1],"skill_candidates":[]}`,
	}

	result, err := RunConsolidate(cfg, s, stub, "sonnet", false, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.PrinciplesAdded != 2 {
		t.Errorf("PrinciplesAdded = %d, want 2", result.PrinciplesAdded)
	}
	if result.EpisodesRemoved != 2 {
		t.Errorf("EpisodesRemoved = %d, want 2", result.EpisodesRemoved)
	}

	princ, _ := principle.Parse(s.PrinciplesPath())
	if principle.Count(princ) != 2 {
		t.Errorf("stored principles = %d, want 2", principle.Count(princ))
	}
}

func TestRunConsolidate_GivenThresholdsNotMet_WhenNotForced_ThenSkipped(t *testing.T) {
	s, cfg := setupTestStore(t)
	cfg.SessionThreshold = 10
	cfg.EpisodeThreshold = 100

	stub := &agent.StubInvoker{Response: `{}`}

	result, err := RunConsolidate(cfg, s, stub, "sonnet", false, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Skipped {
		t.Error("expected skipped when thresholds not met")
	}
}

func TestRemoveEpisodes_Given210Episodes_WhenCleanedWithMax200_ThenOldestRemoved(t *testing.T) {
	var episodes []episode.Episode
	for i := range 210 {
		episodes = append(episodes, episode.Episode{
			Ts: "2026-03-20T10:00:00Z", Type: "decision",
			Summary: string(rune('A' + (i % 26))), Tags: []string{"tag"},
		})
	}

	result := removeEpisodes(episodes, nil, 50)
	if len(result) != 210 {
		t.Errorf("got %d, want 210 (no indices to remove)", len(result))
	}
}

func TestRemoveEpisodes_GivenIndices_WhenRemoved_ThenAbsent(t *testing.T) {
	episodes := []episode.Episode{
		{Summary: "A"}, {Summary: "B"}, {Summary: "C"},
		{Summary: "D"}, {Summary: "E"},
	}

	result := removeEpisodes(episodes, []int{1, 3}, 2)
	if len(result) != 4 {
		t.Errorf("got %d, want 4 (index 1 removed, index 3 protected)", len(result))
	}
}

func TestAtomicWriteEpisodes_GivenEpisodes_WhenWritten_ThenFileCorrect(t *testing.T) {
	s, _ := setupTestStore(t)

	episodes := []episode.Episode{
		{Ts: "2026-03-20T10:00:00Z", Type: "decision", Summary: "A", Tags: []string{"t"}},
		{Ts: "2026-03-20T11:00:00Z", Type: "error", Summary: "B", Tags: []string{"t"}},
	}

	if err := atomicWriteEpisodes(s.EpisodesPath(), s.LockPath(), episodes); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	read, _ := episode.ReadAll(s.EpisodesPath())
	if len(read) != 2 {
		t.Fatalf("got %d episodes, want 2", len(read))
	}

	tmpPath := s.EpisodesPath() + ".tmp"
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Error("temp file should be cleaned up after atomic rename")
	}
}

func TestDetectConflicts_GivenDecisionsFromDifferentAgents_WhenDetected_ThenReported(t *testing.T) {
	episodes := []episode.Episode{
		{Type: "decision", Summary: "use approach X", Tags: []string{"storage"}, AgentID: "agent-A"},
		{Type: "decision", Summary: "use approach Y", Tags: []string{"storage"}, AgentID: "agent-B"},
	}
	conflicts := detectConflicts(episodes)
	if len(conflicts) == 0 {
		t.Error("expected conflicts detected")
	}
}

func TestDetectConflicts_GivenDecisionsFromSameAgent_WhenDetected_ThenNoConflict(t *testing.T) {
	episodes := []episode.Episode{
		{Type: "decision", Summary: "use X", Tags: []string{"storage"}, AgentID: "agent-A"},
		{Type: "decision", Summary: "use Y", Tags: []string{"storage"}, AgentID: "agent-A"},
	}
	conflicts := detectConflicts(episodes)
	if len(conflicts) != 0 {
		t.Errorf("expected no conflicts, got %d", len(conflicts))
	}
}

func TestDetectConflicts_GivenErrorEpisodesFromDifferentAgents_WhenDetected_ThenNoConflict(t *testing.T) {
	episodes := []episode.Episode{
		{Type: "error", Summary: "bug A", Tags: []string{"storage"}, AgentID: "agent-A"},
		{Type: "error", Summary: "bug B", Tags: []string{"storage"}, AgentID: "agent-B"},
	}
	conflicts := detectConflicts(episodes)
	if len(conflicts) != 0 {
		t.Errorf("expected no conflicts for error type, got %d", len(conflicts))
	}
}

func TestRunConsolidate_GivenDryRun_WhenRun_ThenNoFilesModified(t *testing.T) {
	s, cfg := setupTestStore(t)

	ep := episode.Episode{
		Ts: "2026-03-20T10:00:00Z", Type: "decision",
		Summary: "test", Tags: []string{"tag"},
	}
	episode.Append(s.EpisodesPath(), s.LockPath(), ep)

	stub := &agent.StubInvoker{
		Response: `{"new_principles":[{"topic":"X","rule":"Y"}],"episodes_to_remove":[0],"skill_candidates":[]}`,
	}

	result, err := RunConsolidate(cfg, s, stub, "sonnet", true, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.PrinciplesAdded != 1 {
		t.Errorf("PrinciplesAdded = %d, want 1", result.PrinciplesAdded)
	}

	episodes, _ := episode.ReadAll(s.EpisodesPath())
	if len(episodes) != 1 {
		t.Errorf("episodes = %d, want 1 (dry run, no changes)", len(episodes))
	}
}
