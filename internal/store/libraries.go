package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jesseops/coppermind/internal/domain"
)

func (s *SQLiteStore) CreateLibrary(name string) (*domain.Library, error) {
	res, err := s.db.Exec("INSERT INTO libraries (name) VALUES (?)", name)
	if err != nil {
		return nil, fmt.Errorf("insert library: %w", err)
	}
	id, _ := res.LastInsertId()
	return s.GetLibrary(id)
}

func (s *SQLiteStore) GetLibrary(id int64) (*domain.Library, error) {
	var lib domain.Library
	var createdAt string
	err := s.db.QueryRow("SELECT id, name, created_at FROM libraries WHERE id = ?", id).
		Scan(&lib.ID, &lib.Name, &createdAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, notFound("library", id)
		}
		return nil, err
	}
	lib.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	return &lib, nil
}

func (s *SQLiteStore) ListLibraries() ([]domain.Library, error) {
	rows, err := s.db.Query("SELECT id, name, created_at FROM libraries ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var libs []domain.Library
	for rows.Next() {
		var lib domain.Library
		var createdAt string
		if err := rows.Scan(&lib.ID, &lib.Name, &createdAt); err != nil {
			return nil, err
		}
		lib.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		libs = append(libs, lib)
	}
	return libs, rows.Err()
}
