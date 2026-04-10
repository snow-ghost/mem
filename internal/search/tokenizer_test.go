package search

import "testing"

func TestTokenize_GivenSentence_WhenTokenized_ThenStopwordsRemoved(t *testing.T) {
	tokens := Tokenize("The quick brown fox jumps over the lazy dog")
	want := []string{"quick", "brown", "fox", "jumps", "lazy", "dog"}
	if len(tokens) != len(want) {
		t.Fatalf("got %d tokens %v, want %d %v", len(tokens), tokens, len(want), want)
	}
	for i, tok := range tokens {
		if tok != want[i] {
			t.Errorf("token[%d] = %q, want %q", i, tok, want[i])
		}
	}
}

func TestTokenize_GivenEmpty_WhenTokenized_ThenEmpty(t *testing.T) {
	tokens := Tokenize("")
	if len(tokens) != 0 {
		t.Errorf("got %d tokens, want 0", len(tokens))
	}
}

func TestTokenize_GivenAllStopwords_WhenTokenized_ThenEmpty(t *testing.T) {
	tokens := Tokenize("the a an is are")
	if len(tokens) != 0 {
		t.Errorf("got %d tokens %v, want 0", len(tokens), tokens)
	}
}

func TestTokenFrequency_GivenTokens_WhenComputed_ThenCorrect(t *testing.T) {
	tokens := []string{"hello", "world", "hello"}
	freq := TokenFrequency(tokens)
	if freq["hello"] < 0.66 || freq["hello"] > 0.67 {
		t.Errorf("hello freq = %f, want ~0.667", freq["hello"])
	}
	if freq["world"] < 0.33 || freq["world"] > 0.34 {
		t.Errorf("world freq = %f, want ~0.333", freq["world"])
	}
}
