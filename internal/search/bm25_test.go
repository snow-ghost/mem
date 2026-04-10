package search

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/snow-ghost/mem/internal/db"
)

func openTestDB(t testing.TB) *db.DB {
	t.Helper()
	dir := t.TempDir()
	d, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	db.InitSchema(d)
	return d
}

var drawerCounter int64

func insertDrawer(t testing.TB, d *db.DB, content string, wingID, roomID int64) int64 {
	t.Helper()
	drawerCounter++
	res, err := d.Exec(
		`INSERT INTO drawers (content, content_hash, wing_id, room_id, hall, source_file, source_type)
		 VALUES (?, ?, ?, ?, 'facts', 'test.txt', 'file')`,
		content, fmt.Sprintf("hash_%d_%d", drawerCounter, wingID), wingID, roomID,
	)
	if err != nil {
		t.Fatalf("insert drawer: %v", err)
	}
	id, _ := res.LastInsertId()
	return id
}

func TestSearch_GivenIndexedDrawers_WhenSearched_ThenRelevantFirst(t *testing.T) {
	d := openTestDB(t)
	defer d.Close()

	d.Exec("INSERT INTO wings (name, type) VALUES ('test', 'project')")
	d.Exec("INSERT INTO rooms (name, wing_id) VALUES ('auth', 1)")
	d.Exec("INSERT INTO rooms (name, wing_id) VALUES ('deploy', 1)")

	// Auth-related content
	id1 := insertDrawer(t, d, "OAuth authentication login security tokens JWT bearer", 1, 1)
	IndexDrawer(d, id1, "OAuth authentication login security tokens JWT bearer")

	// Deploy-related content
	id2 := insertDrawer(t, d, "kubernetes deployment pipeline CI CD docker containers", 1, 2)
	IndexDrawer(d, id2, "kubernetes deployment pipeline CI CD docker containers")

	// Mixed content
	id3 := insertDrawer(t, d, "the project has both authentication and deployment needs", 1, 1)
	IndexDrawer(d, id3, "the project has both authentication and deployment needs")

	results, err := Search(d, "authentication OAuth login", 0, 0, 5)
	if err != nil {
		t.Fatalf("search error: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected results")
	}
	if results[0].DrawerID != id1 {
		t.Errorf("expected auth drawer first, got drawer %d", results[0].DrawerID)
	}
}

func TestSearch_GivenWingFilter_WhenSearched_ThenOnlyWingResults(t *testing.T) {
	d := openTestDB(t)
	defer d.Close()

	d.Exec("INSERT INTO wings (name, type) VALUES ('app1', 'project')")
	d.Exec("INSERT INTO wings (name, type) VALUES ('app2', 'project')")
	d.Exec("INSERT INTO rooms (name, wing_id) VALUES ('general', 1)")
	d.Exec("INSERT INTO rooms (name, wing_id) VALUES ('general', 2)")

	id1 := insertDrawer(t, d, "database migration PostgreSQL schema", 1, 1)
	IndexDrawer(d, id1, "database migration PostgreSQL schema")

	res2, _ := d.Exec(
		`INSERT INTO drawers (content, content_hash, wing_id, room_id, hall, source_file, source_type)
		 VALUES (?, 'hash_2', 2, 2, 'facts', 'test.txt', 'file')`,
		"database migration PostgreSQL schema different project",
	)
	id2, _ := res2.LastInsertId()
	IndexDrawer(d, id2, "database migration PostgreSQL schema different project")

	results, _ := Search(d, "database migration", 1, 0, 5) // wing 1 only
	for _, r := range results {
		if r.WingID != 1 {
			t.Errorf("got result from wing %d, want only wing 1", r.WingID)
		}
	}
}

func TestSearch_GivenEmptyQuery_WhenSearched_ThenNoResults(t *testing.T) {
	d := openTestDB(t)
	defer d.Close()
	results, _ := Search(d, "", 0, 0, 5)
	if len(results) != 0 {
		t.Errorf("expected 0 results for empty query, got %d", len(results))
	}
}

func TestSearch_GivenNoMatches_WhenSearched_ThenEmptyResults(t *testing.T) {
	d := openTestDB(t)
	defer d.Close()

	d.Exec("INSERT INTO wings (name, type) VALUES ('test', 'project')")
	d.Exec("INSERT INTO rooms (name, wing_id) VALUES ('general', 1)")
	id := insertDrawer(t, d, "golang programming language", 1, 1)
	IndexDrawer(d, id, "golang programming language")

	results, _ := Search(d, "quantum physics chemistry", 0, 0, 5)
	if len(results) != 0 {
		t.Errorf("expected 0 results for unrelated query, got %d", len(results))
	}
}

// === BENCHMARKS ===

func BenchmarkIndexDrawer(b *testing.B) {
	d := openTestDB(b)
	defer d.Close()

	d.Exec("INSERT INTO wings (name, type) VALUES ('bench', 'project')")
	d.Exec("INSERT INTO rooms (name, wing_id) VALUES ('general', 1)")

	content := "The quick brown fox jumps over the lazy dog. " +
		"This is a benchmark test for indexing performance. " +
		"We need to measure how fast the BM25 indexer can process documents. " +
		"Database migrations are important for schema management."

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c := fmt.Sprintf("%s iteration %d", content, i)
		res, _ := d.Exec(
			`INSERT INTO drawers (content, content_hash, wing_id, room_id, hall, source_file, source_type)
			 VALUES (?, ?, 1, 1, 'facts', 'bench.txt', 'file')`,
			c, fmt.Sprintf("bench_hash_%d", i),
		)
		id, _ := res.LastInsertId()
		IndexDrawer(d, id, c)
	}
}

func BenchmarkSearch_10Docs(b *testing.B) {
	benchSearch(b, 10)
}

func BenchmarkSearch_100Docs(b *testing.B) {
	benchSearch(b, 100)
}

func BenchmarkSearch_1000Docs(b *testing.B) {
	benchSearch(b, 1000)
}

func BenchmarkSearch_10000Docs(b *testing.B) {
	benchSearch(b, 10000)
}

func benchSearch(b *testing.B, numDocs int) {
	d := openTestDB(b)
	defer d.Close()

	d.Exec("INSERT INTO wings (name, type) VALUES ('bench', 'project')")
	d.Exec("INSERT INTO rooms (name, wing_id) VALUES ('general', 1)")

	topics := []string{
		"authentication OAuth JWT tokens login security",
		"database migration PostgreSQL schema upgrade",
		"kubernetes deployment CI pipeline docker",
		"frontend React components rendering performance",
		"API endpoint REST GraphQL design patterns",
		"caching Redis memcached performance optimization",
		"logging monitoring observability alerting",
		"testing unit integration benchmark coverage",
		"configuration environment variables secrets",
		"networking DNS load balancing proxy reverse",
	}

	for i := 0; i < numDocs; i++ {
		content := fmt.Sprintf("Document %d about %s with additional context and details for realistic search benchmarking purposes.", i, topics[i%len(topics)])
		res, _ := d.Exec(
			`INSERT INTO drawers (content, content_hash, wing_id, room_id, hall, source_file, source_type)
			 VALUES (?, ?, 1, 1, 'facts', 'bench.txt', 'file')`,
			content, fmt.Sprintf("hash_%d", i),
		)
		id, _ := res.LastInsertId()
		IndexDrawer(d, id, content)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Search(d, "authentication OAuth login", 0, 0, 5)
	}
}

func BenchmarkSearch_WithWingFilter(b *testing.B) {
	d := openTestDB(b)
	defer d.Close()

	for w := 1; w <= 5; w++ {
		d.Exec("INSERT INTO wings (name, type) VALUES (?, 'project')", fmt.Sprintf("wing%d", w))
		d.Exec("INSERT INTO rooms (name, wing_id) VALUES ('general', ?)", w)
	}

	for i := 0; i < 1000; i++ {
		wingID := (i % 5) + 1
		content := fmt.Sprintf("Content about various topics including authentication deployment and testing iteration %d", i)
		res, _ := d.Exec(
			`INSERT INTO drawers (content, content_hash, wing_id, room_id, hall, source_file, source_type)
			 VALUES (?, ?, ?, ?, 'facts', 'bench.txt', 'file')`,
			content, fmt.Sprintf("hash_%d", i), wingID, wingID,
		)
		id, _ := res.LastInsertId()
		IndexDrawer(d, id, content)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Search(d, "authentication deployment", 1, 0, 5) // filtered to wing1
	}
}

func BenchmarkTokenize(b *testing.B) {
	text := "The quick brown fox jumps over the lazy dog. This is a sample text for tokenization benchmarking with stopwords and various punctuation marks! Does it work well?"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Tokenize(text)
	}
}
