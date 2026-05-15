package migrations

import (
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
)

func TestCollectPendingSkipsAppliedMigrations(t *testing.T) {
	mfs := fstest.MapFS{
		"sql/000001_a.up.sql": {Data: []byte("CREATE TABLE a (id INTEGER PRIMARY KEY);")},
		"sql/000002_b.up.sql": {Data: []byte("CREATE TABLE b (id INTEGER PRIMARY KEY);")},
		"sql/000003_c.up.sql": {Data: []byte("CREATE TABLE c (id INTEGER PRIMARY KEY);")},
	}

	got, err := collectPending(mfs, "sql", map[int]bool{1: true, 2: true})
	if err != nil {
		t.Fatalf("collectPending: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("pending count = %d, want 1", len(got))
	}
	if got[0].version != 3 || got[0].name != "000003_c.up.sql" {
		t.Fatalf("pending = %+v, want version 3 migration", got[0])
	}
}

func TestCollectPendingFailsOnMalformedFilename(t *testing.T) {
	cases := map[string]string{
		"not numeric": "notanumber_foo.up.sql",
		"not padded":  "1_foo.up.sql",
		"no suffix":   "000001.up.sql",
	}
	for name, filename := range cases {
		t.Run(name, func(t *testing.T) {
			bad := fstest.MapFS{
				"sql/" + filename: {Data: []byte("CREATE TABLE x (id INTEGER);")},
			}
			if _, err := collectPending(bad, "sql", nil); err == nil {
				t.Error("expected error for malformed filename, got nil")
			}
		})
	}
}

func TestCollectPendingSortsByNumericVersion(t *testing.T) {
	mfs := fstest.MapFS{
		"sql/000003_c.up.sql": {Data: []byte("third")},
		"sql/000001_a.up.sql": {Data: []byte("first")},
		"sql/000002_b.up.sql": {Data: []byte("second")},
	}

	got, err := collectPending(mfs, "sql", nil)
	if err != nil {
		t.Fatalf("collectPending: %v", err)
	}
	var versions []int
	for _, m := range got {
		versions = append(versions, m.version)
	}
	if !reflect.DeepEqual(versions, []int{1, 2, 3}) {
		t.Fatalf("versions = %v, want [1 2 3]", versions)
	}
}

func TestCollectPendingFailsBeforeReturningDuplicateVersions(t *testing.T) {
	mfs := fstest.MapFS{
		"sql/000001_a.up.sql":     {Data: []byte("CREATE TABLE a (id INTEGER PRIMARY KEY);")},
		"sql/000001_again.up.sql": {Data: []byte("CREATE TABLE should_not_exist (id INTEGER PRIMARY KEY);")},
	}

	_, err := collectPending(mfs, "sql", nil)
	if err == nil {
		t.Fatal("expected duplicate version error, got nil")
	}
	if !strings.Contains(err.Error(), "duplicate migration version 1") {
		t.Fatalf("unexpected error: %v", err)
	}
}
