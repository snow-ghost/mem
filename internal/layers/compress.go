package layers

import (
	"fmt"
	"strings"

	"github.com/snow-ghost/mem/internal/db"
)

func CompressL1(d *db.DB, wingFilter string) string {
	// Gather wing summaries
	wingQuery := "SELECT name, type, keywords FROM wings ORDER BY name"
	rows, err := d.Query(wingQuery)
	if err != nil {
		return "## L1 — No data"
	}
	defer rows.Close()

	var lines []string
	lines = append(lines, "## L1 — CRITICAL FACTS")

	for rows.Next() {
		var name, wtype, keywords string
		rows.Scan(&name, &wtype, &keywords)
		if wingFilter != "" && name != wingFilter {
			continue
		}
		entry := fmt.Sprintf("%s(%s)", strings.ToUpper(name), wtype)
		if keywords != "" {
			entry += " | " + keywords
		}
		lines = append(lines, entry)

		// Top rooms for this wing
		var wingID int64
		d.QueryRow("SELECT id FROM wings WHERE name = ?", name).Scan(&wingID)
		roomRows, err := d.Query(`
			SELECT r.name, COUNT(d.id) as cnt
			FROM rooms r JOIN drawers d ON d.room_id = r.id
			WHERE r.wing_id = ?
			GROUP BY r.name ORDER BY cnt DESC LIMIT 5`, wingID)
		if err != nil {
			continue
		}
		var roomParts []string
		for roomRows.Next() {
			var rname string
			var cnt int
			roomRows.Scan(&rname, &cnt)
			roomParts = append(roomParts, fmt.Sprintf("%s(%d)", rname, cnt))
		}
		roomRows.Close()
		if len(roomParts) > 0 {
			lines = append(lines, "  ROOMS: "+strings.Join(roomParts, " | "))
		}
	}

	// Recent drawers
	recentRows, err := d.Query(`
		SELECT d.hall, d.source_file, w.name, r.name
		FROM drawers d
		JOIN wings w ON d.wing_id = w.id
		JOIN rooms r ON d.room_id = r.id
		ORDER BY d.created_at DESC LIMIT 5`)
	if err == nil {
		lines = append(lines, "RECENT:")
		for recentRows.Next() {
			var hall, source, wing, room string
			recentRows.Scan(&hall, &source, &wing, &room)
			lines = append(lines, fmt.Sprintf("  %s/%s [%s] %s", wing, room, hall, source))
		}
		recentRows.Close()
	}

	return strings.Join(lines, "\n")
}
