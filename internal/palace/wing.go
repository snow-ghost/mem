package palace

import (
	"fmt"

	"github.com/snow-ghost/mem/internal/db"
)

type Wing struct {
	ID       int64
	Name     string
	Type     string
	Keywords string
}

func CreateWing(d *db.DB, name, wingType, keywords string) (*Wing, error) {
	if wingType == "" {
		wingType = "general"
	}
	_, err := d.Exec(
		"INSERT OR IGNORE INTO wings (name, type, keywords) VALUES (?, ?, ?)",
		name, wingType, keywords,
	)
	if err != nil {
		return nil, fmt.Errorf("create wing: %w", err)
	}
	return GetWing(d, name)
}

func GetWing(d *db.DB, name string) (*Wing, error) {
	var w Wing
	err := d.QueryRow("SELECT id, name, type, keywords FROM wings WHERE name = ?", name).
		Scan(&w.ID, &w.Name, &w.Type, &w.Keywords)
	if err != nil {
		return nil, fmt.Errorf("get wing %q: %w", name, err)
	}
	return &w, nil
}

func ListWings(d *db.DB) ([]Wing, error) {
	rows, err := d.Query("SELECT id, name, type, keywords FROM wings ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("list wings: %w", err)
	}
	defer rows.Close()

	var wings []Wing
	for rows.Next() {
		var w Wing
		if err := rows.Scan(&w.ID, &w.Name, &w.Type, &w.Keywords); err != nil {
			return nil, err
		}
		wings = append(wings, w)
	}
	return wings, nil
}
