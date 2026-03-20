package runner

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/snow-ghost/mem/internal/agent"
	"github.com/snow-ghost/mem/internal/config"
	"github.com/snow-ghost/mem/internal/episode"
	"github.com/snow-ghost/mem/internal/principle"
	"github.com/snow-ghost/mem/internal/store"
)

const extractPromptTemplate = `You are a memory extraction agent. Your task is to identify significant events from a completed coding session.

Analyze the following git diff and context, then identify significant events:
- Architectural decisions (why X was chosen over Y)
- Discovered bugs and their root causes
- Recurring patterns (similar tasks done before)
- Insights (new learnings about the project/stack)
- Rollbacks (things tried and reverted, with reasons)

DO NOT record:
- Routine operations (standard commits, passing test runs)
- Implementation details already captured in code
- Events already present in recent episodes

Output ONLY a JSON array of episode objects. If no significant events found, output an empty array [].
Format: [{"type":"decision|error|pattern|insight|rollback","summary":"one sentence","tags":["1-3 keywords"]}]

--- Recent Episodes (for dedup context) ---
%s

--- Current Principles ---
%s

--- Session Diff ---
%s
`

type ExtractResult struct {
	Episodes    []episode.Episode
	NewCount    int
	SessionCount int
	ThresholdReached bool
}

func RunExtract(cfg config.Config, s *store.MemoryStore, inv agent.Invoker, sessionID, model string, dryRun bool) (*ExtractResult, error) {
	diff, err := getGitDiff()
	if err != nil {
		diff = "(git diff unavailable)"
	}

	existing, err := episode.ReadLast(s.EpisodesPath(), 20)
	if err != nil {
		return nil, fmt.Errorf("extract: read recent episodes: %w", err)
	}

	princ, err := principle.Parse(s.PrinciplesPath())
	if err != nil {
		return nil, fmt.Errorf("extract: read principles: %w", err)
	}

	recentJSON, _ := json.Marshal(existing)
	princText := formatPrinciples(princ)
	prompt := fmt.Sprintf(extractPromptTemplate, string(recentJSON), princText, diff)

	response, err := inv.Invoke(model, prompt)
	if err != nil {
		return nil, fmt.Errorf("extract: invoke agent: %w", err)
	}

	parsed, err := parseExtractResponse(response)
	if err != nil {
		return nil, fmt.Errorf("extract: parse response: %w", err)
	}

	allExisting, _ := episode.ReadAll(s.EpisodesPath())
	ts := time.Now().UTC().Format(time.RFC3339)

	var newEpisodes []episode.Episode
	for _, ep := range parsed {
		ep.Ts = ts
		ep.Session = sessionID
		ep.AgentID = cfg.AgentID
		if episode.IsDuplicate(ep, allExisting) {
			continue
		}
		newEpisodes = append(newEpisodes, ep)
	}

	if !dryRun {
		for _, ep := range newEpisodes {
			if err := episode.Append(s.EpisodesPath(), s.LockPath(), ep); err != nil {
				return nil, fmt.Errorf("extract: append episode: %w", err)
			}
		}
		if err := s.IncrementSessionCount(); err != nil {
			return nil, fmt.Errorf("extract: increment session count: %w", err)
		}
	}

	sessCount, _ := s.ReadSessionCount()
	epCount, _ := episode.Count(s.EpisodesPath())

	return &ExtractResult{
		Episodes:         newEpisodes,
		NewCount:         len(newEpisodes),
		SessionCount:     sessCount,
		ThresholdReached: sessCount >= cfg.SessionThreshold || epCount >= cfg.EpisodeThreshold,
	}, nil
}

func parseExtractResponse(response string) ([]episode.Episode, error) {
	response = strings.TrimSpace(response)

	start := strings.Index(response, "[")
	end := strings.LastIndex(response, "]")
	if start == -1 || end == -1 || end <= start {
		return nil, fmt.Errorf("no JSON array found in response")
	}
	jsonStr := response[start : end+1]

	var episodes []episode.Episode
	if err := json.Unmarshal([]byte(jsonStr), &episodes); err != nil {
		return nil, fmt.Errorf("unmarshal episodes: %w", err)
	}
	return episodes, nil
}

func getGitDiff() (string, error) {
	cmd := exec.Command("git", "diff", "HEAD~1")
	out, err := cmd.Output()
	if err != nil {
		cmd2 := exec.Command("git", "diff", "--cached")
		out, err = cmd2.Output()
		if err != nil {
			return "", fmt.Errorf("git diff: %w", err)
		}
	}
	diff := string(out)
	if len(diff) > 10000 {
		diff = diff[:10000] + "\n... (truncated)"
	}
	return diff, nil
}

func formatPrinciples(p principle.Principles) string {
	var sb strings.Builder
	for topic, rules := range p {
		sb.WriteString("## " + topic + "\n")
		for _, r := range rules {
			sb.WriteString("- " + r + "\n")
		}
	}
	return sb.String()
}

func GetGitShortHash() string {
	out, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}
