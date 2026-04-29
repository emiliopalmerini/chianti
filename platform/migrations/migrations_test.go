package migrations_test

import (
	"embed"
	"io/fs"
	"sort"
	"testing"
	"testing/fstest"

	"github.com/emiliopalmerini/chianti/platform/database"
	"github.com/emiliopalmerini/chianti/platform/migrations"
)

//go:embed testdata/sql/*.sql
var testFS embed.FS

func openMem(t *testing.T) *database.DB {
	t.Helper()
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestRunCreatesSchemaMigrations(t *testing.T) {
	db := openMem(t)

	if err := migrations.Run(db.DB, testFS, "testdata/sql"); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Errorf("expected 3 applied migrations, got %d", count)
	}
}

func TestRunIsIdempotent(t *testing.T) {
	db := openMem(t)

	if err := migrations.Run(db.DB, testFS, "testdata/sql"); err != nil {
		t.Fatal(err)
	}
	if err := migrations.Run(db.DB, testFS, "testdata/sql"); err != nil {
		t.Fatalf("second Run: %v", err)
	}

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Errorf("expected 3 rows after second Run, got %d", count)
	}
}

func TestRunSkipsAppliedMigrations(t *testing.T) {
	db := openMem(t)

	mfs := fstest.MapFS{
		"sql/000001_a.up.sql": &fstest.MapFile{Data: []byte("CREATE TABLE a (id INTEGER PRIMARY KEY);")},
		"sql/000002_b.up.sql": &fstest.MapFile{Data: []byte("CREATE TABLE b (id INTEGER PRIMARY KEY);")},
		"sql/000003_c.up.sql": &fstest.MapFile{Data: []byte("CREATE TABLE c (id INTEGER PRIMARY KEY);")},
	}

	if _, err := db.Exec(`CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO schema_migrations(version, applied_at) VALUES (1, '2026-01-01T00:00:00Z'), (2, '2026-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}

	if err := migrations.Run(db.DB, mfs, "sql"); err != nil {
		t.Fatalf("Run: %v", err)
	}

	rows, err := db.Query("SELECT version FROM schema_migrations ORDER BY version")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var versions []int
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			t.Fatal(err)
		}
		versions = append(versions, v)
	}
	if len(versions) != 3 || versions[0] != 1 || versions[1] != 2 || versions[2] != 3 {
		t.Errorf("expected versions [1 2 3], got %v", versions)
	}

	if _, err := db.Exec("SELECT id FROM a"); err == nil {
		t.Error("expected table a to not exist (migration 1 was pre-marked applied)")
	}
	if _, err := db.Exec("SELECT id FROM b"); err == nil {
		t.Error("expected table b to not exist (migration 2 was pre-marked applied)")
	}
	if _, err := db.Exec("SELECT id FROM c"); err != nil {
		t.Errorf("expected table c to exist (migration 3 should have run): %v", err)
	}
}

func TestRunFailsOnMalformedFilename(t *testing.T) {
	bad := fstest.MapFS{
		"sql/notanumber_foo.up.sql": &fstest.MapFile{Data: []byte("CREATE TABLE x (id INTEGER);")},
	}
	db := openMem(t)
	if err := migrations.Run(db.DB, bad, "sql"); err == nil {
		t.Error("expected error for malformed filename, got nil")
	}
}

func TestRunAppliesInLexicographicOrder(t *testing.T) {
	db := openMem(t)

	mfs := fstest.MapFS{
		"sql/000001_a.up.sql": &fstest.MapFile{Data: []byte("CREATE TABLE a (id INTEGER PRIMARY KEY);")},
		"sql/000002_b.up.sql": &fstest.MapFile{Data: []byte("CREATE TABLE b (id INTEGER PRIMARY KEY);")},
		"sql/000003_c.up.sql": &fstest.MapFile{Data: []byte("ALTER TABLE a ADD COLUMN bref INTEGER REFERENCES b(id);")},
	}
	entries, _ := fs.ReadDir(mfs, "sql")
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() > entries[j].Name() })
	_ = entries

	if err := migrations.Run(db.DB, mfs, "sql"); err != nil {
		t.Fatalf("Run: %v", err)
	}

	rows, err := db.Query("SELECT version FROM schema_migrations ORDER BY version")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var versions []int
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			t.Fatal(err)
		}
		versions = append(versions, v)
	}
	if len(versions) != 3 || versions[0] != 1 || versions[1] != 2 || versions[2] != 3 {
		t.Errorf("expected [1 2 3], got %v", versions)
	}
}
