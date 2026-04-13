package search

import (
	"strings"
	"unicode"
)

var stopwords = map[string]bool{
	"a": true, "an": true, "and": true, "are": true, "as": true, "at": true,
	"be": true, "by": true, "for": true, "from": true, "has": true, "he": true,
	"in": true, "is": true, "it": true, "its": true, "of": true, "on": true,
	"or": true, "that": true, "the": true, "to": true, "was": true, "were": true,
	"will": true, "with": true, "you": true, "your": true, "this": true, "but": true,
	"had": true, "have": true, "his": true, "her": true, "not": true, "she": true,
	"they": true, "we": true, "been": true, "can": true, "do": true, "did": true,
	"does": true, "each": true, "get": true, "got": true, "how": true, "i": true,
	"if": true, "into": true, "just": true, "me": true, "my": true, "no": true,
	"our": true, "out": true, "so": true, "some": true, "than": true, "them": true,
	"then": true, "there": true, "these": true, "those": true, "up": true, "what": true,
	"when": true, "which": true, "who": true, "whom": true, "why": true, "would": true,
	"about": true, "after": true, "all": true, "also": true, "any": true, "because": true,
	"before": true, "between": true, "both": true, "could": true, "during": true, "few": true,
	"more": true, "most": true, "other": true, "over": true, "same": true, "should": true,
	"such": true, "through": true, "too": true, "under": true, "very": true, "where": true,
}

func Tokenize(text string) []string {
	text = strings.ToLower(text)
	words := strings.FieldsFunc(text, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})

	var tokens []string
	for _, w := range words {
		if len(w) < 2 || stopwords[w] {
			continue
		}
		tokens = append(tokens, porterStem(w))
	}
	return tokens
}

func TokenFrequency(tokens []string) map[string]float64 {
	freq := make(map[string]float64)
	for _, t := range tokens {
		freq[t]++
	}
	total := float64(len(tokens))
	if total == 0 {
		return freq
	}
	for t := range freq {
		freq[t] /= total
	}
	return freq
}
