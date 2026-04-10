package palace

import (
	"fmt"

	"github.com/snow-ghost/mem/internal/db"
)

type Tunnel struct {
	Room  string
	Wings []string
	Count int
}

func FindTunnels(d *db.DB, wingA, wingB string) ([]Tunnel, error) {
	query := `
		SELECT r.name, GROUP_CONCAT(DISTINCT w.name), COUNT(DISTINCT d.id)
		FROM rooms r
		JOIN wings w ON r.wing_id = w.id
		JOIN drawers d ON d.room_id = r.id
		WHERE r.name IN (
			SELECT name FROM rooms GROUP BY name HAVING COUNT(DISTINCT wing_id) > 1
		)
	`
	var args []any
	if wingA != "" {
		query += " AND r.name IN (SELECT name FROM rooms JOIN wings ON rooms.wing_id = wings.id WHERE wings.name = ?)"
		args = append(args, wingA)
	}
	if wingB != "" {
		query += " AND r.name IN (SELECT name FROM rooms JOIN wings ON rooms.wing_id = wings.id WHERE wings.name = ?)"
		args = append(args, wingB)
	}
	query += " GROUP BY r.name ORDER BY COUNT(DISTINCT d.id) DESC"

	rows, err := d.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("find tunnels: %w", err)
	}
	defer rows.Close()

	var tunnels []Tunnel
	for rows.Next() {
		var t Tunnel
		var wingsCSV string
		if err := rows.Scan(&t.Room, &wingsCSV, &t.Count); err != nil {
			return nil, err
		}
		for _, w := range splitCSV(wingsCSV) {
			t.Wings = append(t.Wings, w)
		}
		tunnels = append(tunnels, t)
	}
	return tunnels, nil
}

func ListTunnels(d *db.DB) ([]Tunnel, error) {
	return FindTunnels(d, "", "")
}

func splitCSV(s string) []string {
	var parts []string
	current := ""
	for _, c := range s {
		if c == ',' {
			if current != "" {
				parts = append(parts, current)
			}
			current = ""
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}
