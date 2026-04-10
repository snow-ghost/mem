package palace

import (
	"fmt"
	"os"

	"github.com/snow-ghost/mem/internal/config"
	"github.com/snow-ghost/mem/internal/db"
)

type Status struct {
	PalacePath  string
	Wings       int
	Rooms       int
	Drawers     int
	DbSizeBytes int64
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

	info, err := os.Stat(palacePath)
	if err == nil && !info.IsDir() {
		s.DbSizeBytes = info.Size()
	}

	return s, nil
}
