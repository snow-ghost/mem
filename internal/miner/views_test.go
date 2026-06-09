package miner

import (
	"strings"
	"testing"
)

func TestConversationViews_GivenMixedTurns_WhenBuilt_ThenAllThreeVariantsPresent(t *testing.T) {
	ex := []Exchange{
		{Speaker: "user", Content: "what time is it in tokyo?"},
		{Speaker: "assistant", Content: "it is 3pm JST"},
		{Speaker: "user", Content: "and in london?"},
		{Speaker: "assistant", Content: "it is 7am BST"},
		{Speaker: "user", Content: "thanks"},
		{Speaker: "user", Content: "can you set a reminder?"},
	}
	views := ConversationViews(ex)

	if views["L0"] == "" || views["L1"] == "" || views["L2"] == "" {
		t.Fatalf("expected all 3 halls populated: %v", views)
	}
	if !strings.Contains(views["L0"], "assistant: it is 3pm JST") {
		t.Errorf("L0 missing assistant turn: %q", views["L0"])
	}
	if strings.Contains(views["L1"], "assistant") {
		t.Errorf("L1 should not contain assistant: %q", views["L1"])
	}
	// L2 = first 3 user turns, so the 4th should be missing.
	if strings.Contains(views["L2"], "can you set a reminder") {
		t.Errorf("L2 should be limited to first 3 user turns: %q", views["L2"])
	}
}

func TestConversationViews_GivenHumanSpeakerLabel_WhenBuilt_ThenCountedAsUser(t *testing.T) {
	ex := []Exchange{
		{Speaker: "Human", Content: "hi"},
		{Speaker: "assistant", Content: "hello"},
	}
	views := ConversationViews(ex)
	if !strings.Contains(views["L1"], "hi") {
		t.Errorf("L1 missing Human turn: %q", views["L1"])
	}
}

func TestConversationViews_GivenEmpty_WhenBuilt_ThenNil(t *testing.T) {
	if got := ConversationViews(nil); got != nil {
		t.Errorf("expected nil for empty exchanges, got %v", got)
	}
}
