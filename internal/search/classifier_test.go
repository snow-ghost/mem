package search

import "testing"

func TestClassifyQuestion_GivenSamples_WhenClassified_ThenMatchesIntent(t *testing.T) {
	cases := []struct {
		q    string
		want QuestionType
	}{
		// single-session-assistant: "remind me", "you mentioned", "previous"
		{"Can you remind me of the name of that restaurant?", TypeSingleSessionAssistant},
		{"I'm going back to our previous conversation about dinosaurs.", TypeSingleSessionAssistant},
		{"What was that hotel you recommended in Tokyo?", TypeSingleSessionAssistant},

		// single-session-preference: "can you recommend/suggest"
		{"Can you recommend some resources for video editing?", TypeSingleSessionPreference},
		{"Can you suggest a hotel for my upcoming trip?", TypeSingleSessionPreference},

		// temporal-reasoning: "first", "before", "how many days"
		{"What was the first issue I had with my new car?", TypeTemporalReasoning},
		{"Which event did I attend first, the workshop or the conference?", TypeTemporalReasoning},
		{"How many days before the team meeting did I attend the workshop?", TypeTemporalReasoning},

		// multi-session: count questions
		{"How many projects have I led or am currently leading?", TypeMultiSession},
		{"How many model kits have I bought?", TypeMultiSession},

		// single-session-user: default
		{"What degree did I graduate with?", TypeSingleSessionUser},
		{"Where did I redeem a $5 coupon?", TypeSingleSessionUser},

		// edge cases
		{"", TypeSingleSessionUser},
	}
	for _, c := range cases {
		if got := ClassifyQuestion(c.q); got != c.want {
			t.Errorf("ClassifyQuestion(%q) = %q, want %q", c.q, got, c.want)
		}
	}
}
