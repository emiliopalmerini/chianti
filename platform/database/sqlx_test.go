package database_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
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

func TestWithTx_ReturnsCallbackError(t *testing.T) {
	sentinel := errors.New("boom")
	db := sql.OpenDB(fakeConnector{})
	defer db.Close()

	err := database.WithTx(context.Background(), db, func(tx *sql.Tx) error {
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("WithTx: got %v, want %v", err, sentinel)
	}
}

type fakeConnector struct{}

func (fakeConnector) Connect(context.Context) (driver.Conn, error) {
	return fakeConn{}, nil
}

func (fakeConnector) Driver() driver.Driver { return fakeDriver{} }

type fakeDriver struct{}

func (fakeDriver) Open(string) (driver.Conn, error) { return fakeConn{}, nil }

type fakeConn struct{}

func (fakeConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("not implemented") }

func (fakeConn) Close() error { return nil }

func (fakeConn) Begin() (driver.Tx, error) { return fakeTx{}, nil }

func (fakeConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	return fakeTx{}, nil
}

type fakeTx struct{}

func (fakeTx) Commit() error { return nil }

func (fakeTx) Rollback() error { return nil }
