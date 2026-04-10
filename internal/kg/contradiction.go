package kg

import (
	"fmt"
	"strings"

	"github.com/snow-ghost/mem/internal/db"
)

type Conflict struct {
	Subject     string
	Predicate   string
	ExistingObj string
	NewObj      string
}

func CheckContradiction(d *db.DB, subject, predicate, newObject string) ([]Conflict, error) {
	subID := entityID(subject)
	pred := strings.ToLower(strings.ReplaceAll(predicate, " ", "_"))

	rows, err := d.Query(`
		SELECT e.name FROM triples t
		JOIN entities e ON t.object = e.id
		WHERE t.subject = ? AND t.predicate = ? AND t.valid_to IS NULL AND t.object != ?`,
		subID, pred, entityID(newObject),
	)
	if err != nil {
		return nil, fmt.Errorf("check contradiction: %w", err)
	}
	defer rows.Close()

	var conflicts []Conflict
	for rows.Next() {
		var existingObj string
		rows.Scan(&existingObj)
		conflicts = append(conflicts, Conflict{
			Subject:     subject,
			Predicate:   predicate,
			ExistingObj: existingObj,
			NewObj:      newObject,
		})
	}
	return conflicts, nil
}
