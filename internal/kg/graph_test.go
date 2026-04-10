package kg

import (
	"path/filepath"
	"testing"

	"github.com/snow-ghost/mem/internal/db"
)

func openTestDB(t *testing.T) *db.DB {
	t.Helper()
	dir := t.TempDir()
	d, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	db.InitSchema(d)
	return d
}

func TestAddTriple_GivenFact_WhenQueried_ThenReturned(t *testing.T) {
	d := openTestDB(t)
	defer d.Close()

	AddTriple(d, "Kai", "works_on", "Orion", "2025-06-01", "")

	results, err := QueryEntity(d, "Kai", "", "outgoing")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].ObjName != "Orion" {
		t.Errorf("object = %q, want Orion", results[0].ObjName)
	}
	if !results[0].Current {
		t.Error("expected current=true")
	}
}

func TestInvalidate_GivenFact_WhenInvalidated_ThenNotCurrent(t *testing.T) {
	d := openTestDB(t)
	defer d.Close()

	AddTriple(d, "Kai", "works_on", "Orion", "2025-06-01", "")
	Invalidate(d, "Kai", "works_on", "Orion", "2026-03-01")

	results, err := QueryEntity(d, "Kai", "2026-04-01", "outgoing")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("got %d results, want 0 (invalidated)", len(results))
	}
}

func TestTimeline_GivenMultipleFacts_WhenQueried_ThenChronological(t *testing.T) {
	d := openTestDB(t)
	defer d.Close()

	AddTriple(d, "Kai", "works_on", "Orion", "2025-06-01", "")
	AddTriple(d, "Kai", "recommended", "Clerk", "2026-01-15", "")

	results, err := Timeline(d, "Kai")
	if err != nil {
		t.Fatalf("timeline: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("got %d, want 2", len(results))
	}
	if results[0].ValidFrom > results[1].ValidFrom {
		t.Error("expected chronological order")
	}
}

func TestContradiction_GivenConflictingFacts_WhenChecked_ThenDetected(t *testing.T) {
	d := openTestDB(t)
	defer d.Close()

	AddTriple(d, "Kai", "works_on", "Orion", "2025-06-01", "")

	conflicts, err := CheckContradiction(d, "Kai", "works_on", "Nova")
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if len(conflicts) != 1 {
		t.Fatalf("got %d conflicts, want 1", len(conflicts))
	}
	if conflicts[0].ExistingObj != "Orion" {
		t.Errorf("existing = %q, want Orion", conflicts[0].ExistingObj)
	}
}

func TestContradiction_GivenSameFact_WhenChecked_ThenNoConflict(t *testing.T) {
	d := openTestDB(t)
	defer d.Close()

	AddTriple(d, "Kai", "works_on", "Orion", "", "")

	conflicts, _ := CheckContradiction(d, "Kai", "works_on", "Orion")
	if len(conflicts) != 0 {
		t.Errorf("got %d conflicts, want 0", len(conflicts))
	}
}
