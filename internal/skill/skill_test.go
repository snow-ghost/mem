package skill

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParse_GivenValidSkillMarkdown_WhenParsed_ThenAllSectionsPopulated(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "database-migration.md")
	content := `# Database Migration

## When to apply
- Need to change DB schema

## Prerequisites
- Database access
- Backup of current schema

## Steps
1. Create migration file
2. Write SQL
3. Apply migration

## Success verification
- Migration status shows Applied
- Tests pass

## Anti-patterns
- Do not edit generated model files
`
	os.WriteFile(path, []byte(content), 0644)

	s, err := Parse(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Name != "Database Migration" {
		t.Errorf("Name = %q, want %q", s.Name, "Database Migration")
	}
	if len(s.Triggers) != 1 {
		t.Errorf("Triggers = %d, want 1", len(s.Triggers))
	}
	if len(s.Prerequisites) != 2 {
		t.Errorf("Prerequisites = %d, want 2", len(s.Prerequisites))
	}
	if len(s.Steps) != 3 {
		t.Errorf("Steps = %d, want 3", len(s.Steps))
	}
	if len(s.Verification) != 2 {
		t.Errorf("Verification = %d, want 2", len(s.Verification))
	}
	if len(s.Antipatterns) != 1 {
		t.Errorf("Antipatterns = %d, want 1", len(s.Antipatterns))
	}
}

func TestWrite_GivenSkill_WhenWritten_ThenMatchesExpectedFormat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test-skill.md")

	s := Skill{
		Name:          "Test Skill",
		Triggers:      []string{"when testing"},
		Prerequisites: []string{"go installed"},
		Steps:         []string{"write test", "run test"},
		Verification:  []string{"tests pass"},
		Antipatterns:  []string{"skip tests"},
	}

	if err := Write(path, s); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(path)
	content := string(data)
	if !strings.Contains(content, "# Test Skill") {
		t.Error("missing skill name heading")
	}
	if !strings.Contains(content, "1. write test") {
		t.Error("missing numbered step")
	}
}

func TestList_GivenDirWith3Files_WhenListed_Then3SlugsReturned(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.md", "b.md", "c.md"} {
		os.WriteFile(filepath.Join(dir, name), []byte("# test"), 0644)
	}

	slugs, err := List(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(slugs) != 3 {
		t.Fatalf("got %d slugs, want 3", len(slugs))
	}
}

func TestList_GivenNonExistentDir_WhenListed_ThenNilReturned(t *testing.T) {
	slugs, err := List("/nonexistent/dir")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if slugs != nil {
		t.Errorf("got %v, want nil", slugs)
	}
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"Given Database Migration, Then database-migration", "Database Migration", "database-migration"},
		{"Given Fix: Race Condition!!, Then fix-race-condition", "Fix: Race Condition!!", "fix-race-condition"},
		{"Given leading/trailing spaces, Then trimmed", "  hello world  ", "hello-world"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Slugify(tt.in); got != tt.want {
				t.Errorf("Slugify(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
