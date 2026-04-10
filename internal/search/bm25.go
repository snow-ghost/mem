package search

import (
	"database/sql"
	"fmt"
	"math"
	"strconv"

	"github.com/snow-ghost/mem/internal/db"
)

const (
	bm25K1 = 1.5
	bm25B  = 0.75
)

type SearchResult struct {
	DrawerID   int64
	Content    string
	WingID     int64
	RoomID     int64
	Hall       string
	SourceFile string
	Score      float64
	WingName   string
	RoomName   string
}

func IndexDrawer(d *db.DB, drawerID int64, content string) error {
	tokens := Tokenize(content)
	if len(tokens) == 0 {
		return nil
	}
	tf := TokenFrequency(tokens)

	tx, err := d.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	termStmt, err := tx.Prepare("INSERT OR IGNORE INTO search_terms (term) VALUES (?)")
	if err != nil {
		return fmt.Errorf("prepare term stmt: %w", err)
	}
	defer termStmt.Close()

	selectStmt, err := tx.Prepare("SELECT id FROM search_terms WHERE term = ?")
	if err != nil {
		return fmt.Errorf("prepare select stmt: %w", err)
	}
	defer selectStmt.Close()

	indexStmt, err := tx.Prepare("INSERT OR REPLACE INTO search_index (term_id, drawer_id, tf) VALUES (?, ?, ?)")
	if err != nil {
		return fmt.Errorf("prepare index stmt: %w", err)
	}
	defer indexStmt.Close()

	for term, freq := range tf {
		termStmt.Exec(term)
		var termID int64
		selectStmt.QueryRow(term).Scan(&termID)
		indexStmt.Exec(termID, drawerID, freq)
	}

	updateMetaTx(tx, len(tokens))

	return tx.Commit()
}

func updateMetaTx(tx *sql.Tx, docLen int) {
	var totalDocs int
	var avgDocLen float64
	tx.QueryRow("SELECT value FROM search_meta WHERE key = 'total_docs'").Scan(&totalDocs)
	tx.QueryRow("SELECT value FROM search_meta WHERE key = 'avg_doc_len'").Scan(&avgDocLen)

	newTotal := totalDocs + 1
	newAvg := (avgDocLen*float64(totalDocs) + float64(docLen)) / float64(newTotal)

	tx.Exec("INSERT OR REPLACE INTO search_meta (key, value) VALUES ('total_docs', ?)", strconv.Itoa(newTotal))
	tx.Exec("INSERT OR REPLACE INTO search_meta (key, value) VALUES ('avg_doc_len', ?)", strconv.FormatFloat(newAvg, 'f', 4, 64))
}

// IndexBatch indexes multiple drawers in a single transaction.
func IndexBatch(d *db.DB, items []struct{ ID int64; Content string }) error {
	if len(items) == 0 {
		return nil
	}

	tx, err := d.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	termStmt, _ := tx.Prepare("INSERT OR IGNORE INTO search_terms (term) VALUES (?)")
	defer termStmt.Close()
	selectStmt, _ := tx.Prepare("SELECT id FROM search_terms WHERE term = ?")
	defer selectStmt.Close()
	indexStmt, _ := tx.Prepare("INSERT OR REPLACE INTO search_index (term_id, drawer_id, tf) VALUES (?, ?, ?)")
	defer indexStmt.Close()

	for _, item := range items {
		tokens := Tokenize(item.Content)
		if len(tokens) == 0 {
			continue
		}
		tf := TokenFrequency(tokens)
		for term, freq := range tf {
			termStmt.Exec(term)
			var termID int64
			selectStmt.QueryRow(term).Scan(&termID)
			indexStmt.Exec(termID, item.ID, freq)
		}
		updateMetaTx(tx, len(tokens))
	}

	return tx.Commit()
}

func Search(d *db.DB, query string, wingID, roomID int64, limit int) ([]SearchResult, error) {
	if limit <= 0 {
		limit = 5
	}
	queryTokens := Tokenize(query)
	if len(queryTokens) == 0 {
		return nil, nil
	}

	var totalDocs int
	var avgDocLen float64
	d.QueryRow("SELECT CAST(value AS INTEGER) FROM search_meta WHERE key = 'total_docs'").Scan(&totalDocs)
	d.QueryRow("SELECT CAST(value AS REAL) FROM search_meta WHERE key = 'avg_doc_len'").Scan(&avgDocLen)

	if totalDocs == 0 {
		return nil, nil
	}

	scores := make(map[int64]float64)

	for _, qt := range queryTokens {
		var termID int64
		err := d.QueryRow("SELECT id FROM search_terms WHERE term = ?", qt).Scan(&termID)
		if err != nil {
			continue
		}

		var docFreq int
		d.QueryRow("SELECT COUNT(*) FROM search_index WHERE term_id = ?", termID).Scan(&docFreq)
		if docFreq == 0 {
			continue
		}

		idf := math.Log((float64(totalDocs)-float64(docFreq)+0.5) / (float64(docFreq) + 0.5))
		if idf < 0 {
			idf = 0
		}

		filterQuery := "SELECT si.drawer_id, si.tf FROM search_index si JOIN drawers dr ON si.drawer_id = dr.id WHERE si.term_id = ?"
		var args []any
		args = append(args, termID)
		if wingID > 0 {
			filterQuery += " AND dr.wing_id = ?"
			args = append(args, wingID)
		}
		if roomID > 0 {
			filterQuery += " AND dr.room_id = ?"
			args = append(args, roomID)
		}

		rows, err := d.Query(filterQuery, args...)
		if err != nil {
			continue
		}
		for rows.Next() {
			var dID int64
			var tf float64
			rows.Scan(&dID, &tf)

			docLen := avgDocLen
			score := idf * (tf * (bm25K1 + 1)) / (tf + bm25K1*(1-bm25B+bm25B*docLen/avgDocLen))
			scores[dID] += score
		}
		rows.Close()
	}

	type scored struct {
		id    int64
		score float64
	}
	var ranked []scored
	for id, s := range scores {
		ranked = append(ranked, scored{id, s})
	}

	for i := 0; i < len(ranked); i++ {
		for j := i + 1; j < len(ranked); j++ {
			if ranked[j].score > ranked[i].score {
				ranked[i], ranked[j] = ranked[j], ranked[i]
			}
		}
	}

	if len(ranked) > limit {
		ranked = ranked[:limit]
	}

	var results []SearchResult
	for _, r := range ranked {
		var sr SearchResult
		sr.DrawerID = r.id
		sr.Score = r.score
		d.QueryRow(`SELECT d.content, d.wing_id, d.room_id, d.hall, d.source_file,
			COALESCE(w.name, ''), COALESCE(rm.name, '')
			FROM drawers d
			LEFT JOIN wings w ON d.wing_id = w.id
			LEFT JOIN rooms rm ON d.room_id = rm.id
			WHERE d.id = ?`, r.id).
			Scan(&sr.Content, &sr.WingID, &sr.RoomID, &sr.Hall, &sr.SourceFile, &sr.WingName, &sr.RoomName)
		results = append(results, sr)
	}

	return results, nil
}
