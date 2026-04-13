package search

import "strings"

// porterStem applies Step 1a + 1b of the classic Porter stemming algorithm.
// These two steps cover the vast majority of English suffix variation
// (plurals, gerunds, past tense) — the higher steps add diminishing returns
// for retrieval workloads.
//
// Reference: M.F. Porter, "An algorithm for suffix stripping", 1980.
//   https://tartarus.org/martin/PorterStemmer/
func porterStem(w string) string {
	if len(w) <= 2 {
		return w
	}
	w = step1a(w)
	w = step1b(w)
	return w
}

// step1a — plural/possessive stripping.
func step1a(w string) string {
	switch {
	case strings.HasSuffix(w, "sses"):
		return w[:len(w)-2] // "caresses" -> "caress"
	case strings.HasSuffix(w, "ies"):
		return w[:len(w)-2] // "ponies" -> "poni"
	case strings.HasSuffix(w, "ss"):
		return w // "caress" stays
	case strings.HasSuffix(w, "s"):
		return w[:len(w)-1] // "cats" -> "cat"
	}
	return w
}

// step1b — gerund / past-tense stripping with cleanup.
func step1b(w string) string {
	stripped := false
	switch {
	case strings.HasSuffix(w, "eed"):
		// "agreed" -> "agree" only if measure(stem) > 0
		stem := w[:len(w)-3]
		if measure(stem) > 0 {
			w = w[:len(w)-1]
		}
		return w
	case strings.HasSuffix(w, "ed"):
		stem := w[:len(w)-2]
		if hasVowel(stem) {
			w = stem
			stripped = true
		}
	case strings.HasSuffix(w, "ing"):
		stem := w[:len(w)-3]
		if hasVowel(stem) {
			w = stem
			stripped = true
		}
	}
	if !stripped {
		return w
	}
	// Step 1b cleanup
	switch {
	case strings.HasSuffix(w, "at"),
		strings.HasSuffix(w, "bl"),
		strings.HasSuffix(w, "iz"):
		return w + "e" // "conflated" -> "conflate"
	case endsDoubleConsonant(w) && !endsLSZ(w):
		return w[:len(w)-1] // "hopping" -> "hop"
	case measure(w) == 1 && endsCVC(w):
		return w + "e" // "filing" -> "file"
	}
	return w
}

func isVowelByte(b byte) bool {
	return b == 'a' || b == 'e' || b == 'i' || b == 'o' || b == 'u'
}

// isConsonant — y is a consonant only at the start, or after another vowel.
func isConsonant(s string, i int) bool {
	if isVowelByte(s[i]) {
		return false
	}
	if s[i] == 'y' {
		if i == 0 {
			return true
		}
		return !isConsonant(s, i-1)
	}
	return true
}

func hasVowel(s string) bool {
	for i := range s {
		if !isConsonant(s, i) {
			return true
		}
	}
	return false
}

// measure counts the number of VC sequences in s — Porter's "m" value.
func measure(s string) int {
	n := len(s)
	if n == 0 {
		return 0
	}
	m := 0
	prevConsonant := isConsonant(s, 0)
	for i := 1; i < n; i++ {
		c := isConsonant(s, i)
		if prevConsonant && !c {
			// transition from consonant block to vowel block (no contribution)
		} else if !prevConsonant && c {
			// transition vowel -> consonant: closes a VC pair
			m++
		}
		prevConsonant = c
	}
	return m
}

func endsDoubleConsonant(s string) bool {
	n := len(s)
	if n < 2 {
		return false
	}
	return s[n-1] == s[n-2] && isConsonant(s, n-1)
}

func endsLSZ(s string) bool {
	if len(s) == 0 {
		return false
	}
	c := s[len(s)-1]
	return c == 'l' || c == 's' || c == 'z'
}

// endsCVC: last three chars are consonant-vowel-consonant, and the final
// consonant is not w/x/y. Used in Porter step 1b cleanup.
func endsCVC(s string) bool {
	n := len(s)
	if n < 3 {
		return false
	}
	if !isConsonant(s, n-3) || isConsonant(s, n-2) || !isConsonant(s, n-1) {
		return false
	}
	last := s[n-1]
	return last != 'w' && last != 'x' && last != 'y'
}
