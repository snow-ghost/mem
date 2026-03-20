package skill

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type Skill struct {
	Name          string
	Slug          string
	Triggers      []string
	Prerequisites []string
	Steps         []string
	Verification  []string
	Antipatterns  []string
	CreatedAt     string
}

func Parse(path string) (Skill, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Skill{}, fmt.Errorf("read skill: %w", err)
	}

	var s Skill
	s.Slug = strings.TrimSuffix(filepath.Base(path), ".md")
	var currentSection string

	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") && !strings.HasPrefix(trimmed, "## ") {
			s.Name = strings.TrimPrefix(trimmed, "# ")
		} else if strings.HasPrefix(trimmed, "## ") {
			currentSection = strings.ToLower(strings.TrimPrefix(trimmed, "## "))
		} else if strings.HasPrefix(trimmed, "- ") || (len(trimmed) > 2 && trimmed[1] == '.' && trimmed[0] >= '0' && trimmed[0] <= '9') {
			item := trimmed
			if strings.HasPrefix(item, "- ") {
				item = strings.TrimPrefix(item, "- ")
			} else {
				// numbered list: "1. step"
				parts := strings.SplitN(item, ". ", 2)
				if len(parts) == 2 {
					item = parts[1]
				}
			}
			switch {
			case strings.Contains(currentSection, "when to apply"):
				s.Triggers = append(s.Triggers, item)
			case strings.Contains(currentSection, "prerequisit"):
				s.Prerequisites = append(s.Prerequisites, item)
			case strings.Contains(currentSection, "step"):
				s.Steps = append(s.Steps, item)
			case strings.Contains(currentSection, "verification") || strings.Contains(currentSection, "success"):
				s.Verification = append(s.Verification, item)
			case strings.Contains(currentSection, "anti-pattern") || strings.Contains(currentSection, "antipattern"):
				s.Antipatterns = append(s.Antipatterns, item)
			}
		}
	}
	return s, nil
}

func Write(path string, s Skill) error {
	var sb strings.Builder
	sb.WriteString("# " + s.Name + "\n")

	sb.WriteString("\n## When to apply\n")
	for _, t := range s.Triggers {
		sb.WriteString("- " + t + "\n")
	}

	sb.WriteString("\n## Prerequisites\n")
	for _, p := range s.Prerequisites {
		sb.WriteString("- " + p + "\n")
	}

	sb.WriteString("\n## Steps\n")
	for i, step := range s.Steps {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, step))
	}

	sb.WriteString("\n## Success verification\n")
	for _, v := range s.Verification {
		sb.WriteString("- " + v + "\n")
	}

	sb.WriteString("\n## Anti-patterns\n")
	for _, a := range s.Antipatterns {
		sb.WriteString("- " + a + "\n")
	}

	if err := os.WriteFile(path, []byte(sb.String()), 0644); err != nil {
		return fmt.Errorf("write skill: %w", err)
	}
	return nil
}

func List(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("list skills: %w", err)
	}

	var slugs []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			slugs = append(slugs, strings.TrimSuffix(e.Name(), ".md"))
		}
	}
	return slugs, nil
}

var nonAlphanumeric = regexp.MustCompile(`[^a-z0-9-]+`)
var multiHyphen = regexp.MustCompile(`-{2,}`)

func Slugify(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = strings.ReplaceAll(s, " ", "-")
	s = nonAlphanumeric.ReplaceAllString(s, "")
	s = multiHyphen.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}
