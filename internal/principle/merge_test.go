package principle

import "testing"

func TestMerge_GivenOverlappingTopics_WhenMerged_ThenCombinedWithoutDuplicates(t *testing.T) {
	existing := Principles{"Arch": {"rule1", "rule2"}}
	incoming := Principles{"Arch": {"rule2", "rule3"}, "Test": {"rule4"}}

	result := Merge(existing, incoming)
	if len(result["Arch"]) != 3 {
		t.Errorf("Arch rules = %d, want 3", len(result["Arch"]))
	}
	if len(result["Test"]) != 1 {
		t.Errorf("Test rules = %d, want 1", len(result["Test"]))
	}
}

func TestEnforceLimit_Given105Principles_WhenLimitedTo100_Then100Remain(t *testing.T) {
	p := make(Principles)
	for i := range 105 {
		topic := "Topic"
		if i >= 50 {
			topic = "Topic2"
		}
		p[topic] = append(p[topic], "rule")
	}

	result := EnforceLimit(p, 100)
	if got := Count(result); got > 100 {
		t.Errorf("Count = %d, want <= 100", got)
	}
}

func TestDedup_GivenDuplicateRules_WhenDeduped_ThenOneCopyRemains(t *testing.T) {
	p := Principles{
		"Arch": {"Use JSONL", "use jsonl", "Keep files small"},
	}

	result := Dedup(p)
	if len(result["Arch"]) != 2 {
		t.Errorf("Arch rules = %d, want 2 (after dedup)", len(result["Arch"]))
	}
}
