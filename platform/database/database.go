// Package database opens and tunes the SQLite handle shared across slices.
// Single writer, WAL, foreign keys on.
package database

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	*sql.DB
}

// Open opens a SQLite database at path and applies the platform pragmas.
// path may be ":memory:" or a file path.
func Open(path string) (*DB, error) {
	handle, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %q: %w", path, err)
	}
	handle.SetMaxOpenConns(1)
	handle.SetMaxIdleConns(1)

	pragmas := []string{
		"PRAGMA journal_mode = WAL;",
		"PRAGMA busy_timeout = 5000;",
		"PRAGMA foreign_keys = ON;",
		"PRAGMA synchronous = NORMAL;",
	}
	for _, p := range pragmas {
		if _, err := handle.Exec(p); err != nil {
			_ = handle.Close()
			return nil, fmt.Errorf("pragma %q: %w", p, err)
		}
	}
	return &DB{DB: handle}, nil
}

func (d *DB) Close() error { return d.DB.Close() }
