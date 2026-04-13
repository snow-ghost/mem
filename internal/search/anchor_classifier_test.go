package search

import "testing"

func TestAnchorClassifier_GivenSyntheticVectors_WhenClassified_ThenMatches(t *testing.T) {
	// Build a tiny embed func that returns deterministic vectors based on
	// a per-type signature. Then verify that Classify on a near-anchor
	// query returns that type.
	dim := 8
	signatures := map[QuestionType][]float32{
		TypeKnowledgeUpdate:         {1, 0, 0, 0, 0, 0, 0, 0},
		TypeTemporalReasoning:       {0, 1, 0, 0, 0, 0, 0, 0},
		TypeSingleSessionPreference: {0, 0, 1, 0, 0, 0, 0, 0},
		TypeSingleSessionAssistant:  {0, 0, 0, 1, 0, 0, 0, 0},
		TypeMultiSession:            {0, 0, 0, 0, 1, 0, 0, 0},
		TypeSingleSessionUser:       {0, 0, 0, 0, 0, 1, 0, 0},
	}
	textToType := make(map[string]QuestionType)
	for typ, texts := range anchorTexts {
		for _, t := range texts {
			textToType[t] = typ
		}
	}
	embed := func(text string) ([]float32, error) {
		typ, ok := textToType[text]
		if !ok {
			return make([]float32, dim), nil
		}
		v := make([]float32, dim)
		copy(v, signatures[typ])
		return v, nil
	}
	c := NewAnchorClassifier(embed)
	if err := c.Prepare(); err != nil {
		t.Fatalf("prepare: %v", err)
	}

	for typ, sig := range signatures {
		got := c.Classify(sig)
		if got != typ {
			t.Errorf("classify(%v) = %v, want %v", sig, got, typ)
		}
	}
}

func TestAnchorClassifier_GivenUnpreparedOrEmpty_WhenCalled_ThenSafeDefault(t *testing.T) {
	c := NewAnchorClassifier(func(string) ([]float32, error) { return nil, nil })
	if got := c.Classify([]float32{1, 0, 0}); got != TypeSingleSessionUser {
		t.Errorf("unprepared classify = %v, want default", got)
	}
}
