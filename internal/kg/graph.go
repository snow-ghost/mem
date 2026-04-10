package kg

import (
	"crypto/md5"
	"fmt"
	"strings"
	"time"

	"github.com/snow-ghost/mem/internal/db"
)

type Entity struct {
	ID         string
	Name       string
	Type       string
	Properties string
}

type Triple struct {
	ID         string
	Subject    string
	Predicate  string
	Object     string
	ValidFrom  string
	ValidTo    string
	Confidence float64
	Current    bool
	SubjName   string
	ObjName    string
}

func entityID(name string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(name), " ", "_"))
}

func AddEntity(d *db.DB, name, etype string, props string) (string, error) {
	id := entityID(name)
	if props == "" {
		props = "{}"
	}
	_, err := d.Exec("INSERT OR REPLACE INTO entities (id, name, type, properties) VALUES (?, ?, ?, ?)",
		id, name, etype, props)
	if err != nil {
		return "", fmt.Errorf("add entity: %w", err)
	}
	return id, nil
}

func AddTriple(d *db.DB, subject, predicate, object, validFrom, validTo string) (string, error) {
	subID := entityID(subject)
	objID := entityID(object)
	pred := strings.ToLower(strings.ReplaceAll(predicate, " ", "_"))

	d.Exec("INSERT OR IGNORE INTO entities (id, name) VALUES (?, ?)", subID, subject)
	d.Exec("INSERT OR IGNORE INTO entities (id, name) VALUES (?, ?)", objID, object)

	// Check existing
	var existing string
	err := d.QueryRow(
		"SELECT id FROM triples WHERE subject=? AND predicate=? AND object=? AND valid_to IS NULL",
		subID, pred, objID,
	).Scan(&existing)
	if err == nil {
		return existing, nil
	}

	tripleID := fmt.Sprintf("t_%x", md5.Sum([]byte(fmt.Sprintf("%s%s%s%s", subID, pred, objID, time.Now().String()))))

	var vFrom, vTo any
	if validFrom != "" {
		vFrom = validFrom
	}
	if validTo != "" {
		vTo = validTo
	}
	_, err = d.Exec(
		`INSERT INTO triples (id, subject, predicate, object, valid_from, valid_to, confidence)
		 VALUES (?, ?, ?, ?, ?, ?, 1.0)`,
		tripleID, subID, pred, objID, vFrom, vTo,
	)
	if err != nil {
		return "", fmt.Errorf("add triple: %w", err)
	}
	return tripleID, nil
}

func Invalidate(d *db.DB, subject, predicate, object, ended string) error {
	subID := entityID(subject)
	objID := entityID(object)
	pred := strings.ToLower(strings.ReplaceAll(predicate, " ", "_"))
	if ended == "" {
		ended = time.Now().Format("2006-01-02")
	}
	_, err := d.Exec(
		"UPDATE triples SET valid_to=? WHERE subject=? AND predicate=? AND object=? AND valid_to IS NULL",
		ended, subID, pred, objID,
	)
	if err != nil {
		return fmt.Errorf("invalidate: %w", err)
	}
	return nil
}

func QueryEntity(d *db.DB, name, asOf, direction string) ([]Triple, error) {
	eid := entityID(name)
	if direction == "" {
		direction = "both"
	}
	var results []Triple

	if direction == "outgoing" || direction == "both" {
		query := `SELECT t.id, t.subject, t.predicate, t.object, COALESCE(t.valid_from,''), COALESCE(t.valid_to,''), t.confidence, e.name
			FROM triples t JOIN entities e ON t.object = e.id WHERE t.subject = ?`
		args := []any{eid}
		if asOf != "" {
			query += " AND (t.valid_from IS NULL OR t.valid_from <= ?) AND (t.valid_to IS NULL OR t.valid_to >= ?)"
			args = append(args, asOf, asOf)
		}
		rows, err := d.Query(query, args...)
		if err == nil {
			for rows.Next() {
				var tr Triple
				rows.Scan(&tr.ID, &tr.Subject, &tr.Predicate, &tr.Object, &tr.ValidFrom, &tr.ValidTo, &tr.Confidence, &tr.ObjName)
				tr.SubjName = name
				tr.Current = tr.ValidTo == ""
				results = append(results, tr)
			}
			rows.Close()
		}
	}

	if direction == "incoming" || direction == "both" {
		query := `SELECT t.id, t.subject, t.predicate, t.object, COALESCE(t.valid_from,''), COALESCE(t.valid_to,''), t.confidence, e.name
			FROM triples t JOIN entities e ON t.subject = e.id WHERE t.object = ?`
		args := []any{eid}
		if asOf != "" {
			query += " AND (t.valid_from IS NULL OR t.valid_from <= ?) AND (t.valid_to IS NULL OR t.valid_to >= ?)"
			args = append(args, asOf, asOf)
		}
		rows, err := d.Query(query, args...)
		if err == nil {
			for rows.Next() {
				var tr Triple
				rows.Scan(&tr.ID, &tr.Subject, &tr.Predicate, &tr.Object, &tr.ValidFrom, &tr.ValidTo, &tr.Confidence, &tr.SubjName)
				tr.ObjName = name
				tr.Current = tr.ValidTo == ""
				results = append(results, tr)
			}
			rows.Close()
		}
	}

	return results, nil
}

func Timeline(d *db.DB, entity string) ([]Triple, error) {
	eid := entityID(entity)
	rows, err := d.Query(`
		SELECT t.id, t.subject, t.predicate, t.object, COALESCE(t.valid_from,''), COALESCE(t.valid_to,''), t.confidence,
			s.name, o.name
		FROM triples t
		JOIN entities s ON t.subject = s.id
		JOIN entities o ON t.object = o.id
		WHERE t.subject = ? OR t.object = ?
		ORDER BY t.valid_from ASC NULLS LAST`, eid, eid)
	if err != nil {
		return nil, fmt.Errorf("timeline: %w", err)
	}
	defer rows.Close()

	var results []Triple
	for rows.Next() {
		var tr Triple
		rows.Scan(&tr.ID, &tr.Subject, &tr.Predicate, &tr.Object, &tr.ValidFrom, &tr.ValidTo, &tr.Confidence, &tr.SubjName, &tr.ObjName)
		tr.Current = tr.ValidTo == ""
		results = append(results, tr)
	}
	return results, nil
}

func Stats(d *db.DB) (entities, triples, current int, err error) {
	d.QueryRow("SELECT COUNT(*) FROM entities").Scan(&entities)
	d.QueryRow("SELECT COUNT(*) FROM triples").Scan(&triples)
	d.QueryRow("SELECT COUNT(*) FROM triples WHERE valid_to IS NULL").Scan(&current)
	return
}
