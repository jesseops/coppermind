package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jesseops/coppermind/internal/domain"
)

func (s *SQLiteStore) CreateAuthor(name, sortName string) (*domain.Author, error) {
	res, err := s.db.Exec("INSERT INTO authors (name, sort_name) VALUES (?, ?)", name, sortName)
	if err != nil {
		return nil, fmt.Errorf("insert author: %w", err)
	}
	id, _ := res.LastInsertId()
	return s.GetAuthor(id)
}

func (s *SQLiteStore) GetAuthor(id int64) (*domain.Author, error) {
	var a domain.Author
	var createdAt string
	err := s.db.QueryRow("SELECT id, name, sort_name, created_at FROM authors WHERE id = ?", id).
		Scan(&a.ID, &a.Name, &a.SortName, &createdAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, notFound("author", id)
		}
		return nil, err
	}
	a.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	return &a, nil
}

func (s *SQLiteStore) FindAuthorBySortName(sortName string) (*domain.Author, error) {
	var a domain.Author
	var createdAt string
	err := s.db.QueryRow("SELECT id, name, sort_name, created_at FROM authors WHERE sort_name = ?", sortName).
		Scan(&a.ID, &a.Name, &a.SortName, &createdAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // not found is not an error
		}
		return nil, err
	}
	a.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	return &a, nil
}

func (s *SQLiteStore) ListAuthors(libraryID int64) ([]AuthorWithCount, error) {
	rows, err := s.db.Query(`
		SELECT a.id, a.name, a.sort_name, a.created_at, COUNT(DISTINCT wa.work_id) AS work_count
		FROM authors a
		JOIN work_authors wa ON wa.author_id = a.id
		JOIN works w ON w.id = wa.work_id AND w.library_id = ?
		GROUP BY a.id
		ORDER BY a.sort_name`, libraryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []AuthorWithCount
	for rows.Next() {
		var ac AuthorWithCount
		var createdAt string
		if err := rows.Scan(&ac.ID, &ac.Name, &ac.SortName, &createdAt, &ac.WorkCount); err != nil {
			return nil, err
		}
		ac.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		result = append(result, ac)
	}
	return result, rows.Err()
}

func (s *SQLiteStore) LinkWorkAuthor(workID, authorID int64, role string) error {
	_, err := s.db.Exec(
		"INSERT OR IGNORE INTO work_authors (work_id, author_id, role) VALUES (?, ?, ?)",
		workID, authorID, role,
	)
	return err
}

func (s *SQLiteStore) UnlinkWorkAuthor(workID, authorID int64, role string) error {
	_, err := s.db.Exec(
		"DELETE FROM work_authors WHERE work_id = ? AND author_id = ? AND role = ?",
		workID, authorID, role,
	)
	return err
}

func (s *SQLiteStore) GetWorkAuthors(workID int64) ([]domain.WorkAuthor, error) {
	rows, err := s.db.Query(`
		SELECT wa.work_id, wa.author_id, wa.role, a.name
		FROM work_authors wa
		JOIN authors a ON a.id = wa.author_id
		WHERE wa.work_id = ?
		ORDER BY wa.role, a.sort_name`, workID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []domain.WorkAuthor
	for rows.Next() {
		var wa domain.WorkAuthor
		if err := rows.Scan(&wa.WorkID, &wa.AuthorID, &wa.Role, &wa.AuthorName); err != nil {
			return nil, err
		}
		result = append(result, wa)
	}
	return result, rows.Err()
}
