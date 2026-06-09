package palace

import (
	"path/filepath"
	"testing"

	"github.com/snow-ghost/mem/internal/config"
	"github.com/snow-ghost/mem/internal/db"
)

func setupPalace(t *testing.T) (*db.DB, config.Config) {
	t.Helper()
	dir := t.TempDir()
	cfg := config.Config{PalacePath: dir, DbFile: "test.db"}
	d, err := Init(cfg)
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	return d, cfg
}

func TestInit_GivenEmptyDir_WhenInit_ThenDbCreated(t *testing.T) {
	d, cfg := setupPalace(t)
	defer d.Close()

	s, _ := GetStatus(d, cfg.PalacePath)
	if s.Wings != 0 || s.Rooms != 0 || s.Drawers != 0 {
		t.Errorf("expected all zeros, got W=%d R=%d D=%d", s.Wings, s.Rooms, s.Drawers)
	}
}

func TestWing_GivenCreate_WhenListed_ThenPresent(t *testing.T) {
	d, _ := setupPalace(t)
	defer d.Close()

	CreateWing(d, "myapp", "project", "go,web")
	wings, _ := ListWings(d)
	if len(wings) != 1 || wings[0].Name != "myapp" {
		t.Errorf("expected 1 wing 'myapp', got %v", wings)
	}
}

func TestWing_GivenIdempotent_WhenCreatedTwice_ThenOneWing(t *testing.T) {
	d, _ := setupPalace(t)
	defer d.Close()

	CreateWing(d, "myapp", "project", "")
	CreateWing(d, "myapp", "project", "")
	wings, _ := ListWings(d)
	if len(wings) != 1 {
		t.Errorf("expected 1 wing, got %d", len(wings))
	}
}

func TestRoom_GivenCreate_WhenListed_ThenPresent(t *testing.T) {
	d, _ := setupPalace(t)
	defer d.Close()

	w, _ := CreateWing(d, "app", "project", "")
	CreateRoom(d, "auth", w.ID)
	CreateRoom(d, "deploy", w.ID)

	rooms, _ := ListRooms(d, w.ID)
	if len(rooms) != 2 {
		t.Errorf("expected 2 rooms, got %d", len(rooms))
	}
}

func TestDrawer_GivenAdd_WhenRetrieved_ThenContentMatches(t *testing.T) {
	d, _ := setupPalace(t)
	defer d.Close()

	w, _ := CreateWing(d, "app", "project", "")
	r, _ := CreateRoom(d, "auth", w.ID)

	dr, err := AddDrawer(d, "OAuth implementation details", w.ID, r.ID, "facts", "auth.go", "file")
	if err != nil {
		t.Fatalf("add drawer: %v", err)
	}
	if dr == nil {
		t.Fatal("expected drawer, got nil")
	}

	got, _ := GetDrawer(d, dr.ID)
	if got.Content != "OAuth implementation details" {
		t.Errorf("content = %q", got.Content)
	}
}

func TestDrawer_GivenSameContentSameSource_WhenAddedTwice_ThenDeduped(t *testing.T) {
	d, _ := setupPalace(t)
	defer d.Close()

	w, _ := CreateWing(d, "app", "project", "")
	r, _ := CreateRoom(d, "auth", w.ID)

	AddDrawer(d, "same content", w.ID, r.ID, "facts", "a.go", "file")
	dup, _ := AddDrawer(d, "same content", w.ID, r.ID, "facts", "a.go", "file")
	if dup != nil {
		t.Error("expected nil for same (content, source, hall) duplicate, got drawer")
	}

	count, _ := CountDrawers(d)
	if count != 1 {
		t.Errorf("expected 1 drawer, got %d", count)
	}
}

func TestDrawer_GivenSameContentDifferentSource_WhenAdded_ThenBothStored(t *testing.T) {
	d, _ := setupPalace(t)
	defer d.Close()

	w, _ := CreateWing(d, "app", "project", "")
	r, _ := CreateRoom(d, "auth", w.ID)

	first, _ := AddDrawer(d, "same content", w.ID, r.ID, "facts", "a.go", "file")
	second, err := AddDrawer(d, "same content", w.ID, r.ID, "facts", "b.go", "file")
	if err != nil {
		t.Fatalf("add drawer: %v", err)
	}
	if first == nil || second == nil {
		t.Fatal("expected both drawers stored, got nil")
	}
	if first.ID == second.ID {
		t.Errorf("expected distinct drawer IDs, got %d/%d", first.ID, second.ID)
	}

	count, _ := CountDrawers(d)
	if count != 2 {
		t.Errorf("expected 2 drawers, got %d", count)
	}
}

func TestAddDrawerVariants_GivenThreeViews_WhenAdded_ThenEachHallStored(t *testing.T) {
	d, _ := setupPalace(t)
	defer d.Close()

	w, _ := CreateWing(d, "app", "project", "")
	r, _ := CreateRoom(d, "general", w.ID)

	views := map[string]string{
		"L0": "full session: user asks about auth, assistant explains oauth",
		"L1": "asks about auth",
		"L2": "asks about auth",
	}
	drawers, err := AddDrawerVariants(d, views, w.ID, r.ID, "session_42", "conversation")
	if err != nil {
		t.Fatalf("variants: %v", err)
	}
	for hall := range views {
		if drawers[hall] == nil {
			t.Errorf("expected drawer for hall %q, got nil", hall)
		}
	}

	count, _ := CountDrawers(d)
	// L1 and L2 have identical content+source+hall-L2 vs hall-L1 is different
	// so we expect 3 distinct drawers (L0 distinct text, L1 and L2 same text
	// but different hall → different hash).
	if count != 3 {
		t.Errorf("expected 3 drawers for 3 halls, got %d", count)
	}
}

func TestDrawer_GivenSameContentDifferentHall_WhenAdded_ThenBothStored(t *testing.T) {
	d, _ := setupPalace(t)
	defer d.Close()

	w, _ := CreateWing(d, "app", "project", "")
	r, _ := CreateRoom(d, "auth", w.ID)

	AddDrawer(d, "same content", w.ID, r.ID, "facts", "a.go", "file")
	second, err := AddDrawer(d, "same content", w.ID, r.ID, "events", "a.go", "file")
	if err != nil {
		t.Fatalf("add drawer: %v", err)
	}
	if second == nil {
		t.Fatal("expected drawer with different hall stored, got nil")
	}

	count, _ := CountDrawers(d)
	if count != 2 {
		t.Errorf("expected 2 drawers, got %d", count)
	}
}

// === BENCHMARKS ===

func BenchmarkAddDrawer(b *testing.B) {
	dir := b.TempDir()
	cfg := config.Config{PalacePath: dir, DbFile: "bench.db"}
	d, _ := Init(cfg)
	defer d.Close()

	w, _ := CreateWing(d, "bench", "project", "")
	r, _ := CreateRoom(d, "general", w.ID)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		content := filepath.Join("content", string(rune('A'+i%26)), string(rune('0'+i%10)))
		AddDrawer(d, content+" some benchmark text for testing drawer insertion", w.ID, r.ID, "facts", "bench.go", "file")
	}
}

func BenchmarkMine100Files(b *testing.B) {
	// Simulate mining by creating drawers + indexing
	dir := b.TempDir()
	cfg := config.Config{PalacePath: dir, DbFile: "bench.db"}
	d, _ := Init(cfg)
	defer d.Close()

	w, _ := CreateWing(d, "bench", "project", "")
	r, _ := CreateRoom(d, "general", w.ID)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < 100; j++ {
			content := "File content about various topics including database deployment and testing for benchmark purposes iteration"
			AddDrawer(d, content+string(rune('A'+j%26))+string(rune('0'+i%10)), w.ID, r.ID, "facts", "bench.go", "file")
		}
	}
}
