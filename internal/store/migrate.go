package store

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
)

// migration is a single schema migration step.
type migration struct {
	version int
	sql     string
}

// All migrations in order. Each migration's SQL is run inside a transaction.
var migrations = []migration{
	{
		version: 1,
		sql: `
-- ── Libraries ───────────────────────────────────────────────────────
CREATE TABLE libraries (
    id         INTEGER PRIMARY KEY,
    name       TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

-- ── Identity ────────────────────────────────────────────────────────
CREATE TABLE users (
    id            INTEGER PRIMARY KEY,
    username      TEXT    NOT NULL UNIQUE,
    display_name  TEXT    NOT NULL,
    password_hash TEXT    NOT NULL,
    role          TEXT    NOT NULL DEFAULT 'viewer',
    kindle_email  TEXT    NOT NULL DEFAULT '',
    created_at    TEXT    NOT NULL DEFAULT (datetime('now')),
    updated_at    TEXT    NOT NULL DEFAULT (datetime('now'))
);

-- ── Bibliographic ───────────────────────────────────────────────────
CREATE TABLE authors (
    id         INTEGER PRIMARY KEY,
    name       TEXT NOT NULL,
    sort_name  TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE series (
    id          INTEGER PRIMARY KEY,
    name        TEXT NOT NULL,
    description TEXT,
    created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE works (
    id              INTEGER PRIMARY KEY,
    library_id      INTEGER NOT NULL REFERENCES libraries(id),
    title           TEXT    NOT NULL,
    sort_title      TEXT    NOT NULL,
    description     TEXT,
    series_id       INTEGER REFERENCES series(id),
    series_index    REAL,
    language        TEXT,
    first_published INTEGER,
    cover_path      TEXT,
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at      TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE work_authors (
    work_id   INTEGER NOT NULL REFERENCES works(id) ON DELETE CASCADE,
    author_id INTEGER NOT NULL REFERENCES authors(id),
    role      TEXT    NOT NULL DEFAULT 'author',
    PRIMARY KEY (work_id, author_id, role)
);

CREATE TABLE work_tags (
    work_id INTEGER NOT NULL REFERENCES works(id) ON DELETE CASCADE,
    tag     TEXT    NOT NULL,
    PRIMARY KEY (work_id, tag)
);

-- ── Editions & Files ────────────────────────────────────────────────
CREATE TABLE editions (
    id               INTEGER PRIMARY KEY,
    work_id          INTEGER NOT NULL REFERENCES works(id) ON DELETE CASCADE,
    edition_type     TEXT    NOT NULL,
    format           TEXT,
    isbn             TEXT,
    publisher        TEXT,
    published_year   INTEGER,
    narrator         TEXT,
    duration_seconds INTEGER,
    file_path        TEXT,
    file_hash        TEXT,
    file_size        INTEGER,
    cover_path       TEXT,
    notes            TEXT,
    status           TEXT NOT NULL DEFAULT 'active',
    created_at       TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at       TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE tracks (
    id               INTEGER PRIMARY KEY,
    edition_id       INTEGER NOT NULL REFERENCES editions(id) ON DELETE CASCADE,
    track_index      INTEGER NOT NULL,
    title            TEXT,
    duration_seconds INTEGER,
    file_path        TEXT NOT NULL,
    file_hash        TEXT,
    created_at       TEXT NOT NULL DEFAULT (datetime('now'))
);

-- ── User State ──────────────────────────────────────────────────────
CREATE TABLE reading_states (
    user_id          INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    edition_id       INTEGER NOT NULL REFERENCES editions(id) ON DELETE CASCADE,
    status           TEXT    NOT NULL DEFAULT 'unread',
    progress         REAL    NOT NULL DEFAULT 0,
    chapter_index    INTEGER,
    scroll_position  REAL,
    track_index      INTEGER,
    position_seconds REAL,
    started_at       TEXT,
    finished_at      TEXT,
    updated_at       TEXT NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (user_id, edition_id)
);

CREATE TABLE user_ratings (
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    work_id    INTEGER NOT NULL REFERENCES works(id) ON DELETE CASCADE,
    rating     INTEGER,
    review     TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (user_id, work_id)
);

CREATE TABLE shelves (
    id          INTEGER PRIMARY KEY,
    user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name        TEXT    NOT NULL,
    description TEXT,
    is_public   INTEGER NOT NULL DEFAULT 1,
    created_at  TEXT    NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE shelf_works (
    shelf_id INTEGER NOT NULL REFERENCES shelves(id) ON DELETE CASCADE,
    work_id  INTEGER NOT NULL REFERENCES works(id) ON DELETE CASCADE,
    added_at TEXT    NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (shelf_id, work_id)
);

-- ── Indexes ─────────────────────────────────────────────────────────
CREATE INDEX idx_works_library_id   ON works(library_id);
CREATE INDEX idx_works_sort_title   ON works(sort_title);
CREATE INDEX idx_works_series_id    ON works(series_id);
CREATE INDEX idx_authors_sort_name  ON authors(sort_name);
CREATE INDEX idx_editions_work_id   ON editions(work_id);
CREATE INDEX idx_editions_status    ON editions(status);
CREATE INDEX idx_editions_file_hash ON editions(file_hash);
CREATE INDEX idx_tracks_edition_id  ON tracks(edition_id);
CREATE INDEX idx_shelves_user_id    ON shelves(user_id);
`,
	},
}

// RunMigrations applies all pending migrations to the database.
func RunMigrations(db *sqlx.DB) error {
	// Ensure schema_version table exists.
	if _, err := db.Exec("CREATE TABLE IF NOT EXISTS schema_version (version INTEGER NOT NULL)"); err != nil {
		return fmt.Errorf("create schema_version: %w", err)
	}

	current, err := currentVersion(db)
	if err != nil {
		return fmt.Errorf("get schema version: %w", err)
	}

	for _, m := range migrations {
		if m.version <= current {
			continue
		}

		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("begin migration %d: %w", m.version, err)
		}

		if _, err := tx.Exec(m.sql); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migration %d: %w", m.version, err)
		}

		if err := setVersion(tx, m.version); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("set version %d: %w", m.version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %d: %w", m.version, err)
		}
	}

	return nil
}

func currentVersion(db *sqlx.DB) (int, error) {
	var version int
	err := db.QueryRow("SELECT version FROM schema_version LIMIT 1").Scan(&version)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil
		}
		return 0, err
	}
	return version, nil
}

func setVersion(tx *sql.Tx, version int) error {
	var count int
	if err := tx.QueryRow("SELECT COUNT(*) FROM schema_version").Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		_, err := tx.Exec("INSERT INTO schema_version (version) VALUES (?)", version)
		return err
	}
	_, err := tx.Exec("UPDATE schema_version SET version = ?", version)
	return err
}

// ConfigureDB sets SQLite pragmas for optimal operation.
func ConfigureDB(db *sqlx.DB) error {
	pragmas := []string{
		"PRAGMA journal_mode = WAL",
		"PRAGMA busy_timeout = 5000",
		"PRAGMA foreign_keys = ON",
		"PRAGMA synchronous = NORMAL",
	}
	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			return fmt.Errorf("%s: %w", pragma, err)
		}
	}
	return nil
}
