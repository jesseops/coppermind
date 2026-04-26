package store

import (
	"testing"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

func TestRunMigrations(t *testing.T) {
	db, err := sqlx.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := ConfigureDB(db); err != nil {
		t.Fatalf("ConfigureDB: %v", err)
	}

	// Run migrations.
	if err := RunMigrations(db); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	// Check version.
	var version int
	if err := db.QueryRow("SELECT version FROM schema_version").Scan(&version); err != nil {
		t.Fatalf("query version: %v", err)
	}
	if version != 1 {
		t.Errorf("version = %d, want 1", version)
	}

	// Check tables exist.
	tables := []string{
		"libraries", "users", "authors", "series", "works",
		"work_authors", "work_tags", "editions", "tracks",
		"reading_states", "user_ratings", "shelves", "shelf_works",
	}
	for _, table := range tables {
		var name string
		err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		if err != nil {
			t.Errorf("table %s not found: %v", table, err)
		}
	}

	// Running migrations again should be a no-op.
	if err := RunMigrations(db); err != nil {
		t.Fatalf("RunMigrations (idempotent): %v", err)
	}
}
