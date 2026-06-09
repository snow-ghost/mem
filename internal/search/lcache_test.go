package search

import (
	"fmt"
	"math"
	"testing"

	"github.com/snow-ghost/mem/internal/db"
	"github.com/snow-ghost/mem/internal/embeddings"
)

func insertDrawerWithEmbedding(t testing.TB, d *db.DB, content, sourceFile, hall string, vec []float32) int64 {
	t.Helper()
	drawerCounter++
	res, err := d.Exec(
		`INSERT INTO drawers (content, content_hash, wing_id, room_id, hall, source_file, source_type, embedding)
		 VALUES (?, ?, 1, 1, ?, ?, 'conversation', ?)`,
		content, fmt.Sprintf("hash_lcache_%d", drawerCounter), hall, sourceFile, embeddings.Encode(vec),
	)
	if err != nil {
		t.Fatalf("insert drawer: %v", err)
	}
	id, _ := res.LastInsertId()
	return id
}

func unitVec(dim int, direction int) []float32 {
	v := make([]float32, dim)
	// Anchor at canonical basis; small offsets so cosine differs per drawer.
	v[direction%dim] = 1
	for i := range v {
		if i != direction%dim {
			v[i] = float32(math.Sin(float64(i+direction))) * 0.01
		}
	}
	return v
}

func TestSearchByLCache_GivenVariantsPerSource_WhenSearched_ThenMaxScorePerSource(t *testing.T) {
	d := openTestDB(t)
	defer d.Close()

	d.Exec("INSERT INTO wings (name, type) VALUES ('bench', 'project')")
	d.Exec("INSERT INTO rooms (name, wing_id) VALUES ('general', 1)")

	dim := 16
	// Session A has 3 variants, one of which matches the query vector exactly.
	target := unitVec(dim, 0)
	insertDrawerWithEmbedding(t, d, "A-L0 full session text", "session_A", "L0", unitVec(dim, 3))
	winnerID := insertDrawerWithEmbedding(t, d, "A-L1 user turns", "session_A", "L1", target)
	insertDrawerWithEmbedding(t, d, "A-L2 first 3 user", "session_A", "L2", unitVec(dim, 5))

	// Session B has one variant that's only loosely related.
	insertDrawerWithEmbedding(t, d, "B-full", "session_B", "L0", unitVec(dim, 7))

	results, err := SearchByLCache(d, target, 0, 0, Scope{}, 5)
	if err != nil {
		t.Fatalf("lcache search: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 sources, got %d: %+v", len(results), results)
	}
	// Session A must be first (it contains the exact-match variant).
	if results[0].SourceFile != "session_A" {
		t.Fatalf("expected session_A first, got %q", results[0].SourceFile)
	}
	if results[0].DrawerID != winnerID {
		t.Errorf("expected winning drawer %d (L1), got %d", winnerID, results[0].DrawerID)
	}
	if results[0].Score <= results[1].Score {
		t.Errorf("expected session_A score > session_B, got %.4f vs %.4f",
			results[0].Score, results[1].Score)
	}
	// No duplicate session entries.
	seen := map[string]bool{}
	for _, r := range results {
		if seen[r.SourceFile] {
			t.Errorf("duplicate source_file %q in results", r.SourceFile)
		}
		seen[r.SourceFile] = true
	}
}

func TestSearchByLCache_GivenNoVariants_WhenSearched_ThenMatchesSearchVector(t *testing.T) {
	d := openTestDB(t)
	defer d.Close()

	d.Exec("INSERT INTO wings (name, type) VALUES ('bench', 'project')")
	d.Exec("INSERT INTO rooms (name, wing_id) VALUES ('general', 1)")

	dim := 8
	insertDrawerWithEmbedding(t, d, "single_A", "s_A", "facts", unitVec(dim, 0))
	insertDrawerWithEmbedding(t, d, "single_B", "s_B", "facts", unitVec(dim, 2))
	insertDrawerWithEmbedding(t, d, "single_C", "s_C", "facts", unitVec(dim, 4))

	q := unitVec(dim, 0)
	lcache, _ := SearchByLCache(d, q, 0, 0, Scope{}, 10)
	vec, _ := SearchVector(d, q, 0, 0, 10)

	if len(lcache) != len(vec) {
		t.Fatalf("lcache=%d vector=%d, expected equal when each source has one drawer",
			len(lcache), len(vec))
	}
	for i := range lcache {
		if lcache[i].DrawerID != vec[i].DrawerID {
			t.Errorf("result %d: lcache drawer %d != vector drawer %d",
				i, lcache[i].DrawerID, vec[i].DrawerID)
		}
	}
}
