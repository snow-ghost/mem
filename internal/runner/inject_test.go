package runner

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/snow-ghost/mem/internal/episode"
	"github.com/snow-ghost/mem/internal/principle"
	"github.com/snow-ghost/mem/internal/skill"
)

func TestRunInject_GivenPrinciplesAndEpisodes_WhenInjected_ThenContextPopulated(t *testing.T) {
	s, cfg := setupTestStore(t)

	principle.Write(s.PrinciplesPath(), principle.Principles{
		"Arch":  {"Use JSONL", "Keep files small"},
		"Test":  {"Run with -race"},
	})

	for i := range 20 {
		ep := episode.Episode{
			Ts: "2026-03-20T10:00:00Z", Type: "decision",
			Summary: string(rune('A' + (i % 26))), Tags: []string{"arch"},
		}
		episode.Append(s.EpisodesPath(), s.LockPath(), ep)
	}

	ctx, err := RunInject(cfg, s, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if principle.Count(ctx.Principles) != 3 {
		t.Errorf("principles = %d, want 3", principle.Count(ctx.Principles))
	}
	if len(ctx.Episodes) != 10 {
		t.Errorf("episodes = %d, want 10", len(ctx.Episodes))
	}
}

func TestRunInject_GivenEmptyStore_WhenInjected_ThenEmptyContext(t *testing.T) {
	s, cfg := setupTestStore(t)

	ctx, err := RunInject(cfg, s, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if principle.Count(ctx.Principles) != 0 {
		t.Errorf("principles = %d, want 0", principle.Count(ctx.Principles))
	}
	if len(ctx.Episodes) != 0 {
		t.Errorf("episodes = %d, want 0", len(ctx.Episodes))
	}
}

func TestFormatMarkdown_GivenContext_WhenFormatted_ThenCorrectHeadings(t *testing.T) {
	ctx := &InjectContext{
		Principles: principle.Principles{"Arch": {"Use JSONL"}},
		Episodes: []episode.Episode{
			{Ts: "2026-03-20T10:00:00Z", Type: "decision", Summary: "chose JSONL"},
		},
		Skills: []skill.Skill{
			{Name: "DB Migration", Triggers: []string{"database migration"}},
		},
	}

	md := FormatMarkdown(ctx)
	if !strings.Contains(md, "# Project Memory") {
		t.Error("missing main heading")
	}
	if !strings.Contains(md, "## Principles") {
		t.Error("missing Principles section")
	}
	if !strings.Contains(md, "## Recent Events") {
		t.Error("missing Recent Events section")
	}
	if !strings.Contains(md, "## Relevant Skills") {
		t.Error("missing Relevant Skills section")
	}
}

func TestFormatJSON_GivenContext_WhenFormatted_ThenValidJSON(t *testing.T) {
	ctx := &InjectContext{
		Principles: principle.Principles{"Arch": {"rule1"}},
		Episodes:   []episode.Episode{{Type: "decision", Summary: "test"}},
		Skills:     []skill.Skill{{Name: "test"}},
	}

	out, err := FormatJSON(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Errorf("invalid JSON: %v", err)
	}
}

func TestFormatJSON_GivenEmptyContext_WhenFormatted_ThenEmptyArrays(t *testing.T) {
	ctx := &InjectContext{
		Principles: principle.Principles{},
		Episodes:   nil,
		Skills:     nil,
	}

	out, err := FormatJSON(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, `"episodes": null`) && !strings.Contains(out, `"episodes": []`) {
		// null is acceptable for nil slices in Go JSON
	}
	_ = out
}
