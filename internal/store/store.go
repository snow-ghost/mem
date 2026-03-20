package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const DefaultExtractPrompt = `You are a memory extraction agent. Identify significant events from the session diff.
Output ONLY a JSON array: [{"type":"decision|error|pattern|insight|rollback","summary":"one sentence","tags":["1-3 keywords"]}]
If no significant events, output [].`

const DefaultConsolidatePrompt = `You are a consolidation agent. Analyze episodes, extract principles, detect skills, flag duplicates.
Output ONLY JSON: {"new_principles":[{"topic":"...","rule":"..."}],"episodes_to_remove":[indices],"skill_candidates":[{"name":"...","occurrences":N,"triggers":[],"prerequisites":[],"steps":[],"verification":[],"antipatterns":[]}]}`

type MemoryStore struct {
	Root string
}

func New(root string) *MemoryStore {
	return &MemoryStore{Root: root}
}

func (s *MemoryStore) Init() error {
	if _, err := os.Stat(s.EpisodesPath()); err == nil {
		return fmt.Errorf("memory store already initialized at %s", s.Root)
	}

	dirs := []string{s.Root, s.SkillsDir(), s.PromptsDir()}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return fmt.Errorf("create directory %s: %w", d, err)
		}
	}

	files := map[string]string{
		s.EpisodesPath():            "",
		s.PrinciplesPath():          "# Project Principles\n",
		s.ConsolidationLogPath():    "# Consolidation Log\n",
		s.ExtractPromptPath():       DefaultExtractPrompt,
		s.ConsolidatePromptPath():   DefaultConsolidatePrompt,
	}
	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return fmt.Errorf("create file %s: %w", path, err)
		}
	}

	return nil
}

// EnsureInit creates the store if it doesn't exist, or creates only
// missing files if the root dir exists. Never overwrites existing files.
// Returns (true, nil) if the store was freshly created.
func (s *MemoryStore) EnsureInit() (bool, error) {
	created := false
	if _, err := os.Stat(s.Root); os.IsNotExist(err) {
		created = true
	}

	dirs := []string{s.Root, s.SkillsDir(), s.PromptsDir()}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return false, fmt.Errorf("create directory %s: %w", d, err)
		}
	}

	defaults := map[string]string{
		s.EpisodesPath():          "",
		s.PrinciplesPath():        "# Project Principles\n",
		s.ConsolidationLogPath():  "# Consolidation Log\n",
		s.ExtractPromptPath():     DefaultExtractPrompt,
		s.ConsolidatePromptPath(): DefaultConsolidatePrompt,
	}
	for path, content := range defaults {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			if err := os.WriteFile(path, []byte(content), 0644); err != nil {
				return false, fmt.Errorf("create file %s: %w", path, err)
			}
		}
	}

	return created, nil
}

func (s *MemoryStore) EpisodesPath() string          { return filepath.Join(s.Root, "episodes.jsonl") }
func (s *MemoryStore) PrinciplesPath() string         { return filepath.Join(s.Root, "principles.md") }
func (s *MemoryStore) ConsolidationLogPath() string   { return filepath.Join(s.Root, "consolidation-log.md") }
func (s *MemoryStore) SkillsDir() string              { return filepath.Join(s.Root, "skills") }
func (s *MemoryStore) PromptsDir() string             { return filepath.Join(s.Root, "prompts") }
func (s *MemoryStore) LockPath() string               { return filepath.Join(s.Root, ".lock") }
func (s *MemoryStore) SessionCountPath() string       { return filepath.Join(s.Root, ".session-count") }

func (s *MemoryStore) ReadSessionCount() (int, error) {
	data, err := os.ReadFile(s.SessionCountPath())
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("read session count: %w", err)
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, nil
	}
	return n, nil
}

func (s *MemoryStore) IncrementSessionCount() error {
	n, err := s.ReadSessionCount()
	if err != nil {
		return err
	}
	return os.WriteFile(s.SessionCountPath(), []byte(strconv.Itoa(n+1)), 0644)
}

func (s *MemoryStore) ResetSessionCount() error {
	return os.WriteFile(s.SessionCountPath(), []byte("0"), 0644)
}

func (s *MemoryStore) ExtractPromptPath() string    { return filepath.Join(s.PromptsDir(), "extract.md") }
func (s *MemoryStore) ConsolidatePromptPath() string { return filepath.Join(s.PromptsDir(), "consolidate.md") }

func (s *MemoryStore) ReadPrompt(path, fallback string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return fallback
	}
	content := strings.TrimSpace(string(data))
	if content == "" {
		return fallback
	}
	return content
}

func (s *MemoryStore) StoreSize() (int64, error) {
	var total int64
	err := filepath.Walk(s.Root, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total, err
}
