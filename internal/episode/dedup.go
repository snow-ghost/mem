package episode

import "strings"

func IsDuplicate(ep Episode, existing []Episode) bool {
	key := dedupKey(ep)
	for _, e := range existing {
		if dedupKey(e) == key {
			return true
		}
	}
	return false
}

func dedupKey(ep Episode) string {
	return ep.Type + ":" + strings.ToLower(strings.TrimSpace(ep.Summary))
}
