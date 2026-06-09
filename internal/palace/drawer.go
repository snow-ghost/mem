package palace

import (
	"crypto/sha256"
	"fmt"

	"github.com/snow-ghost/mem/internal/db"
)

type Drawer struct {
	ID          int64
	Content     string
	ContentHash string
	WingID      int64
	RoomID      int64
	Hall        string
	SourceFile  string
	SourceType  string
}

// ContentHash derives the dedup key for a drawer.
//
// The hash spans (content, sourceFile, hall) — not content alone — so that
// two drawers with identical text but different origin coexist. A content-
// only hash collapses ~23% of LongMemEval's `_s_cleaned` sessions under
// the same answer session-id; real `mem mine` users hit the same issue
// when re-importing exports with duplicated chunks. The column-level
// UNIQUE on content_hash stays correct under the new formula — same
// (content, source, hall) still dedupes as before.
func ContentHash(content, sourceFile, hall string) string {
	h := sha256.Sum256([]byte(content + "\x00" + sourceFile + "\x00" + hall))
	return fmt.Sprintf("%x", h)
}

func AddDrawer(d *db.DB, content string, wingID, roomID int64, hall, sourceFile, sourceType string) (*Drawer, error) {
	if hall == "" {
		hall = "facts"
	}
	if sourceType == "" {
		sourceType = "file"
	}
	hash := ContentHash(content, sourceFile, hall)

	res, err := d.Exec(
		`INSERT OR IGNORE INTO drawers (content, content_hash, wing_id, room_id, hall, source_file, source_type)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		content, hash, wingID, roomID, hall, sourceFile, sourceType,
	)
	if err != nil {
		return nil, fmt.Errorf("add drawer: %w", err)
	}

	affected, _ := res.RowsAffected()
	if affected == 0 {
		return nil, nil // duplicate, already exists
	}

	id, _ := res.LastInsertId()
	return &Drawer{
		ID: id, Content: content, ContentHash: hash,
		WingID: wingID, RoomID: roomID, Hall: hall,
		SourceFile: sourceFile, SourceType: sourceType,
	}, nil
}

// AddDrawerVariants stores several hall-tagged projections of the same
// logical item under one source_file. It is the ingestion half of
// Schift's "L# Cache" technique: pair with search.SearchByLCache to get
// one result per source ranked by the best-matching variant.
//
// Typical use: for a chat session keyed by session_id, pass
//
//	{"L0": full, "L1": userTurnsOnly, "L2": firstThreeUserTurns}
//
// all three share source_file = session_id; content_hash includes hall
// so the UNIQUE constraint does not collapse them. Variants that already
// exist (same content, source, hall) are skipped and surface as nil.
// A nil or empty views map is a no-op.
func AddDrawerVariants(d *db.DB, views map[string]string, wingID, roomID int64, sourceFile, sourceType string) (map[string]*Drawer, error) {
	if len(views) == 0 {
		return nil, nil
	}
	out := make(map[string]*Drawer, len(views))
	for hall, content := range views {
		dr, err := AddDrawer(d, content, wingID, roomID, hall, sourceFile, sourceType)
		if err != nil {
			return out, fmt.Errorf("add variant %q: %w", hall, err)
		}
		out[hall] = dr
	}
	return out, nil
}

func GetDrawer(d *db.DB, id int64) (*Drawer, error) {
	var dr Drawer
	err := d.QueryRow(
		"SELECT id, content, content_hash, wing_id, room_id, hall, source_file, source_type FROM drawers WHERE id = ?", id,
	).Scan(&dr.ID, &dr.Content, &dr.ContentHash, &dr.WingID, &dr.RoomID, &dr.Hall, &dr.SourceFile, &dr.SourceType)
	if err != nil {
		return nil, fmt.Errorf("get drawer: %w", err)
	}
	return &dr, nil
}

func ListDrawers(d *db.DB, wingID, roomID int64) ([]Drawer, error) {
	query := "SELECT id, content, content_hash, wing_id, room_id, hall, source_file, source_type FROM drawers WHERE 1=1"
	var args []any
	if wingID > 0 {
		query += " AND wing_id = ?"
		args = append(args, wingID)
	}
	if roomID > 0 {
		query += " AND room_id = ?"
		args = append(args, roomID)
	}
	query += " ORDER BY created_at DESC"

	rows, err := d.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list drawers: %w", err)
	}
	defer rows.Close()

	var drawers []Drawer
	for rows.Next() {
		var dr Drawer
		if err := rows.Scan(&dr.ID, &dr.Content, &dr.ContentHash, &dr.WingID, &dr.RoomID, &dr.Hall, &dr.SourceFile, &dr.SourceType); err != nil {
			return nil, err
		}
		drawers = append(drawers, dr)
	}
	return drawers, nil
}

func CountDrawers(d *db.DB) (int, error) {
	var count int
	err := d.QueryRow("SELECT COUNT(*) FROM drawers").Scan(&count)
	return count, err
}
