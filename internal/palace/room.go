package palace

import (
	"fmt"

	"github.com/snow-ghost/mem/internal/db"
)

type Room struct {
	ID     int64
	Name   string
	WingID int64
}

func CreateRoom(d *db.DB, name string, wingID int64) (*Room, error) {
	_, err := d.Exec(
		"INSERT OR IGNORE INTO rooms (name, wing_id) VALUES (?, ?)",
		name, wingID,
	)
	if err != nil {
		return nil, fmt.Errorf("create room: %w", err)
	}
	return GetRoom(d, name, wingID)
}

func GetRoom(d *db.DB, name string, wingID int64) (*Room, error) {
	var r Room
	err := d.QueryRow("SELECT id, name, wing_id FROM rooms WHERE name = ? AND wing_id = ?", name, wingID).
		Scan(&r.ID, &r.Name, &r.WingID)
	if err != nil {
		return nil, fmt.Errorf("get room %q: %w", name, err)
	}
	return &r, nil
}

func ListRooms(d *db.DB, wingID int64) ([]Room, error) {
	rows, err := d.Query("SELECT id, name, wing_id FROM rooms WHERE wing_id = ? ORDER BY name", wingID)
	if err != nil {
		return nil, fmt.Errorf("list rooms: %w", err)
	}
	defer rows.Close()

	var rooms []Room
	for rows.Next() {
		var r Room
		if err := rows.Scan(&r.ID, &r.Name, &r.WingID); err != nil {
			return nil, err
		}
		rooms = append(rooms, r)
	}
	return rooms, nil
}
