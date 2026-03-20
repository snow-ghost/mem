package principle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParse_GivenMarkdownWith2Topics_WhenParsed_ThenCorrectMap(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "principles.md")
	content := `# Project Principles

## Architecture
- Use JSONL for append-only logs
- Keep files under 150 lines

## Testing
- Run tests with -race flag
`
	os.WriteFile(path, []byte(content), 0644)

	p, err := Parse(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(p) != 2 {
		t.Fatalf("got %d topics, want 2", len(p))
	}
	if len(p["Architecture"]) != 2 {
		t.Errorf("Architecture rules = %d, want 2", len(p["Architecture"]))
	}
	if len(p["Testing"]) != 1 {
		t.Errorf("Testing rules = %d, want 1", len(p["Testing"]))
	}
}

func TestParse_GivenEmptyFile_WhenParsed_ThenEmptyMap(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "principles.md")
	os.WriteFile(path, []byte(""), 0644)

	p, err := Parse(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(p) != 0 {
		t.Errorf("got %d topics, want 0", len(p))
	}
}

func TestParse_GivenNonExistentFile_WhenParsed_ThenEmptyMap(t *testing.T) {
	p, err := Parse("/nonexistent/principles.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(p) != 0 {
		t.Errorf("got %d topics, want 0", len(p))
	}
}

func TestWrite_GivenPrinciples_WhenWritten_ThenValidMarkdown(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "principles.md")

	p := Principles{
		"Architecture": {"Use JSONL", "Keep files small"},
		"Testing":      {"Run with -race"},
	}

	if err := Write(path, p); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(path)
	content := string(data)
	if !strings.Contains(content, "# Project Principles") {
		t.Error("missing main heading")
	}
	if !strings.Contains(content, "## Architecture") {
		t.Error("missing Architecture heading")
	}
	if !strings.Contains(content, "- Use JSONL") {
		t.Error("missing rule")
	}
}

func TestCount_GivenPrinciples_WhenCounted_ThenCorrectTotal(t *testing.T) {
	p := Principles{
		"A": {"r1", "r2"},
		"B": {"r3"},
	}
	if got := Count(p); got != 3 {
		t.Errorf("Count = %d, want 3", got)
	}
}
