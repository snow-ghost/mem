package consolidation

import (
	"fmt"
	"os"
	"strings"
)

type LogEntry struct {
	Date              string
	Number            int
	EpisodesProcessed int
	PrinciplesAdded   int
	PrinciplesUpdated int
	PrinciplesRemoved int
	EpisodesRemoved   int
	SkillsCreated     int
	SkillCandidates   []string
	Conflicts         []string
}

func Append(path string, entry LogEntry) error {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n## %s — Consolidation #%d\n", entry.Date, entry.Number))
	sb.WriteString(fmt.Sprintf("- Episodes processed: %d\n", entry.EpisodesProcessed))
	sb.WriteString(fmt.Sprintf("- Principles added: %d\n", entry.PrinciplesAdded))
	sb.WriteString(fmt.Sprintf("- Principles updated: %d\n", entry.PrinciplesUpdated))
	sb.WriteString(fmt.Sprintf("- Principles removed: %d\n", entry.PrinciplesRemoved))
	sb.WriteString(fmt.Sprintf("- Episodes removed: %d\n", entry.EpisodesRemoved))
	sb.WriteString(fmt.Sprintf("- Skills created: %d\n", entry.SkillsCreated))
	if len(entry.SkillCandidates) > 0 {
		sb.WriteString(fmt.Sprintf("- Skill candidates: %s\n", strings.Join(entry.SkillCandidates, ", ")))
	}
	if len(entry.Conflicts) > 0 {
		sb.WriteString("- Conflicts detected:\n")
		for _, c := range entry.Conflicts {
			sb.WriteString("  - " + c + "\n")
		}
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("open consolidation log: %w", err)
	}
	defer f.Close()

	if _, err := f.WriteString(sb.String()); err != nil {
		return fmt.Errorf("write consolidation log: %w", err)
	}
	return nil
}

func ReadLast(path string) (LogEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return LogEntry{}, nil
		}
		return LogEntry{}, fmt.Errorf("read consolidation log: %w", err)
	}

	content := string(data)
	sections := strings.Split(content, "## ")
	if len(sections) < 2 {
		return LogEntry{}, nil
	}

	last := sections[len(sections)-1]
	var entry LogEntry

	lines := strings.Split(last, "\n")
	if len(lines) > 0 {
		header := lines[0]
		parts := strings.SplitN(header, " — Consolidation #", 2)
		if len(parts) == 2 {
			entry.Date = strings.TrimSpace(parts[0])
			fmt.Sscanf(parts[1], "%d", &entry.Number)
		}
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		fmt.Sscanf(line, "- Episodes processed: %d", &entry.EpisodesProcessed)
		fmt.Sscanf(line, "- Principles added: %d", &entry.PrinciplesAdded)
		fmt.Sscanf(line, "- Principles updated: %d", &entry.PrinciplesUpdated)
		fmt.Sscanf(line, "- Principles removed: %d", &entry.PrinciplesRemoved)
		fmt.Sscanf(line, "- Episodes removed: %d", &entry.EpisodesRemoved)
		fmt.Sscanf(line, "- Skills created: %d", &entry.SkillsCreated)
	}

	return entry, nil
}
