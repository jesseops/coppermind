package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jesseops/coppermind/internal/domain"
)

func (s *SQLiteStore) CreateSeries(name, description string) (*domain.Series, error) {
	res, err := s.db.Exec("INSERT INTO series (name, description) VALUES (?, ?)", name, nullOrEmpty(description))
	if err != nil {
		return nil, fmt.Errorf("insert series: %w", err)
	}
	id, _ := res.LastInsertId()
	return s.GetSeries(id)
}

func (s *SQLiteStore) GetSeries(id int64) (*domain.Series, error) {
	var ser domain.Series
	var createdAt string
	var desc sql.NullString
	err := s.db.QueryRow("SELECT id, name, description, created_at FROM series WHERE id = ?", id).
		Scan(&ser.ID, &ser.Name, &desc, &createdAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("series %d not found", id)
		}
		return nil, err
	}
	ser.Description = desc.String
	ser.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	return &ser, nil
}

func (s *SQLiteStore) FindSeriesByName(name string) (*domain.Series, error) {
	var ser domain.Series
	var createdAt string
	var desc sql.NullString
	err := s.db.QueryRow(
		"SELECT id, name, description, created_at FROM series WHERE lower(trim(name)) = lower(trim(?))", name,
	).Scan(&ser.ID, &ser.Name, &desc, &createdAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // not found is not an error
		}
		return nil, err
	}
	ser.Description = desc.String
	ser.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	return &ser, nil
}

func (s *SQLiteStore) ListSeries(libraryID int64) ([]SeriesWithCount, error) {
	rows, err := s.db.Query(`
		SELECT se.id, se.name, se.description, se.created_at, COUNT(DISTINCT w.id) AS work_count
		FROM series se
		JOIN works w ON w.series_id = se.id AND w.library_id = ?
		GROUP BY se.id
		ORDER BY se.name`, libraryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []SeriesWithCount
	for rows.Next() {
		var sc SeriesWithCount
		var createdAt string
		var desc sql.NullString
		if err := rows.Scan(&sc.ID, &sc.Name, &desc, &createdAt, &sc.WorkCount); err != nil {
			return nil, err
		}
		sc.Description = desc.String
		sc.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		result = append(result, sc)
	}
	return result, rows.Err()
}

// nullOrEmpty returns nil for empty strings, the string otherwise.
func nullOrEmpty(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}
