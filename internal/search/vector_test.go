package search

import (
	"path/filepath"
	"testing"

	"github.com/snow-ghost/mem/internal/db"
	"github.com/snow-ghost/mem/internal/embeddings"
)

func openVecTestDB(t testing.TB) *db.DB {
	t.Helper()
	dir := t.TempDir()
	d, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	db.InitSchema(d)
	return d
}

func TestVectorSearch_GivenIndexedDrawers_WhenSearched_ThenClosestFirst(t *testing.T) {
	d := openVecTestDB(t)
	defer d.Close()

	d.Exec("INSERT INTO wings (name, type) VALUES ('test', 'project')")
	d.Exec("INSERT INTO rooms (name, wing_id) VALUES ('general', 1)")

	// Three drawers with distinct vectors.
	id1 := insertDrawer(t, d, "auth content", 1, 1)
	id2 := insertDrawer(t, d, "deploy content", 1, 1)
	id3 := insertDrawer(t, d, "database content", 1, 1)

	IndexEmbedding(d, id1, embeddings.Encode([]float32{1, 0, 0}))
	IndexEmbedding(d, id2, embeddings.Encode([]float32{0, 1, 0}))
	IndexEmbedding(d, id3, embeddings.Encode([]float32{0, 0, 1}))

	// Query closest to id2
	results, err := SearchVector(d, []float32{0.1, 0.9, 0.0}, 0, 0, 3)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected results")
	}
	if results[0].DrawerID != id2 {
		t.Errorf("expected id2 first, got %d", results[0].DrawerID)
	}
}

func TestVectorSearch_GivenNoEmbeddings_WhenSearched_ThenEmpty(t *testing.T) {
	d := openVecTestDB(t)
	defer d.Close()
	d.Exec("INSERT INTO wings (name, type) VALUES ('test', 'project')")
	d.Exec("INSERT INTO rooms (name, wing_id) VALUES ('general', 1)")
	insertDrawer(t, d, "no vector", 1, 1)

	results, _ := SearchVector(d, []float32{1, 0, 0}, 0, 0, 5)
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestHybridSearch_GivenBM25AndVector_WhenFused_ThenRRFCombines(t *testing.T) {
	d := openVecTestDB(t)
	defer d.Close()

	d.Exec("INSERT INTO wings (name, type) VALUES ('test', 'project')")
	d.Exec("INSERT INTO rooms (name, wing_id) VALUES ('general', 1)")

	// id1: lexical match for "unique token", weak vector match
	// id2: no lexical match, strong vector match
	// id3: both weak
	id1 := insertDrawer(t, d, "unique token authentication login", 1, 1)
	id2 := insertDrawer(t, d, "a completely different topic", 1, 1)
	id3 := insertDrawer(t, d, "some filler content", 1, 1)

	IndexDrawer(d, id1, "unique token authentication login")
	IndexDrawer(d, id2, "a completely different topic")
	IndexDrawer(d, id3, "some filler content")

	IndexEmbedding(d, id1, embeddings.Encode([]float32{0.1, 0.1, 1.0}))
	IndexEmbedding(d, id2, embeddings.Encode([]float32{1.0, 0.0, 0.0}))
	IndexEmbedding(d, id3, embeddings.Encode([]float32{0.0, 1.0, 0.0}))

	// Query: lexically hits id1, vector points at id2
	results, err := SearchHybrid(d, "unique token", []float32{1.0, 0.0, 0.0}, 0, 0, 3)
	if err != nil {
		t.Fatalf("hybrid: %v", err)
	}
	if len(results) < 2 {
		t.Fatalf("expected 2+ results, got %d", len(results))
	}
	// Both id1 and id2 should appear in top 3; hybrid covers both signals.
	foundIDs := map[int64]bool{}
	for _, r := range results {
		foundIDs[r.DrawerID] = true
	}
	if !foundIDs[id1] || !foundIDs[id2] {
		t.Errorf("expected both id1 and id2 in results, got %v", foundIDs)
	}
}

func TestApplyRecencyBoost_GivenTiedScores_WhenBoosted_ThenNewerWins(t *testing.T) {
	in := []SearchResult{
		{DrawerID: 100, Score: 1.0},
		{DrawerID: 200, Score: 1.0},
		{DrawerID: 50, Score: 1.0},
	}
	out := ApplyRecencyBoost(in, 0.5)
	// All scores tied → boost orders by DrawerID descending.
	if out[0].DrawerID != 200 {
		t.Errorf("top should be newest (200), got %d", out[0].DrawerID)
	}
	if out[2].DrawerID != 50 {
		t.Errorf("bottom should be oldest (50), got %d", out[2].DrawerID)
	}
}

func TestApplyRecencyBoost_GivenStrongScoreGap_WhenBoosted_ThenStrongScoreStillWins(t *testing.T) {
	in := []SearchResult{
		{DrawerID: 100, Score: 0.10},
		{DrawerID: 200, Score: 0.50},
	}
	out := ApplyRecencyBoost(in, 0.1)
	// Strong score (0.50) + 0 bonus > weak score (0.10) + 0.1 bonus
	if out[0].DrawerID != 200 {
		t.Errorf("top should still be 200 (strong score), got %d", out[0].DrawerID)
	}
}

func TestApplyRecencyBoost_GivenZeroWeight_WhenCalled_ThenNoOp(t *testing.T) {
	in := []SearchResult{
		{DrawerID: 100, Score: 1.0},
		{DrawerID: 200, Score: 0.5},
	}
	out := ApplyRecencyBoost(in, 0)
	if out[0].Score != 1.0 || out[1].Score != 0.5 {
		t.Errorf("zero weight should not change scores, got %v", out)
	}
}

func TestListDrawersWithoutEmbeddings_GivenMixed_WhenListed_ThenOnlyMissing(t *testing.T) {
	d := openVecTestDB(t)
	defer d.Close()
	d.Exec("INSERT INTO wings (name, type) VALUES ('test', 'project')")
	d.Exec("INSERT INTO rooms (name, wing_id) VALUES ('general', 1)")

	id1 := insertDrawer(t, d, "has embedding", 1, 1)
	id2 := insertDrawer(t, d, "no embedding", 1, 1)
	IndexEmbedding(d, id1, embeddings.Encode([]float32{1, 2, 3}))

	pending, err := ListDrawersWithoutEmbeddings(d, 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(pending) != 1 || pending[0].ID != id2 {
		t.Errorf("got %v, want id2 only", pending)
	}

	got, _ := CountDrawersWithEmbeddings(d)
	if got != 1 {
		t.Errorf("count: got %d want 1", got)
	}
}
