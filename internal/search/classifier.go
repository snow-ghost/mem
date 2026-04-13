package search

import (
	"regexp"
	"strings"
)

// QuestionType is a coarse-grained label for a query, used to pick
// per-type tuning (RRF weight, rerank gating, etc.). Matches the
// LongMemEval question types where useful and stays generic otherwise.
type QuestionType string

const (
	TypeSingleSessionUser       QuestionType = "single-session-user"
	TypeSingleSessionAssistant  QuestionType = "single-session-assistant"
	TypeSingleSessionPreference QuestionType = "single-session-preference"
	TypeMultiSession            QuestionType = "multi-session"
	TypeKnowledgeUpdate         QuestionType = "knowledge-update"
	TypeTemporalReasoning       QuestionType = "temporal-reasoning"
)

var (
	reAssistantRecall = regexp.MustCompile(`(?i)\b(remind me|you (mentioned|recommended|suggested|told|said)|previous (chat|conversation|discussion)|going back to our|our (previous|earlier))\b`)
	rePreference      = regexp.MustCompile(`(?i)\bcan you (recommend|suggest)|please (recommend|suggest)|what (are some|would you)\b`)
	reTemporal        = regexp.MustCompile(`(?i)\b(first|last|before|after|earliest|latest|how (many|long) (days|weeks|months|years|hours))\b`)
	reTemporalCompare = regexp.MustCompile(`(?i)\bwhich .* (first|earlier|later|before|after)\b`)
	reCountStart      = regexp.MustCompile(`(?i)^\s*how many\b`)
)

// ClassifyQuestion returns a best-effort question type label using
// keyword heuristics. It is intentionally simple — no ML, no embeddings —
// because the categorical signal is weak in production (we don't get
// ground-truth types) and the cost of misclassification is bounded
// (worst case: revert to default RRF weight / no rerank).
//
// Order matters: more specific patterns checked first.
func ClassifyQuestion(q string) QuestionType {
	q = strings.TrimSpace(q)
	if q == "" {
		return TypeSingleSessionUser
	}
	switch {
	case reAssistantRecall.MatchString(q):
		return TypeSingleSessionAssistant
	case rePreference.MatchString(q):
		return TypeSingleSessionPreference
	case reTemporalCompare.MatchString(q), reTemporal.MatchString(q):
		return TypeTemporalReasoning
	case reCountStart.MatchString(q):
		return TypeMultiSession
	}
	return TypeSingleSessionUser
}
