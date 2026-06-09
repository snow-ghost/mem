package miner

import "strings"

// ConversationViews returns the L# Cache projections of a conversation
// as a map from `hall` tag to text:
//
//	L0 = full transcript (every turn with speaker prefix)
//	L1 = user/human turns only (drops assistant verbosity)
//	L2 = first 3 user turns (zero-cost summary proxy)
//
// Adapted from Schift's 2024 LongMemEval writeup. Indexing a conversation
// session as three drawers sharing one source_file, then retrieving via
// search.SearchByLCache with max-merge, yields roughly +20 pp R@5 on
// LongMemEval `_s_cleaned` over a single whole-session drawer.
//
// Speakers matched as user turns: "user", "human" (case-insensitive).
// An empty exchange slice returns nil.
func ConversationViews(exchanges []Exchange) map[string]string {
	if len(exchanges) == 0 {
		return nil
	}
	full := make([]string, 0, len(exchanges))
	user := make([]string, 0, len(exchanges))
	for _, ex := range exchanges {
		full = append(full, ex.Speaker+": "+ex.Content)
		switch strings.ToLower(ex.Speaker) {
		case "user", "human":
			user = append(user, ex.Content)
		}
	}
	l0 := strings.Join(full, "\n")
	l1 := strings.Join(user, "\n")
	l2 := l1
	if len(user) > 3 {
		l2 = strings.Join(user[:3], "\n")
	}
	return map[string]string{"L0": l0, "L1": l1, "L2": l2}
}
