package search

import (
	"sort"
	"strconv"

	"github.com/snow-ghost/mem/internal/db"
)

// SearchByLCache runs vector retrieval and collapses multiple drawers
// per source_file into one result, keeping the max similarity across
// variants. This is the retrieval half of Schift's "L# Cache" technique.
//
// If the caller has indexed multiple views of each logical item under a
// shared source_file (e.g., L0=full session, L1=user turns only, L2=first
// 3 user turns) this returns one SearchResult per source ranked by the
// best-matching variant. On LongMemEval `_s_cleaned` with scoped
// retrieval, moving from a single whole-session drawer to L# max-merge
// lifts R@5(sid) by roughly +20 pp.
//
// On palaces without variant indexing, each source contributes one drawer
// and SearchByLCache behaves identically to SearchVectorScoped.
//
// The internal candidate pool is `limit * 12` (≥ 20). Rationale: with
// three variants per source that yields ~4x distinct sources before the
// max-collapse, enough for stable top-`limit` ranking without loading
// every embedding in the palace.
func SearchByLCache(d *db.DB, queryVec []float32, wingID, roomID int64, scope Scope, limit int) ([]SearchResult, error) {
	if limit <= 0 {
		limit = 5
	}
	pool := limit * 12
	if pool < 20 {
		pool = 20
	}
	candidates, err := SearchVectorScoped(d, queryVec, wingID, roomID, scope, pool)
	if err != nil {
		return nil, err
	}

	bySource := make(map[string]SearchResult, len(candidates))
	for _, r := range candidates {
		key := r.SourceFile
		if key == "" {
			// Treat empty source as a unique key per drawer so orphan
			// drawers don't collapse into a single result.
			key = rowKey(r.DrawerID)
		}
		if existing, ok := bySource[key]; !ok || r.Score > existing.Score {
			bySource[key] = r
		}
	}

	out := make([]SearchResult, 0, len(bySource))
	for _, r := range bySource {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// SearchByLCacheUnscoped is a convenience wrapper without a Scope arg.
func SearchByLCacheUnscoped(d *db.DB, queryVec []float32, wingID, roomID int64, limit int) ([]SearchResult, error) {
	return SearchByLCache(d, queryVec, wingID, roomID, Scope{}, limit)
}

func rowKey(id int64) string {
	return "__drawer_" + strconv.FormatInt(id, 10)
}
