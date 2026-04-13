package palace

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/snow-ghost/mem/internal/config"
	"github.com/snow-ghost/mem/internal/db"
)

type WingStat struct {
	Name             string
	Type             string
	Drawers          int
	DrawersEmbedded  int
}

type HNSWStat struct {
	Name        string
	DrawerCount int
	Dim         int
	BlobBytes   int64
	UpdatedAt   string
}

type Status struct {
	PalacePath        string
	Wings             int
	Rooms             int
	Drawers           int
	DrawersEmbedded   int
	Triples           int
	DbSizeBytes       int64
	WingBreakdown     []WingStat
	HNSWCaches        []HNSWStat
}

func Init(cfg config.Config) (*db.DB, error) {
	if err := os.MkdirAll(cfg.PalacePath, 0755); err != nil {
		return nil, fmt.Errorf("create palace dir: %w", err)
	}
	d, err := db.Open(cfg.DbPath())
	if err != nil {
		return nil, err
	}
	if err := db.InitSchema(d); err != nil {
		d.Close()
		return nil, err
	}
	return d, nil
}

func GetStatus(d *db.DB, palacePath string) (*Status, error) {
	s := &Status{PalacePath: palacePath}

	d.QueryRow("SELECT COUNT(*) FROM wings").Scan(&s.Wings)
	d.QueryRow("SELECT COUNT(*) FROM rooms").Scan(&s.Rooms)
	d.QueryRow("SELECT COUNT(*) FROM drawers").Scan(&s.Drawers)
	d.QueryRow("SELECT COUNT(*) FROM drawers WHERE embedding IS NOT NULL").Scan(&s.DrawersEmbedded)
	d.QueryRow("SELECT COUNT(*) FROM triples").Scan(&s.Triples)

	if info, err := os.Stat(filepath.Join(palacePath, "palace.db")); err == nil {
		s.DbSizeBytes = info.Size()
	}

	// Per-wing breakdown
	rows, err := d.Query(`SELECT w.name, w.type, COUNT(d.id),
		SUM(CASE WHEN d.embedding IS NOT NULL THEN 1 ELSE 0 END)
		FROM wings w
		LEFT JOIN drawers d ON d.wing_id = w.id
		GROUP BY w.id ORDER BY w.name`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var ws WingStat
			var emb *int
			if err := rows.Scan(&ws.Name, &ws.Type, &ws.Drawers, &emb); err != nil {
				continue
			}
			if emb != nil {
				ws.DrawersEmbedded = *emb
			}
			s.WingBreakdown = append(s.WingBreakdown, ws)
		}
	}

	// HNSW cache entries (table may not exist on very old palaces)
	hrows, err := d.Query(`SELECT name, drawer_count, dim, length(blob), updated_at FROM hnsw_cache ORDER BY name`)
	if err == nil {
		defer hrows.Close()
		for hrows.Next() {
			var h HNSWStat
			if err := hrows.Scan(&h.Name, &h.DrawerCount, &h.Dim, &h.BlobBytes, &h.UpdatedAt); err != nil {
				continue
			}
			s.HNSWCaches = append(s.HNSWCaches, h)
		}
	}

	return s, nil
}
