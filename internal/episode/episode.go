package episode

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/snow-ghost/mem/internal/filelock"
)

var validTypes = map[string]bool{
	"decision": true,
	"error":    true,
	"pattern":  true,
	"insight":  true,
	"rollback": true,
}

type Episode struct {
	Ts      string   `json:"ts"`
	Session string   `json:"session"`
	Type    string   `json:"type"`
	Summary string   `json:"summary"`
	Tags    []string `json:"tags"`
	AgentID string   `json:"agent_id,omitempty"`
}

func (e Episode) Validate() error {
	if !validTypes[e.Type] {
		return fmt.Errorf("invalid episode type: %q", e.Type)
	}
	s := strings.TrimSpace(e.Summary)
	if s == "" {
		return fmt.Errorf("summary must not be empty")
	}
	if len(s) > 500 {
		return fmt.Errorf("summary exceeds 500 characters: %d", len(s))
	}
	if len(e.Tags) < 1 || len(e.Tags) > 3 {
		return fmt.Errorf("tags must contain 1-3 entries, got %d", len(e.Tags))
	}
	for i, tag := range e.Tags {
		if strings.TrimSpace(tag) == "" {
			return fmt.Errorf("tag[%d] must not be empty", i)
		}
	}
	if _, err := time.Parse(time.RFC3339, e.Ts); err != nil {
		return fmt.Errorf("invalid ISO 8601 timestamp: %w", err)
	}
	return nil
}

func ReadAll(path string) ([]Episode, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open episodes: %w", err)
	}
	defer f.Close()

	var episodes []Episode
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 64*1024)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var ep Episode
		if err := json.Unmarshal([]byte(line), &ep); err != nil {
			slog.Warn("skipping corrupt episode line", "line", lineNum, "error", err)
			continue
		}
		episodes = append(episodes, ep)
	}
	if err := scanner.Err(); err != nil {
		return episodes, fmt.Errorf("scan episodes: %w", err)
	}
	return episodes, nil
}

func Append(path, lockPath string, ep Episode) error {
	return filelock.WithLock(lockPath, func() error {
		data, err := json.Marshal(ep)
		if err != nil {
			return fmt.Errorf("marshal episode: %w", err)
		}
		data = append(data, '\n')

		f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
		if err != nil {
			return fmt.Errorf("open episodes for append: %w", err)
		}
		defer f.Close()

		if _, err := f.Write(data); err != nil {
			return fmt.Errorf("write episode: %w", err)
		}
		return nil
	})
}

func ReadLast(path string, n int) ([]Episode, error) {
	all, err := ReadAll(path)
	if err != nil {
		return nil, err
	}
	if len(all) <= n {
		return all, nil
	}
	return all[len(all)-n:], nil
}

func Count(path string) (int, error) {
	all, err := ReadAll(path)
	if err != nil {
		return 0, err
	}
	return len(all), nil
}
