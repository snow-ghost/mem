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

func ContentHash(content string) string {
	h := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", h)
}

func AddDrawer(d *db.DB, content string, wingID, roomID int64, hall, sourceFile, sourceType string) (*Drawer, error) {
	if hall == "" {
		hall = "facts"
	}
	if sourceType == "" {
		sourceType = "file"
	}
	hash := ContentHash(content)

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
