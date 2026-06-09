package search

import (
	"fmt"
	"testing"
)

func TestSearchScoped_GivenSessionFilter_WhenSearched_ThenOnlyMatchingSources(t *testing.T) {
	d := openTestDB(t)
	defer d.Close()

	d.Exec("INSERT INTO wings (name, type) VALUES ('bench', 'project')")
	d.Exec("INSERT INTO rooms (name, wing_id) VALUES ('general', 1)")

	mk := func(content, source string) int64 {
		drawerCounter++
		res, _ := d.Exec(
			`INSERT INTO drawers (content, content_hash, wing_id, room_id, hall, source_file, source_type)
			 VALUES (?, ?, 1, 1, 'facts', ?, 'conversation')`,
			content, fmt.Sprintf("hash_scope_%d", drawerCounter), source,
		)
		id, _ := res.LastInsertId()
		IndexDrawer(d, id, content)
		return id
	}

	inHaystack := mk("authentication oauth token lookup", "session_A")
	mk("authentication oauth token lookup from elsewhere", "session_B")
	mk("authentication oauth token lookup under different id", "session_C")

	all, err := Search(d, "authentication oauth", 0, 0, 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("unscoped: expected 3 results, got %d", len(all))
	}

	scoped, err := SearchScoped(d, "authentication oauth", 0, 0,
		Scope{SessionIDs: []string{"session_A"}}, 10)
	if err != nil {
		t.Fatalf("scoped search: %v", err)
	}
	if len(scoped) != 1 {
		t.Fatalf("scoped: expected 1 result, got %d", len(scoped))
	}
	if scoped[0].DrawerID != inHaystack {
		t.Errorf("scoped: expected drawer %d, got %d", inHaystack, scoped[0].DrawerID)
	}
	if scoped[0].SourceFile != "session_A" {
		t.Errorf("scoped: wrong source_file %q", scoped[0].SourceFile)
	}
}

func TestSearchScoped_GivenEmptyScope_WhenSearched_ThenMatchesUnscoped(t *testing.T) {
	d := openTestDB(t)
	defer d.Close()

	d.Exec("INSERT INTO wings (name, type) VALUES ('bench', 'project')")
	d.Exec("INSERT INTO rooms (name, wing_id) VALUES ('general', 1)")
	drawerCounter++
	res, _ := d.Exec(
		`INSERT INTO drawers (content, content_hash, wing_id, room_id, hall, source_file, source_type)
		 VALUES (?, ?, 1, 1, 'facts', 'session_X', 'conversation')`,
		"database migration", fmt.Sprintf("hash_scope_%d", drawerCounter),
	)
	id, _ := res.LastInsertId()
	IndexDrawer(d, id, "database migration")

	unscoped, _ := Search(d, "database migration", 0, 0, 5)
	scoped, _ := SearchScoped(d, "database migration", 0, 0, Scope{}, 5)

	if len(unscoped) != len(scoped) {
		t.Fatalf("expected empty scope to match unscoped; got %d vs %d",
			len(unscoped), len(scoped))
	}
}
