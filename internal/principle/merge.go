package principle

import "strings"

func Merge(existing, incoming Principles) Principles {
	result := make(Principles)
	for topic, rules := range existing {
		result[topic] = append(result[topic], rules...)
	}
	for topic, rules := range incoming {
		for _, rule := range rules {
			if !containsRule(result[topic], rule) {
				result[topic] = append(result[topic], rule)
			}
		}
	}
	return result
}

func EnforceLimit(p Principles, max int) Principles {
	total := Count(p)
	if total <= max {
		return p
	}

	remove := total - max
	result := make(Principles)
	for topic, rules := range p {
		if remove <= 0 {
			result[topic] = rules
			continue
		}
		if len(rules) <= remove {
			remove -= len(rules)
			continue
		}
		result[topic] = rules[remove:]
		remove = 0
	}
	return result
}

func Dedup(p Principles) Principles {
	result := make(Principles)
	for topic, rules := range p {
		seen := make(map[string]bool)
		for _, rule := range rules {
			key := strings.ToLower(strings.TrimSpace(rule))
			if !seen[key] {
				seen[key] = true
				result[topic] = append(result[topic], rule)
			}
		}
	}
	return result
}

func containsRule(rules []string, rule string) bool {
	key := strings.ToLower(strings.TrimSpace(rule))
	for _, r := range rules {
		if strings.ToLower(strings.TrimSpace(r)) == key {
			return true
		}
	}
	return false
}
