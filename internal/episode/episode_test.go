package episode

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidate_GivenValidFields_WhenValidated_ThenNoError(t *testing.T) {
	ep := Episode{
		Ts: "2026-03-20T10:00:00Z", Type: "decision",
		Summary: "chose JSONL", Tags: []string{"arch"},
	}
	if err := ep.Validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_GivenUnknownType_WhenValidated_ThenError(t *testing.T) {
	ep := Episode{
		Ts: "2026-03-20T10:00:00Z", Type: "foo",
		Summary: "test", Tags: []string{"tag"},
	}
	if err := ep.Validate(); err == nil {
		t.Error("expected error for unknown type")
	}
}

func TestValidate_GivenEmptySummary_WhenValidated_ThenError(t *testing.T) {
	ep := Episode{
		Ts: "2026-03-20T10:00:00Z", Type: "decision",
		Summary: "", Tags: []string{"tag"},
	}
	if err := ep.Validate(); err == nil {
		t.Error("expected error for empty summary")
	}
}

func TestValidate_GivenZeroTags_WhenValidated_ThenError(t *testing.T) {
	ep := Episode{
		Ts: "2026-03-20T10:00:00Z", Type: "decision",
		Summary: "test", Tags: []string{},
	}
	if err := ep.Validate(); err == nil {
		t.Error("expected error for zero tags")
	}
}

func TestValidate_GivenFourTags_WhenValidated_ThenError(t *testing.T) {
	ep := Episode{
		Ts: "2026-03-20T10:00:00Z", Type: "decision",
		Summary: "test", Tags: []string{"a", "b", "c", "d"},
	}
	if err := ep.Validate(); err == nil {
		t.Error("expected error for 4 tags")
	}
}

func TestValidate_GivenInvalidTimestamp_WhenValidated_ThenError(t *testing.T) {
	ep := Episode{
		Ts: "not-a-date", Type: "decision",
		Summary: "test", Tags: []string{"tag"},
	}
	if err := ep.Validate(); err == nil {
		t.Error("expected error for invalid timestamp")
	}
}

func TestReadAll_GivenCorruptLine_WhenRead_ThenSkippedAndValidReturned(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "episodes.jsonl")
	content := `{"ts":"2026-03-20T10:00:00Z","type":"decision","summary":"good","tags":["arch"]}
NOT JSON
{"ts":"2026-03-20T11:00:00Z","type":"error","summary":"also good","tags":["bug"]}
`
	os.WriteFile(path, []byte(content), 0644)

	episodes, err := ReadAll(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(episodes) != 2 {
		t.Fatalf("got %d episodes, want 2", len(episodes))
	}
	if episodes[0].Summary != "good" {
		t.Errorf("first summary = %q, want %q", episodes[0].Summary, "good")
	}
}

func TestAppend_GivenValidEpisode_WhenAppended_ThenFileContainsRecord(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "episodes.jsonl")
	lockPath := filepath.Join(dir, ".lock")

	ep := Episode{
		Ts: "2026-03-20T10:00:00Z", Type: "decision",
		Summary: "chose JSONL", Tags: []string{"arch"},
	}
	if err := Append(path, lockPath, ep); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	episodes, err := ReadAll(path)
	if err != nil {
		t.Fatalf("read error: %v", err)
	}
	if len(episodes) != 1 {
		t.Fatalf("got %d episodes, want 1", len(episodes))
	}
	if episodes[0].Summary != "chose JSONL" {
		t.Errorf("summary = %q, want %q", episodes[0].Summary, "chose JSONL")
	}
}

func TestReadAll_GivenEmptyFile_WhenRead_ThenEmptySlice(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "episodes.jsonl")
	os.WriteFile(path, nil, 0644)

	episodes, err := ReadAll(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(episodes) != 0 {
		t.Fatalf("got %d episodes, want 0", len(episodes))
	}
}

func TestReadAll_GivenNonExistentFile_WhenRead_ThenNilSlice(t *testing.T) {
	episodes, err := ReadAll("/nonexistent/path/episodes.jsonl")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if episodes != nil {
		t.Fatalf("got %v, want nil", episodes)
	}
}

func TestReadLast_GivenFiveEpisodes_WhenReadLast3_ThenLast3Returned(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "episodes.jsonl")
	lockPath := filepath.Join(dir, ".lock")

	for i := range 5 {
		ep := Episode{
			Ts: "2026-03-20T10:00:00Z", Type: "decision",
			Summary: string(rune('A' + i)), Tags: []string{"tag"},
		}
		Append(path, lockPath, ep)
	}

	last, err := ReadLast(path, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(last) != 3 {
		t.Fatalf("got %d, want 3", len(last))
	}
	if last[0].Summary != "C" {
		t.Errorf("first of last 3 = %q, want %q", last[0].Summary, "C")
	}
}
