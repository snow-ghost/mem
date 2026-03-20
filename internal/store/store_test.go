package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInit_GivenEmptyDir_WhenInitCalled_ThenAllFilesCreated(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".memory")
	s := New(root)

	if err := s.Init(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, path := range []string{
		s.EpisodesPath(), s.PrinciplesPath(), s.ConsolidationLogPath(),
	} {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("file not created: %s", path)
		}
	}
	for _, dir := range []string{s.SkillsDir(), s.PromptsDir()} {
		if info, err := os.Stat(dir); err != nil || !info.IsDir() {
			t.Errorf("directory not created: %s", dir)
		}
	}
}

func TestInit_GivenAlreadyInitialized_WhenInitCalled_ThenError(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".memory")
	s := New(root)

	s.Init()
	err := s.Init()
	if err == nil {
		t.Error("expected error for already initialized store")
	}
	if !strings.Contains(err.Error(), "already initialized") {
		t.Errorf("error = %q, want 'already initialized'", err.Error())
	}
}

func TestPaths_GivenInitializedStore_WhenResolved_ThenCorrect(t *testing.T) {
	root := "/tmp/test-mem/.memory"
	s := New(root)

	if !strings.HasSuffix(s.EpisodesPath(), "episodes.jsonl") {
		t.Errorf("EpisodesPath = %q", s.EpisodesPath())
	}
	if !strings.HasSuffix(s.PrinciplesPath(), "principles.md") {
		t.Errorf("PrinciplesPath = %q", s.PrinciplesPath())
	}
	if !strings.HasSuffix(s.SkillsDir(), "skills") {
		t.Errorf("SkillsDir = %q", s.SkillsDir())
	}
}

func TestSessionCount_GivenNoFile_WhenRead_ThenReturns0(t *testing.T) {
	dir := t.TempDir()
	s := New(filepath.Join(dir, ".memory"))
	os.MkdirAll(s.Root, 0755)

	n, err := s.ReadSessionCount()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 0 {
		t.Errorf("got %d, want 0", n)
	}
}

func TestSessionCount_GivenCount7_WhenIncremented_ThenReturns8(t *testing.T) {
	dir := t.TempDir()
	s := New(filepath.Join(dir, ".memory"))
	os.MkdirAll(s.Root, 0755)
	os.WriteFile(s.SessionCountPath(), []byte("7"), 0644)

	if err := s.IncrementSessionCount(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	n, _ := s.ReadSessionCount()
	if n != 8 {
		t.Errorf("got %d, want 8", n)
	}
}

func TestSessionCount_GivenCount10_WhenReset_ThenReturns0(t *testing.T) {
	dir := t.TempDir()
	s := New(filepath.Join(dir, ".memory"))
	os.MkdirAll(s.Root, 0755)
	os.WriteFile(s.SessionCountPath(), []byte("10"), 0644)

	if err := s.ResetSessionCount(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	n, _ := s.ReadSessionCount()
	if n != 0 {
		t.Errorf("got %d, want 0", n)
	}
}
