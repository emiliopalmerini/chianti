package database_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/emiliopalmerini/chianti/platform/database"
)

func TestPtrToNullable_NilReturnsNil(t *testing.T) {
	var p *int
	if got := database.PtrToNullable(p); got != nil {
		t.Errorf("got %v, want nil", got)
	}
}

func TestPtrToNullable_NonNilReturnsValue(t *testing.T) {
	v := 42
	got := database.PtrToNullable(&v)
	if got != 42 {
		t.Errorf("got %v, want 42", got)
	}
}

func TestNullStringToPtr_InvalidReturnsNil(t *testing.T) {
	if got := database.NullStringToPtr(sql.NullString{Valid: false}); got != nil {
		t.Errorf("got %v, want nil", got)
	}
}

func TestNullStringToPtr_ValidReturnsValue(t *testing.T) {
	got := database.NullStringToPtr(sql.NullString{String: "hi", Valid: true})
	if got == nil || *got != "hi" {
		t.Errorf("got %v, want pointer to %q", got, "hi")
	}
}

func TestNullInt64ToIntPtr_InvalidReturnsNil(t *testing.T) {
	if got := database.NullInt64ToIntPtr(sql.NullInt64{Valid: false}); got != nil {
		t.Errorf("got %v, want nil", got)
	}
}

func TestNullInt64ToIntPtr_ValidReturnsValue(t *testing.T) {
	got := database.NullInt64ToIntPtr(sql.NullInt64{Int64: 7, Valid: true})
	if got == nil || *got != 7 {
		t.Errorf("got %v, want pointer to 7", got)
	}
}

func TestIsUniqueConstraint_MatchesAndFiltersByNeedle(t *testing.T) {
	err := errors.New("UNIQUE constraint failed: events.slug")
	if !database.IsUniqueConstraint(err) {
		t.Error("expected match without needles")
	}
	if !database.IsUniqueConstraint(err, "events.slug") {
		t.Error("expected match with matching needle")
	}
	if database.IsUniqueConstraint(err, "events.title") {
		t.Error("expected no match with non-matching needle")
	}
	if database.IsUniqueConstraint(nil) {
		t.Error("nil error must not match")
	}
	if database.IsUniqueConstraint(errors.New("FOREIGN KEY failed")) {
		t.Error("non-UNIQUE error must not match")
	}
}

func TestIsForeignKeyViolation_DetectsTypicalMessage(t *testing.T) {
	if !database.IsForeignKeyViolation(errors.New("FOREIGN KEY constraint failed")) {
		t.Error("expected FK match")
	}
	if database.IsForeignKeyViolation(nil) {
		t.Error("nil error must not match")
	}
	if database.IsForeignKeyViolation(errors.New("UNIQUE failed")) {
		t.Error("non-FK error must not match")
	}
}

func TestClampLimit_PositiveAndNonPositive(t *testing.T) {
	if got := database.ClampLimit(50, 20); got != 50 {
		t.Errorf("positive: got %d, want 50", got)
	}
	if got := database.ClampLimit(0, 20); got != 20 {
		t.Errorf("zero: got %d, want 20", got)
	}
	if got := database.ClampLimit(-5, 20); got != 20 {
		t.Errorf("negative: got %d, want 20", got)
	}
}

func TestWithTx_CommitOnSuccess(t *testing.T) {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec("CREATE TABLE t (v INTEGER)"); err != nil {
		t.Fatal(err)
	}

	err = database.WithTx(context.Background(), db.DB, func(tx *sql.Tx) error {
		_, err := tx.Exec("INSERT INTO t(v) VALUES (1)")
		return err
	})
	if err != nil {
		t.Fatalf("WithTx: %v", err)
	}

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM t").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("count after commit = %d, want 1", count)
	}
}

func TestWithTx_RollbackOnError(t *testing.T) {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec("CREATE TABLE t (v INTEGER)"); err != nil {
		t.Fatal(err)
	}

	sentinel := errors.New("boom")
	err = database.WithTx(context.Background(), db.DB, func(tx *sql.Tx) error {
		if _, err := tx.Exec("INSERT INTO t(v) VALUES (1)"); err != nil {
			return err
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("WithTx: got %v, want %v", err, sentinel)
	}

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM t").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("count after rollback = %d, want 0", count)
	}
}
