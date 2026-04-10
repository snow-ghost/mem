package db

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpen_GivenValidPath_WhenOpened_ThenNoError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	d, err := Open(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer d.Close()

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("database file was not created")
	}
}

func TestInitSchema_GivenNewDB_WhenInitialized_ThenTablesCreated(t *testing.T) {
	dir := t.TempDir()
	d, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer d.Close()

	if err := InitSchema(d); err != nil {
		t.Fatalf("init schema: %v", err)
	}

	tables := []string{"wings", "rooms", "drawers", "closets", "search_terms", "search_index", "search_meta", "entities", "triples"}
	for _, table := range tables {
		var count int
		err := d.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count)
		if err != nil {
			t.Errorf("table %q not created: %v", table, err)
		}
	}
}

func TestInitSchema_GivenIdempotent_WhenCalledTwice_ThenNoError(t *testing.T) {
	dir := t.TempDir()
	d, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer d.Close()

	InitSchema(d)
	if err := InitSchema(d); err != nil {
		t.Fatalf("second init should not error: %v", err)
	}
}

func OpenTestDB(t *testing.T) *DB {
	t.Helper()
	dir := t.TempDir()
	d, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := InitSchema(d); err != nil {
		t.Fatalf("init schema: %v", err)
	}
	return d
}
