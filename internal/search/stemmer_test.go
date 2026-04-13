package search

import "testing"

func TestPorterStem_GivenCommonWords_WhenStemmed_ThenMatchesExpected(t *testing.T) {
	cases := []struct{ in, want string }{
		// Step 1a — plurals
		{"cats", "cat"},
		{"caresses", "caress"},
		{"caress", "caress"},
		{"ponies", "poni"},
		{"ties", "ti"},

		// Step 1b — gerund/past tense, with cleanup
		{"feed", "feed"},
		{"agreed", "agree"},
		{"plastered", "plaster"},
		{"bled", "bled"},  // "bl" has no vowel → no strip
		{"motoring", "motor"},
		{"sing", "sing"}, // "s" has no vowel → no strip
		{"hopping", "hop"},
		{"tanned", "tan"},
		{"falling", "fall"},
		{"hissing", "hiss"},
		{"fizzed", "fizz"},
		{"failing", "fail"},
		{"filing", "file"},
		{"conflated", "conflate"},
		{"troubled", "trouble"},
		{"sized", "size"},

		// Edge cases
		{"is", "is"},
		{"a", "a"},
	}
	for _, c := range cases {
		if got := porterStem(c.in); got != c.want {
			t.Errorf("porterStem(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestTokenize_GivenWords_WhenTokenized_ThenStemsApplied(t *testing.T) {
	// happily → happily (step 1c not implemented)
	got := Tokenize("running cats happily processing trains")
	want := []string{"run", "cat", "happily", "process", "train"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d] got %q, want %q", i, got[i], want[i])
		}
	}
}
