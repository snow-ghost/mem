package miner

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Exchange struct {
	Speaker   string
	Content   string
	Timestamp string
}

func ParseConversation(path string) ([]Exchange, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read conversation: %w", err)
	}

	ext := strings.ToLower(filepath.Ext(path))
	content := string(data)

	switch {
	case ext == ".jsonl":
		return parseClaudeJSONL(content)
	case ext == ".json" && strings.Contains(content, "mapping"):
		return parseChatGPTJSON(data)
	case ext == ".json" && strings.Contains(content, "\"messages\""):
		return parseSlackJSON(data)
	default:
		return parsePlainText(content)
	}
}

func parseClaudeJSONL(content string) ([]Exchange, error) {
	var exchanges []Exchange
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var msg map[string]any
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}
		role, _ := msg["role"].(string)
		text, _ := msg["content"].(string)
		if text == "" {
			if parts, ok := msg["content"].([]any); ok {
				for _, p := range parts {
					if pm, ok := p.(map[string]any); ok {
						if t, ok := pm["text"].(string); ok {
							text += t + " "
						}
					}
				}
			}
		}
		if role != "" && text != "" {
			exchanges = append(exchanges, Exchange{
				Speaker: role,
				Content: strings.TrimSpace(text),
			})
		}
	}
	return exchanges, nil
}

func parseChatGPTJSON(data []byte) ([]Exchange, error) {
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	var exchanges []Exchange
	if mapping, ok := raw["mapping"].(map[string]any); ok {
		for _, v := range mapping {
			node, ok := v.(map[string]any)
			if !ok {
				continue
			}
			msg, ok := node["message"].(map[string]any)
			if !ok {
				continue
			}
			role, _ := msg["author"].(map[string]any)["role"].(string)
			content, _ := msg["content"].(map[string]any)
			parts, _ := content["parts"].([]any)
			var text string
			for _, p := range parts {
				if s, ok := p.(string); ok {
					text += s
				}
			}
			if role != "" && text != "" {
				exchanges = append(exchanges, Exchange{Speaker: role, Content: text})
			}
		}
	}
	return exchanges, nil
}

func parseSlackJSON(data []byte) ([]Exchange, error) {
	var messages []map[string]any
	if err := json.Unmarshal(data, &messages); err != nil {
		var wrapper map[string]any
		if err := json.Unmarshal(data, &wrapper); err != nil {
			return nil, err
		}
		if msgs, ok := wrapper["messages"].([]any); ok {
			for _, m := range msgs {
				if mm, ok := m.(map[string]any); ok {
					messages = append(messages, mm)
				}
			}
		}
	}

	var exchanges []Exchange
	for _, msg := range messages {
		user, _ := msg["user"].(string)
		text, _ := msg["text"].(string)
		ts, _ := msg["ts"].(string)
		if text != "" {
			if user == "" {
				user = "unknown"
			}
			exchanges = append(exchanges, Exchange{Speaker: user, Content: text, Timestamp: ts})
		}
	}
	return exchanges, nil
}

func parsePlainText(content string) ([]Exchange, error) {
	var exchanges []Exchange
	var current *Exchange

	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "Human:") || strings.HasPrefix(trimmed, "User:") {
			if current != nil {
				exchanges = append(exchanges, *current)
			}
			text := strings.TrimPrefix(strings.TrimPrefix(trimmed, "Human:"), "User:")
			current = &Exchange{Speaker: "human", Content: strings.TrimSpace(text)}
		} else if strings.HasPrefix(trimmed, "Assistant:") || strings.HasPrefix(trimmed, "AI:") {
			if current != nil {
				exchanges = append(exchanges, *current)
			}
			text := strings.TrimPrefix(strings.TrimPrefix(trimmed, "Assistant:"), "AI:")
			current = &Exchange{Speaker: "assistant", Content: strings.TrimSpace(text)}
		} else if current != nil && trimmed != "" {
			current.Content += "\n" + trimmed
		}
	}
	if current != nil {
		exchanges = append(exchanges, *current)
	}
	return exchanges, nil
}
