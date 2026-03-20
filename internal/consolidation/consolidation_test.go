package consolidation

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAppend_GivenEmptyLog_WhenAppended_ThenFileContainsFormattedEntry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "consolidation-log.md")

	entry := LogEntry{
		Date:              "2026-03-20",
		Number:            1,
		EpisodesProcessed: 12,
		PrinciplesAdded:   2,
		PrinciplesUpdated: 1,
		PrinciplesRemoved: 0,
		EpisodesRemoved:   3,
		SkillsCreated:     0,
		SkillCandidates:   []string{"database-migration (3 occurrences)"},
	}

	if err := Append(path, entry); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(path)
	content := string(data)
	if len(content) == 0 {
		t.Fatal("file is empty")
	}
}

func TestReadLast_GivenLogWithEntries_WhenReadLast_ThenMostRecentReturned(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "consolidation-log.md")
	os.WriteFile(path, []byte("# Consolidation Log\n"), 0644)

	Append(path, LogEntry{Date: "2026-03-18", Number: 1, EpisodesProcessed: 10})
	Append(path, LogEntry{Date: "2026-03-19", Number: 2, EpisodesProcessed: 15})
	Append(path, LogEntry{Date: "2026-03-20", Number: 3, EpisodesProcessed: 20})

	entry, err := ReadLast(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.Date != "2026-03-20" {
		t.Errorf("Date = %q, want %q", entry.Date, "2026-03-20")
	}
	if entry.Number != 3 {
		t.Errorf("Number = %d, want 3", entry.Number)
	}
	if entry.EpisodesProcessed != 20 {
		t.Errorf("EpisodesProcessed = %d, want 20", entry.EpisodesProcessed)
	}
}

func TestReadLast_GivenEmptyFile_WhenReadLast_ThenZeroEntry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "consolidation-log.md")
	os.WriteFile(path, []byte(""), 0644)

	entry, err := ReadLast(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.Number != 0 {
		t.Errorf("Number = %d, want 0", entry.Number)
	}
}
