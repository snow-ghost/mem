package db

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

type DB struct {
	*sql.DB
}

func Open(path string) (*DB, error) {
	sqlDB, err := sql.Open("sqlite", path+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	sqlDB.SetMaxOpenConns(2)
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return &DB{sqlDB}, nil
}
