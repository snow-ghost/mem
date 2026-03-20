package runner

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/snow-ghost/mem/internal/agent"
	"github.com/snow-ghost/mem/internal/config"
	"github.com/snow-ghost/mem/internal/consolidation"
	"github.com/snow-ghost/mem/internal/episode"
	"github.com/snow-ghost/mem/internal/filelock"
	"github.com/snow-ghost/mem/internal/principle"
	"github.com/snow-ghost/mem/internal/skill"
	"github.com/snow-ghost/mem/internal/store"
)

const consolidatePromptTemplate = `You are a memory consolidation agent. Analyze the accumulated episodes and extract reusable knowledge.

Tasks:
A) GROUP episodes by tags/themes. Find clusters of 3+ similar events.
B) EXTRACT PRINCIPLES from clusters: formulate concrete, verifiable action rules.
C) DETECT SKILLS: if 3+ episodes describe similar multi-step procedures, output full skill sections.
D) FLAG episodes for removal: duplicates, outdated, or fully covered by extracted principles (older than 30 days). Keep the 50 most recent untouched.
E) CHECK existing principles: flag any that contradict current episodes.

Output ONLY valid JSON:
{
  "new_principles": [{"topic": "...", "rule": "..."}],
  "episodes_to_remove": [0, 3, 7],
  "skill_candidates": [{"name": "...", "occurrences": 3, "triggers": ["..."], "prerequisites": ["..."], "steps": ["..."], "verification": ["..."], "antipatterns": ["..."]}]
}

--- All Episodes ---
%s

--- Current Principles ---
%s

--- Existing Skills ---
%s
`

type ConsolidateResult struct {
	EpisodesProcessed int
	PrinciplesAdded   int
	PrinciplesUpdated int
	PrinciplesRemoved int
	EpisodesRemoved   int
	SkillsCreated     int
	SkillCandidates   []string
	Conflicts         []string
	Skipped           bool
}

type consolidateResponse struct {
	NewPrinciples   []struct{ Topic, Rule string } `json:"new_principles"`
	EpisodesToRemove []int                          `json:"episodes_to_remove"`
	SkillCandidates []struct {
		Name          string   `json:"name"`
		Occurrences   int      `json:"occurrences"`
		Triggers      []string `json:"triggers"`
		Prerequisites []string `json:"prerequisites"`
		Steps         []string `json:"steps"`
		Verification  []string `json:"verification"`
		Antipatterns  []string `json:"antipatterns"`
	} `json:"skill_candidates"`
}

func RunConsolidate(cfg config.Config, s *store.MemoryStore, inv agent.Invoker, model string, dryRun, force bool) (*ConsolidateResult, error) {
	if !force {
		sessCount, _ := s.ReadSessionCount()
		epCount, _ := episode.Count(s.EpisodesPath())
		if sessCount < cfg.SessionThreshold && epCount < cfg.EpisodeThreshold {
			return &ConsolidateResult{Skipped: true}, nil
		}
	}

	allEpisodes, err := episode.ReadAll(s.EpisodesPath())
	if err != nil {
		return nil, fmt.Errorf("consolidate: read episodes: %w", err)
	}

	princ, err := principle.Parse(s.PrinciplesPath())
	if err != nil {
		return nil, fmt.Errorf("consolidate: read principles: %w", err)
	}

	skills, _ := skill.List(s.SkillsDir())

	epJSON, _ := json.Marshal(allEpisodes)
	princText := formatPrinciples(princ)
	skillsText := strings.Join(skills, ", ")

	prompt := fmt.Sprintf(consolidatePromptTemplate, string(epJSON), princText, skillsText)

	response, err := inv.Invoke(model, prompt)
	if err != nil {
		return nil, fmt.Errorf("consolidate: invoke agent: %w", err)
	}

	parsed, err := parseConsolidateResponse(response)
	if err != nil {
		return nil, fmt.Errorf("consolidate: parse response: %w", err)
	}

	result := &ConsolidateResult{
		EpisodesProcessed: len(allEpisodes),
	}

	incoming := make(principle.Principles)
	for _, p := range parsed.NewPrinciples {
		incoming[p.Topic] = append(incoming[p.Topic], p.Rule)
	}
	result.PrinciplesAdded = principle.Count(incoming)

	merged := principle.Merge(princ, incoming)
	merged = principle.Dedup(merged)
	merged = principle.EnforceLimit(merged, cfg.PrinciplesMax)

	remaining := removeEpisodes(allEpisodes, parsed.EpisodesToRemove, cfg.EpisodesKeep)
	if len(remaining) > cfg.EpisodesMax {
		remaining = remaining[len(remaining)-cfg.EpisodesMax:]
	}
	result.EpisodesRemoved = len(allEpisodes) - len(remaining)

	result.Conflicts = detectConflicts(allEpisodes)

	staleSkills := detectStaleSkills(s, 6)
	for _, staleName := range staleSkills {
		result.SkillCandidates = append(result.SkillCandidates,
			fmt.Sprintf("STALE: %s (review recommended)", staleName))
	}

	for _, sc := range parsed.SkillCandidates {
		if sc.Occurrences >= 3 && len(sc.Steps) > 0 {
			result.SkillsCreated++
			if !dryRun {
				slug := skill.Slugify(sc.Name)
				sk := skill.Skill{
					Name:          sc.Name,
					Slug:          slug,
					Triggers:      sc.Triggers,
					Prerequisites: sc.Prerequisites,
					Steps:         sc.Steps,
					Verification:  sc.Verification,
					Antipatterns:  sc.Antipatterns,
					CreatedAt:     time.Now().UTC().Format("2006-01-02"),
				}
				skill.Write(filepath.Join(s.SkillsDir(), slug+".md"), sk)
			}
		} else {
			result.SkillCandidates = append(result.SkillCandidates,
				fmt.Sprintf("%s (%d occurrences)", sc.Name, sc.Occurrences))
		}
	}

	if !dryRun {
		if err := principle.Write(s.PrinciplesPath(), merged); err != nil {
			return nil, fmt.Errorf("consolidate: write principles: %w", err)
		}

		if err := atomicWriteEpisodes(s.EpisodesPath(), s.LockPath(), remaining); err != nil {
			return nil, fmt.Errorf("consolidate: write episodes: %w", err)
		}

		lastEntry, _ := consolidation.ReadLast(s.ConsolidationLogPath())
		entry := consolidation.LogEntry{
			Date:              time.Now().UTC().Format("2006-01-02"),
			Number:            lastEntry.Number + 1,
			EpisodesProcessed: result.EpisodesProcessed,
			PrinciplesAdded:   result.PrinciplesAdded,
			PrinciplesUpdated: result.PrinciplesUpdated,
			PrinciplesRemoved: result.PrinciplesRemoved,
			EpisodesRemoved:   result.EpisodesRemoved,
			SkillsCreated:     result.SkillsCreated,
			SkillCandidates:   result.SkillCandidates,
			Conflicts:         result.Conflicts,
		}
		if err := consolidation.Append(s.ConsolidationLogPath(), entry); err != nil {
			return nil, fmt.Errorf("consolidate: write log: %w", err)
		}

		if err := s.ResetSessionCount(); err != nil {
			return nil, fmt.Errorf("consolidate: reset session count: %w", err)
		}
	}

	return result, nil
}

func parseConsolidateResponse(response string) (*consolidateResponse, error) {
	response = strings.TrimSpace(response)
	start := strings.Index(response, "{")
	end := strings.LastIndex(response, "}")
	if start == -1 || end == -1 || end <= start {
		return nil, fmt.Errorf("no JSON object found in response")
	}

	var parsed consolidateResponse
	if err := json.Unmarshal([]byte(response[start:end+1]), &parsed); err != nil {
		return nil, fmt.Errorf("unmarshal consolidation response: %w", err)
	}
	return &parsed, nil
}

func removeEpisodes(all []episode.Episode, indices []int, keepRecent int) []episode.Episode {
	removeSet := make(map[int]bool)
	protected := len(all) - keepRecent
	for _, idx := range indices {
		if idx >= 0 && idx < len(all) && idx < protected {
			removeSet[idx] = true
		}
	}

	var result []episode.Episode
	for i, ep := range all {
		if !removeSet[i] {
			result = append(result, ep)
		}
	}
	return result
}

func detectConflicts(episodes []episode.Episode) []string {
	type decisionKey struct {
		tag     string
		agentID string
	}
	decisions := make(map[string][]episode.Episode)
	for _, ep := range episodes {
		if ep.Type != "decision" || ep.AgentID == "" {
			continue
		}
		for _, tag := range ep.Tags {
			key := strings.ToLower(tag)
			decisions[key] = append(decisions[key], ep)
		}
	}

	var conflicts []string
	for tag, eps := range decisions {
		agents := make(map[string]episode.Episode)
		for _, ep := range eps {
			if existing, ok := agents[ep.AgentID]; ok {
				_ = existing
			} else {
				agents[ep.AgentID] = ep
			}
		}
		if len(agents) > 1 {
			var parts []string
			for agentID, ep := range agents {
				parts = append(parts, fmt.Sprintf("[%s] %s", agentID, ep.Summary))
			}
			conflicts = append(conflicts,
				fmt.Sprintf("tag=%q: %s", tag, strings.Join(parts, " vs ")))
		}
	}
	return conflicts
}

func detectStaleSkills(s *store.MemoryStore, monthsThreshold int) []string {
	slugs, err := skill.List(s.SkillsDir())
	if err != nil {
		return nil
	}
	cutoff := time.Now().AddDate(0, -monthsThreshold, 0)
	var stale []string
	for _, slug := range slugs {
		sk, err := skill.Parse(filepath.Join(s.SkillsDir(), slug+".md"))
		if err != nil {
			continue
		}
		if sk.CreatedAt != "" {
			created, err := time.Parse("2006-01-02", sk.CreatedAt)
			if err == nil && created.Before(cutoff) {
				stale = append(stale, sk.Name)
			}
		}
	}
	return stale
}

func atomicWriteEpisodes(path, lockPath string, episodes []episode.Episode) error {
	return filelock.WithLock(lockPath, func() error {
		tmpPath := path + ".tmp"
		f, err := os.Create(tmpPath)
		if err != nil {
			return fmt.Errorf("create temp file: %w", err)
		}

		enc := json.NewEncoder(f)
		for _, ep := range episodes {
			if err := enc.Encode(ep); err != nil {
				f.Close()
				os.Remove(tmpPath)
				return fmt.Errorf("encode episode: %w", err)
			}
		}
		f.Close()

		return os.Rename(tmpPath, path)
	})
}
