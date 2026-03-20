package skill

import "strings"

func MatchSkills(skills []Skill, recentTags []string) []Skill {
	tagSet := make(map[string]bool)
	for _, t := range recentTags {
		tagSet[strings.ToLower(t)] = true
	}

	var matched []Skill
	for _, s := range skills {
		if matchesTriggers(s.Triggers, tagSet) {
			matched = append(matched, s)
		}
	}
	return matched
}

func matchesTriggers(triggers []string, tagSet map[string]bool) bool {
	for _, trigger := range triggers {
		words := strings.Fields(strings.ToLower(trigger))
		for _, w := range words {
			if tagSet[w] {
				return true
			}
		}
	}
	return false
}
