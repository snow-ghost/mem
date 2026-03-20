package episode

import "testing"

func TestIsDuplicate_GivenMatchingTypeSummary_WhenChecked_ThenTrue(t *testing.T) {
	existing := []Episode{{Type: "decision", Summary: "chose JSONL"}}
	ep := Episode{Type: "decision", Summary: "chose JSONL"}
	if !IsDuplicate(ep, existing) {
		t.Error("expected duplicate")
	}
}

func TestIsDuplicate_GivenDifferentType_WhenChecked_ThenFalse(t *testing.T) {
	existing := []Episode{{Type: "decision", Summary: "chose JSONL"}}
	ep := Episode{Type: "error", Summary: "chose JSONL"}
	if IsDuplicate(ep, existing) {
		t.Error("expected not duplicate (different type)")
	}
}

func TestIsDuplicate_GivenCaseInsensitive_WhenChecked_ThenTrue(t *testing.T) {
	existing := []Episode{{Type: "decision", Summary: "Chose JSONL"}}
	ep := Episode{Type: "decision", Summary: "chose jsonl"}
	if !IsDuplicate(ep, existing) {
		t.Error("expected duplicate (case-insensitive)")
	}
}

func TestIsDuplicate_GivenEmptyStore_WhenChecked_ThenFalse(t *testing.T) {
	ep := Episode{Type: "decision", Summary: "new event"}
	if IsDuplicate(ep, nil) {
		t.Error("expected not duplicate for empty store")
	}
}
