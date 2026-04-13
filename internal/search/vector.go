package search

import (
	"sort"

	"github.com/snow-ghost/mem/internal/db"
	"github.com/snow-ghost/mem/internal/embeddings"
)

// SearchVector scores every indexed drawer by cosine similarity against the
// query vector. Returns the top `limit` results.
//
// This is a full scan — fine up to ~100k drawers on a modern CPU. For larger
// palaces an ANN index (e.g., HNSW) would be needed.
func SearchVector(d *db.DB, queryVec []float32, wingID, roomID int64, limit int) ([]SearchResult, error) {
	if limit <= 0 {
		limit = 5
	}
	if len(queryVec) == 0 {
		return nil, nil
	}

	query := `SELECT d.id, d.content, d.wing_id, d.room_id, d.hall, d.source_file,
		COALESCE(w.name, ''), COALESCE(rm.name, ''), d.embedding
		FROM drawers d
		LEFT JOIN wings w ON d.wing_id = w.id
		LEFT JOIN rooms rm ON d.room_id = rm.id
		WHERE d.embedding IS NOT NULL`
	var args []any
	if wingID > 0 {
		query += " AND d.wing_id = ?"
		args = append(args, wingID)
	}
	if roomID > 0 {
		query += " AND d.room_id = ?"
		args = append(args, roomID)
	}

	rows, err := d.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var sr SearchResult
		var blob []byte
		if err := rows.Scan(&sr.DrawerID, &sr.Content, &sr.WingID, &sr.RoomID,
			&sr.Hall, &sr.SourceFile, &sr.WingName, &sr.RoomName, &blob); err != nil {
			continue
		}
		vec, err := embeddings.Decode(blob)
		if err != nil {
			continue
		}
		sr.Score = float64(embeddings.Cosine(queryVec, vec))
		results = append(results, sr)
	}

	sort.Slice(results, func(i, j int) bool { return results[i].Score > results[j].Score })
	if len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

// SearchHybrid combines BM25 and vector results using Reciprocal Rank Fusion
// (RRF) with k=60 — the standard constant from the original RRF paper.
// Equivalent to SearchHybridWeighted with bm25Weight=0.5.
func SearchHybrid(d *db.DB, query string, queryVec []float32, wingID, roomID int64, limit int) ([]SearchResult, error) {
	return SearchHybridWeighted(d, query, queryVec, wingID, roomID, limit, 0.5)
}

// DefaultPerTypeWeights is the LongMemEval-derived BM25 weight per
// question type (oracle sweep result). Used by SearchHybridAuto when
// the caller doesn't supply a custom map. Knowledge-update questions
// reformulate facts so vector helps more (low BM25 weight); single-
// session-* are recall-of-specifics queries where BM25 wins.
var DefaultPerTypeWeights = map[QuestionType]float64{
	TypeKnowledgeUpdate:         0.30,
	TypeSingleSessionPreference: 0.70,
	TypeTemporalReasoning:       0.70,
	TypeMultiSession:            0.70,
	TypeSingleSessionUser:       0.90,
	TypeSingleSessionAssistant:  0.90,
}

// ApplyRecencyBoost re-scores results in-place by adding a recency
// bonus, then re-sorts. Larger drawer IDs are treated as newer
// (matches insert order for SQLite AUTOINCREMENT). The bonus is
//
//	bonus(d) = recencyWeight * (drawer.ID - minID) / (maxID - minID)
//
// so the most recent drawer in the result set gets the full bonus and
// the oldest gets none. Useful when the downstream metric prefers the
// latest version of a fact (ConvoMem changing_evidence-style queries).
func ApplyRecencyBoost(results []SearchResult, recencyWeight float64) []SearchResult {
	if len(results) <= 1 || recencyWeight == 0 {
		return results
	}
	var minID, maxID int64 = results[0].DrawerID, results[0].DrawerID
	for _, r := range results {
		if r.DrawerID < minID {
			minID = r.DrawerID
		}
		if r.DrawerID > maxID {
			maxID = r.DrawerID
		}
	}
	span := float64(maxID - minID)
	if span == 0 {
		return results
	}
	for i := range results {
		bonus := recencyWeight * float64(results[i].DrawerID-minID) / span
		results[i].Score += bonus
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})
	return results
}

// SearchHybridAuto classifies the query, looks up the per-type BM25
// weight (from DefaultPerTypeWeights or override), and runs weighted
// RRF. Falls back to weight 0.7 if the predicted type isn't in the map.
func SearchHybridAuto(d *db.DB, query string, queryVec []float32, wingID, roomID int64, limit int, weights map[QuestionType]float64) ([]SearchResult, error) {
	if weights == nil {
		weights = DefaultPerTypeWeights
	}
	t := ClassifyQuestion(query)
	w, ok := weights[t]
	if !ok {
		w = 0.7
	}
	return SearchHybridWeighted(d, query, queryVec, wingID, roomID, limit, w)
}

// SearchHybridWeighted is RRF with adjustable BM25 weight in [0, 1].
// The vector contribution is weighted (1 - bm25Weight). Setting bm25Weight=1
// degenerates to BM25-only ranking, 0 to vector-only.
//
//	rrf(d) = bm25Weight / (k + bm25_rank(d))
//	       + (1-bm25Weight) / (k + vec_rank(d))
//
// Drawers missing from one system contribute 0 from that system. We widen the
// per-system candidate pool to 4*limit to give RRF enough material to work with.
func SearchHybridWeighted(d *db.DB, query string, queryVec []float32, wingID, roomID int64, limit int, bm25Weight float64) ([]SearchResult, error) {
	if limit <= 0 {
		limit = 5
	}
	if bm25Weight < 0 {
		bm25Weight = 0
	}
	if bm25Weight > 1 {
		bm25Weight = 1
	}
	pool := limit * 4
	const k = 60.0
	vecWeight := 1.0 - bm25Weight

	bm25Results, err := Search(d, query, wingID, roomID, pool)
	if err != nil {
		return nil, err
	}
	vecResults, err := SearchVector(d, queryVec, wingID, roomID, pool)
	if err != nil {
		return nil, err
	}

	fused := make(map[int64]*SearchResult)
	for rank, r := range bm25Results {
		copy := r
		fused[r.DrawerID] = &copy
		fused[r.DrawerID].Score = bm25Weight / (k + float64(rank+1))
	}
	for rank, r := range vecResults {
		score := vecWeight / (k + float64(rank+1))
		if existing, ok := fused[r.DrawerID]; ok {
			existing.Score += score
		} else {
			copy := r
			copy.Score = score
			fused[r.DrawerID] = &copy
		}
	}

	out := make([]SearchResult, 0, len(fused))
	for _, r := range fused {
		out = append(out, *r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// IndexEmbedding stores a precomputed embedding blob on a drawer.
func IndexEmbedding(d *db.DB, drawerID int64, blob []byte) error {
	_, err := d.Exec("UPDATE drawers SET embedding = ? WHERE id = ?", blob, drawerID)
	return err
}

// CountDrawersWithEmbeddings returns how many drawers already have embeddings.
func CountDrawersWithEmbeddings(d *db.DB) (int, error) {
	var n int
	err := d.QueryRow("SELECT COUNT(*) FROM drawers WHERE embedding IS NOT NULL").Scan(&n)
	return n, err
}

// BuildHNSWFromPalace loads every embedded drawer and inserts it into a
// fresh HNSWIndex. Returns nil if no drawers have embeddings yet.
// Caller decides when this is worth doing: at small scale (<5k drawers)
// SearchVector's full scan is competitive with HNSW; at larger scale
// HNSW gives ~5x speedup at 10k and ~25x at 50k (see hnsw_test.go).
//
// For repeated CLI invocations prefer LoadOrBuildHNSW which transparently
// caches the graph in the hnsw_cache table.
func BuildHNSWFromPalace(d *db.DB) (*HNSWIndex, error) {
	rows, err := d.Query("SELECT id, embedding FROM drawers WHERE embedding IS NOT NULL ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var idx *HNSWIndex
	for rows.Next() {
		var id int64
		var blob []byte
		if err := rows.Scan(&id, &blob); err != nil {
			continue
		}
		vec, err := embeddings.Decode(blob)
		if err != nil {
			continue
		}
		if idx == nil {
			idx = NewHNSWIndex(len(vec))
		}
		idx.Insert(id, vec)
	}
	return idx, nil
}

// LoadHNSWFromCache returns a previously persisted HNSW index from the
// hnsw_cache table, or (nil, nil) if no cache row exists. The check
// against drawer_count is a cheap sanity guard — if the embedded-drawer
// count has changed since we serialized, the cache is stale and the
// caller should rebuild.
func LoadHNSWFromCache(d *db.DB, name string) (*HNSWIndex, error) {
	var blob []byte
	var cachedCount int
	err := d.QueryRow("SELECT blob, drawer_count FROM hnsw_cache WHERE name = ?", name).
		Scan(&blob, &cachedCount)
	if err != nil {
		return nil, nil
	}
	currentCount, _ := CountDrawersWithEmbeddings(d)
	if currentCount != cachedCount {
		return nil, nil // stale
	}
	return LoadHNSW(blob)
}

// SaveHNSWToCache serializes the index to the hnsw_cache table under
// the given name.
func SaveHNSWToCache(d *db.DB, name string, idx *HNSWIndex) error {
	if idx == nil || idx.Size() == 0 {
		return nil
	}
	blob, err := idx.Marshal()
	if err != nil {
		return err
	}
	_, err = d.Exec(`INSERT INTO hnsw_cache (name, drawer_count, dim, blob, updated_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(name) DO UPDATE SET drawer_count=excluded.drawer_count, dim=excluded.dim,
			blob=excluded.blob, updated_at=excluded.updated_at`,
		name, idx.Size(), idx.dim, blob)
	return err
}

// LoadOrBuildHNSW returns a cached HNSW index when available and not
// stale, or builds a fresh one and persists it. `name` is a logical
// label; "default" works for single-index palaces.
func LoadOrBuildHNSW(d *db.DB, name string) (*HNSWIndex, error) {
	if idx, _ := LoadHNSWFromCache(d, name); idx != nil {
		return idx, nil
	}
	idx, err := BuildHNSWFromPalace(d)
	if err != nil || idx == nil {
		return idx, err
	}
	_ = SaveHNSWToCache(d, name, idx)
	return idx, nil
}

// SearchHNSW runs vector search via a pre-built HNSW index, returning
// SearchResult entries hydrated from the database. Falls back to nil
// if the index is empty. Each result's Score is the actual cosine
// similarity (matches SearchVector for downstream comparison).
func SearchHNSW(d *db.DB, idx *HNSWIndex, queryVec []float32, limit int) ([]SearchResult, error) {
	if idx == nil || idx.Size() == 0 || len(queryVec) == 0 {
		return nil, nil
	}
	if limit <= 0 {
		limit = 5
	}
	ids := idx.Search(queryVec, limit, 0)
	if len(ids) == 0 {
		return nil, nil
	}

	// Hydrate each id, including the embedding so we can compute the
	// real cosine score (HNSW orders by it but the index doesn't return
	// the raw value).
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	q := `SELECT d.id, d.content, d.wing_id, d.room_id, d.hall, d.source_file,
		COALESCE(w.name, ''), COALESCE(rm.name, ''), d.embedding
		FROM drawers d
		LEFT JOIN wings w ON d.wing_id = w.id
		LEFT JOIN rooms rm ON d.room_id = rm.id
		WHERE d.id IN (` + joinComma(placeholders) + `)`
	rows, err := d.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byID := make(map[int64]SearchResult, len(ids))
	for rows.Next() {
		var sr SearchResult
		var blob []byte
		if err := rows.Scan(&sr.DrawerID, &sr.Content, &sr.WingID, &sr.RoomID,
			&sr.Hall, &sr.SourceFile, &sr.WingName, &sr.RoomName, &blob); err != nil {
			continue
		}
		if vec, err := embeddings.Decode(blob); err == nil {
			sr.Score = float64(embeddings.Cosine(queryVec, vec))
		}
		byID[sr.DrawerID] = sr
	}

	// Preserve HNSW order (= approximate relevance order).
	out := make([]SearchResult, 0, len(ids))
	for _, id := range ids {
		if sr, ok := byID[id]; ok {
			out = append(out, sr)
		}
	}
	return out, nil
}

func joinComma(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += ","
		}
		out += p
	}
	return out
}


type DrawerContent struct {
	ID      int64
	Content string
}

// ListDrawersWithoutEmbeddings returns drawer IDs and contents that still
// need embedding. Caller should batch these through an embeddings client.
func ListDrawersWithoutEmbeddings(d *db.DB, limit int) ([]DrawerContent, error) {
	q := "SELECT id, content FROM drawers WHERE embedding IS NULL ORDER BY id"
	var args []any
	if limit > 0 {
		q += " LIMIT ?"
		args = append(args, limit)
	}
	rows, err := d.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []DrawerContent
	for rows.Next() {
		var item DrawerContent
		if err := rows.Scan(&item.ID, &item.Content); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}
