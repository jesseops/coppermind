package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jesseops/coppermind/internal/domain"
)

func (s *SQLiteStore) CreateShelf(userID int64, name, description string, isPublic bool) (*domain.Shelf, error) {
	pub := 0
	if isPublic {
		pub = 1
	}
	res, err := s.db.Exec(
		"INSERT INTO shelves (user_id, name, description, is_public) VALUES (?, ?, ?, ?)",
		userID, name, nullOrEmpty(description), pub,
	)
	if err != nil {
		return nil, fmt.Errorf("insert shelf: %w", err)
	}
	id, _ := res.LastInsertId()
	return s.GetShelf(id)
}

func (s *SQLiteStore) GetShelf(id int64) (*domain.Shelf, error) {
	var sh domain.Shelf
	var desc sql.NullString
	var isPublic int
	var createdAt string

	err := s.db.QueryRow(
		"SELECT id, user_id, name, description, is_public, created_at FROM shelves WHERE id = ?", id,
	).Scan(&sh.ID, &sh.UserID, &sh.Name, &desc, &isPublic, &createdAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("shelf %d not found", id)
		}
		return nil, err
	}
	sh.Description = desc.String
	sh.IsPublic = isPublic != 0
	sh.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)

	// Count works.
	s.db.QueryRow("SELECT COUNT(*) FROM shelf_works WHERE shelf_id = ?", id).Scan(&sh.WorkCount)
	return &sh, nil
}

func (s *SQLiteStore) ListShelves(userID int64) ([]domain.Shelf, error) {
	rows, err := s.db.Query(`
		SELECT s.id, s.user_id, s.name, s.description, s.is_public, s.created_at,
		       (SELECT COUNT(*) FROM shelf_works sw WHERE sw.shelf_id = s.id) AS work_count
		FROM shelves s WHERE s.user_id = ? ORDER BY s.name`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var shelves []domain.Shelf
	for rows.Next() {
		var sh domain.Shelf
		var desc sql.NullString
		var isPublic int
		var createdAt string
		if err := rows.Scan(&sh.ID, &sh.UserID, &sh.Name, &desc, &isPublic, &createdAt, &sh.WorkCount); err != nil {
			return nil, err
		}
		sh.Description = desc.String
		sh.IsPublic = isPublic != 0
		sh.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		shelves = append(shelves, sh)
	}
	return shelves, rows.Err()
}

func (s *SQLiteStore) DeleteShelf(id int64) error {
	res, err := s.db.Exec("DELETE FROM shelves WHERE id = ?", id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("shelf %d not found", id)
	}
	return nil
}

func (s *SQLiteStore) AddToShelf(shelfID, workID int64) error {
	_, err := s.db.Exec(
		"INSERT OR IGNORE INTO shelf_works (shelf_id, work_id) VALUES (?, ?)",
		shelfID, workID,
	)
	return err
}

func (s *SQLiteStore) RemoveFromShelf(shelfID, workID int64) error {
	_, err := s.db.Exec(
		"DELETE FROM shelf_works WHERE shelf_id = ? AND work_id = ?",
		shelfID, workID,
	)
	return err
}

func (s *SQLiteStore) ListShelfWorks(shelfID int64) ([]domain.Work, error) {
	rows, err := s.db.Query(`
		SELECT w.id, w.library_id, w.title, w.sort_title, w.description, w.series_id, w.series_index,
		       w.language, w.first_published, w.cover_path, w.created_at, w.updated_at
		FROM works w
		JOIN shelf_works sw ON sw.work_id = w.id
		WHERE sw.shelf_id = ?
		ORDER BY sw.added_at DESC`, shelfID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var works []domain.Work
	for rows.Next() {
		w, err := scanWorkRow(rows)
		if err != nil {
			return nil, err
		}
		works = append(works, *w)
	}
	return works, rows.Err()
}
