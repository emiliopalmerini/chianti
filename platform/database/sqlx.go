// Package database also exposes small helpers that every slice's sqlite
// adapter used to re-implement: a row/Rows scanner shim and nullable
// conversion helpers between *T and sql.NullX.
package database

import (
	"context"
	"database/sql"
	"strings"
)

// TimeFormat is the canonical on-disk representation for every time column
// stored by the slices. It is fixed-width so lexicographic comparison in SQL
// (ORDER BY, range predicates) matches chronological order; RFC3339Nano
// cannot be used because it trims trailing zeros and "12:00:00Z" would
// compare greater than "12:00:00.5Z".
const TimeFormat = "2006-01-02T15:04:05.000000000Z07:00"

// Scanner is the common subset of *sql.Row and *sql.Rows used by per-slice
// scan helpers. Each adapter used to declare its own local copy.
type Scanner interface {
	Scan(dest ...any) error
}

// PtrToNullable returns the dereferenced value or nil, suitable for passing
// as a database/sql driver argument that may be NULL.
func PtrToNullable[T any](p *T) any {
	if p == nil {
		return nil
	}
	return *p
}

// NullStringToPtr converts a sql.NullString to *string, returning nil when
// the column was NULL.
func NullStringToPtr(n sql.NullString) *string {
	if !n.Valid {
		return nil
	}
	v := n.String
	return &v
}

// NullInt64ToIntPtr converts a sql.NullInt64 to *int, returning nil when the
// column was NULL. The widening cast matches how domain types model ages and
// counts as plain int.
func NullInt64ToIntPtr(n sql.NullInt64) *int {
	if !n.Valid {
		return nil
	}
	v := int(n.Int64)
	return &v
}

// IsUniqueConstraint reports whether err is a sqlite UNIQUE constraint
// violation. Extra substrings can be supplied to target a specific column
// (e.g. IsUniqueConstraint(err, "events.slug")).
func IsUniqueConstraint(err error, needles ...string) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	if !strings.Contains(msg, "UNIQUE") {
		return false
	}
	for _, n := range needles {
		if !strings.Contains(msg, n) {
			return false
		}
	}
	return true
}

// IsForeignKeyViolation reports whether err is a sqlite FOREIGN KEY
// constraint violation.
func IsForeignKeyViolation(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "FOREIGN KEY constraint failed")
}

// ClampLimit returns limit when it is positive, otherwise fallback. Used by
// list queries to honour an explicit page size and fall back to a default
// when the caller passes zero or a negative value.
func ClampLimit(limit, fallback int) int {
	if limit <= 0 {
		return fallback
	}
	return limit
}

// WithTx runs fn inside a sqlite transaction, committing on success and
// rolling back on any error. The deferred rollback is a no-op after a
// successful commit, so the idiom is safe on either path.
func WithTx(ctx context.Context, db *sql.DB, fn func(*sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}
