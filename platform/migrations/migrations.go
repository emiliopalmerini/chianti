// Package migrations applies numbered .up.sql files from a caller-supplied
// fs.FS in lexicographic order, inside a transaction per file. Each applied
// version is recorded in schema_migrations; re-running Run is a no-op once a
// version is marked applied.
package migrations

import (
	"database/sql"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Run applies pending .up.sql files from fsys[dir] to db, in lexicographic
// order, in a transaction per file. Each applied version is recorded in
// schema_migrations. Re-running is idempotent. The version is parsed as the
// prefix before the first underscore: e.g. "000003_documents.up.sql" -> 3.
func Run(db *sql.DB, fsys fs.FS, dir string) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		applied_at TEXT NOT NULL
	)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	applied, err := loadApplied(db)
	if err != nil {
		return err
	}

	pending, err := collectPending(fsys, dir, applied)
	if err != nil {
		return err
	}

	for _, m := range pending {
		if err := applyOne(db, m); err != nil {
			return err
		}
	}
	return nil
}

type migration struct {
	version int
	name    string
	sql     string
}

func loadApplied(db *sql.DB) (map[int]bool, error) {
	rows, err := db.Query("SELECT version FROM schema_migrations")
	if err != nil {
		return nil, fmt.Errorf("select schema_migrations: %w", err)
	}
	defer rows.Close()
	out := map[int]bool{}
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out[v] = true
	}
	return out, rows.Err()
}

func collectPending(fsys fs.FS, dir string, applied map[int]bool) ([]migration, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", dir, err)
	}
	var all []migration
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".up.sql") {
			continue
		}
		v, err := parseVersion(name)
		if err != nil {
			return nil, err
		}
		if applied[v] {
			continue
		}
		body, err := fs.ReadFile(fsys, dir+"/"+name)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", name, err)
		}
		all = append(all, migration{version: v, name: name, sql: string(body)})
	}
	sort.Slice(all, func(i, j int) bool { return all[i].version < all[j].version })
	return all, nil
}

func parseVersion(name string) (int, error) {
	head, _, ok := strings.Cut(name, "_")
	if !ok {
		return 0, fmt.Errorf("migration %q missing version prefix", name)
	}
	v, err := strconv.Atoi(head)
	if err != nil {
		return 0, fmt.Errorf("migration %q: bad version: %w", name, err)
	}
	return v, nil
}

func applyOne(db *sql.DB, m migration) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin %s: %w", m.name, err)
	}
	if _, err := tx.Exec(m.sql); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("apply %s: %w", m.name, err)
	}
	if _, err := tx.Exec("INSERT INTO schema_migrations(version, applied_at) VALUES (?, ?)", m.version, time.Now().UTC().Format(time.RFC3339)); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("record %s: %w", m.name, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit %s: %w", m.name, err)
	}
	return nil
}
