package store

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

// SQLiteStore implements the Store interface using SQLite.
type SQLiteStore struct {
	db *sqlx.DB
}

var _ Store = (*SQLiteStore)(nil)

// NewSQLiteStore opens (or creates) a SQLite database and runs migrations.
func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	// Ensure parent directory exists.
	dir := filepath.Dir(dbPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create db directory: %w", err)
		}
	}

	db, err := sqlx.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// SQLite should use a single connection to avoid locking issues.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if err := ConfigureDB(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("configure database: %w", err)
	}

	if err := RunMigrations(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	return &SQLiteStore{db: db}, nil
}

// NewMemoryStore creates an in-memory SQLite store, primarily for testing.
func NewMemoryStore() (*SQLiteStore, error) {
	return NewSQLiteStore(":memory:")
}

// Close closes the database connection.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// DB returns the underlying sqlx.DB (for advanced queries if needed).
func (s *SQLiteStore) DB() *sqlx.DB {
	return s.db
}
