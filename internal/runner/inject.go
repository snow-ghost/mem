package runner

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/snow-ghost/mem/internal/config"
	"github.com/snow-ghost/mem/internal/episode"
	"github.com/snow-ghost/mem/internal/principle"
	"github.com/snow-ghost/mem/internal/skill"
	"github.com/snow-ghost/mem/internal/store"
)

type InjectContext struct {
	Principles principle.Principles
	Episodes   []episode.Episode
	Skills     []skill.Skill
	AllSkills  []skill.Skill
}

func RunInject(cfg config.Config, s *store.MemoryStore, episodeCount int) (*InjectContext, error) {
	princ, err := principle.Parse(s.PrinciplesPath())
	if err != nil {
		return nil, fmt.Errorf("inject: read principles: %w", err)
	}

	recent, err := episode.ReadLast(s.EpisodesPath(), episodeCount)
	if err != nil {
		return nil, fmt.Errorf("inject: read episodes: %w", err)
	}

	slugs, err := skill.List(s.SkillsDir())
	if err != nil {
		return nil, fmt.Errorf("inject: list skills: %w", err)
	}

	var allSkills []skill.Skill
	for _, slug := range slugs {
		sk, err := skill.Parse(filepath.Join(s.SkillsDir(), slug+".md"))
		if err != nil {
			continue
		}
		allSkills = append(allSkills, sk)
	}

	var recentTags []string
	for _, ep := range recent {
		recentTags = append(recentTags, ep.Tags...)
	}

	matched := skill.MatchSkills(allSkills, recentTags)
	if len(matched) == 0 {
		matched = allSkills
	}

	return &InjectContext{
		Principles: princ,
		Episodes:   recent,
		Skills:     matched,
		AllSkills:  allSkills,
	}, nil
}

func FormatMarkdown(ctx *InjectContext) string {
	var sb strings.Builder
	sb.WriteString("# Project Memory\n")

	if principle.Count(ctx.Principles) > 0 {
		sb.WriteString("\n## Principles\n")
		for topic, rules := range ctx.Principles {
			sb.WriteString(fmt.Sprintf("### %s\n", topic))
			for _, r := range rules {
				sb.WriteString("- " + r + "\n")
			}
		}
	}

	if len(ctx.Episodes) > 0 {
		sb.WriteString("\n## Recent Events\n")
		for _, ep := range ctx.Episodes {
			ts := ep.Ts
			if len(ts) >= 10 {
				ts = ts[:10]
			}
			sb.WriteString(fmt.Sprintf("- [%s] [%s] %s\n", ts, ep.Type, ep.Summary))
		}
	}

	if len(ctx.Skills) > 0 {
		sb.WriteString("\n## Relevant Skills\n")
		for _, sk := range ctx.Skills {
			sb.WriteString(fmt.Sprintf("- **%s**: %s\n", sk.Name, strings.Join(sk.Triggers, ", ")))
		}
	}

	return sb.String()
}

func FormatJSON(ctx *InjectContext) (string, error) {
	out := map[string]any{
		"principles": ctx.Principles,
		"episodes":   ctx.Episodes,
		"skills":     ctx.Skills,
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal inject context: %w", err)
	}
	return string(data), nil
}
