package skill

import "testing"

func TestMatchSkills_GivenMatchingTrigger_WhenMatched_ThenIncluded(t *testing.T) {
	skills := []Skill{
		{Name: "DB Migration", Triggers: []string{"database migration"}},
	}
	tags := []string{"migration", "schema"}

	matched := MatchSkills(skills, tags)
	if len(matched) != 1 {
		t.Fatalf("got %d matched, want 1", len(matched))
	}
	if matched[0].Name != "DB Migration" {
		t.Errorf("matched = %q, want %q", matched[0].Name, "DB Migration")
	}
}

func TestMatchSkills_GivenNoMatchingTrigger_WhenMatched_ThenNotIncluded(t *testing.T) {
	skills := []Skill{
		{Name: "Deploy", Triggers: []string{"deploy staging"}},
	}
	tags := []string{"migration", "schema"}

	matched := MatchSkills(skills, tags)
	if len(matched) != 0 {
		t.Errorf("got %d matched, want 0", len(matched))
	}
}

func TestMatchSkills_GivenNoSkillsMatch_WhenMatched_ThenEmptyList(t *testing.T) {
	skills := []Skill{
		{Name: "A", Triggers: []string{"x y z"}},
	}
	tags := []string{"a", "b"}

	matched := MatchSkills(skills, tags)
	if matched != nil {
		t.Errorf("got %v, want nil", matched)
	}
}
