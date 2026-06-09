package search

import "strings"

// Scope narrows a search to drawers whose source_file is in SessionIDs.
//
// Applications that know the active conversation/session set (chat agents,
// per-document assistants) can pass a Scope to emulate MemPalace-style
// ephemeral palaces without duplicating the index. On LongMemEval
// `_s_cleaned` this lifts R@5(sid) by +7.6 pp when the haystack is known.
//
// An empty Scope (zero SessionIDs) is a no-op — the caller gets the same
// behavior as the un-scoped Search* functions.
type Scope struct {
	SessionIDs []string
}

// IsEmpty reports whether the scope applies no filter.
func (s Scope) IsEmpty() bool { return len(s.SessionIDs) == 0 }

// appendFilter emits ` AND d.source_file IN (?, ?, ...)` with positional
// arguments appended to `args`. The caller prefixes its own SELECT with
// an alias `d` for the drawers table.
func (s Scope) appendFilter(args []any) (string, []any) {
	if len(s.SessionIDs) == 0 {
		return "", args
	}
	placeholders := make([]string, len(s.SessionIDs))
	for i, id := range s.SessionIDs {
		placeholders[i] = "?"
		args = append(args, id)
	}
	return " AND d.source_file IN (" + strings.Join(placeholders, ",") + ")", args
}
