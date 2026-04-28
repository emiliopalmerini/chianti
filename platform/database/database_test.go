package database_test

import (
	"testing"

	"github.com/emiliopalmerini/chianti/platform/database"
)

func TestOpenAppliesPragmas(t *testing.T) {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	var fk int
	if err := db.QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil {
		t.Fatal(err)
	}
	if fk != 1 {
		t.Errorf("foreign_keys = %d, want 1", fk)
	}
	// journal_mode is "memory" for :memory: DB (WAL is silently downgraded),
	// so we only assert foreign_keys here; WAL is exercised in integration tests
	// that use a temp-file DB.
}
