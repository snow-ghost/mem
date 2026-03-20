package principle

import (
	"fmt"
	"os"
	"strings"
)

type Principles map[string][]string

func Parse(path string) (Principles, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(Principles), nil
		}
		return nil, fmt.Errorf("read principles: %w", err)
	}

	p := make(Principles)
	var currentTopic string
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") {
			currentTopic = strings.TrimPrefix(trimmed, "## ")
		} else if strings.HasPrefix(trimmed, "- ") && currentTopic != "" {
			rule := strings.TrimPrefix(trimmed, "- ")
			p[currentTopic] = append(p[currentTopic], rule)
		}
	}
	return p, nil
}

func Write(path string, p Principles) error {
	var sb strings.Builder
	sb.WriteString("# Project Principles\n")

	for topic, rules := range p {
		sb.WriteString("\n## " + topic + "\n")
		for _, rule := range rules {
			sb.WriteString("- " + rule + "\n")
		}
	}

	if err := os.WriteFile(path, []byte(sb.String()), 0644); err != nil {
		return fmt.Errorf("write principles: %w", err)
	}
	return nil
}

func Count(p Principles) int {
	total := 0
	for _, rules := range p {
		total += len(rules)
	}
	return total
}
