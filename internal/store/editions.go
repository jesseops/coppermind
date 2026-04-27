package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jesseops/coppermind/internal/domain"
)

func (s *SQLiteStore) CreateEdition(e *domain.Edition) error {
	if e.Status == "" {
		e.Status = domain.EditionStatusActive
	}
	res, err := s.db.Exec(`
		INSERT INTO editions (work_id, edition_type, format, isbn, publisher, published_year,
		                      narrator, duration_seconds, file_path, file_hash, file_size,
		                      cover_path, notes, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.WorkID, e.EditionType,
		nullOrEmpty(e.Format), nullOrEmpty(e.ISBN), nullOrEmpty(e.Publisher),
		nullOrZeroInt(e.PublishedYear), nullOrEmpty(e.Narrator),
		nullOrZeroInt(e.DurationSeconds),
		nullOrEmpty(e.FilePath), nullOrEmpty(e.FileHash), nullOrZeroInt64(e.FileSize),
		nullOrEmpty(e.CoverPath), nullOrEmpty(e.Notes), e.Status,
	)
	if err != nil {
		return fmt.Errorf("insert edition: %w", err)
	}
	e.ID, _ = res.LastInsertId()
	return nil
}

func (s *SQLiteStore) GetEdition(id int64) (*domain.Edition, error) {
	return s.scanEdition(
		`SELECT id, work_id, edition_type, format, isbn, publisher, published_year,
		        narrator, duration_seconds, file_path, file_hash, file_size,
		        cover_path, notes, status, created_at, updated_at
		 FROM editions WHERE id = ?`, id)
}

func (s *SQLiteStore) ListEditions(workID int64) ([]domain.Edition, error) {
	rows, err := s.db.Query(`
		SELECT id, work_id, edition_type, format, isbn, publisher, published_year,
		       narrator, duration_seconds, file_path, file_hash, file_size,
		       cover_path, notes, status, created_at, updated_at
		FROM editions WHERE work_id = ? AND status = 'active'
		ORDER BY edition_type, format`, workID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var editions []domain.Edition
	for rows.Next() {
		e, err := scanEditionRow(rows)
		if err != nil {
			return nil, err
		}
		editions = append(editions, *e)
	}
	return editions, rows.Err()
}

func (s *SQLiteStore) UpdateEdition(id int64, updates EditionUpdate) error {
	clauses := []string{}
	args := []any{}

	if updates.Format != nil {
		clauses = append(clauses, "format = ?")
		args = append(args, nullOrEmpty(*updates.Format))
	}
	if updates.ISBN != nil {
		clauses = append(clauses, "isbn = ?")
		args = append(args, nullOrEmpty(*updates.ISBN))
	}
	if updates.Publisher != nil {
		clauses = append(clauses, "publisher = ?")
		args = append(args, nullOrEmpty(*updates.Publisher))
	}
	if updates.PublishedYear != nil {
		clauses = append(clauses, "published_year = ?")
		args = append(args, nullOrZeroInt(*updates.PublishedYear))
	}
	if updates.Narrator != nil {
		clauses = append(clauses, "narrator = ?")
		args = append(args, nullOrEmpty(*updates.Narrator))
	}
	if updates.DurationSeconds != nil {
		clauses = append(clauses, "duration_seconds = ?")
		args = append(args, nullOrZeroInt(*updates.DurationSeconds))
	}
	if updates.FilePath != nil {
		clauses = append(clauses, "file_path = ?")
		args = append(args, nullOrEmpty(*updates.FilePath))
	}
	if updates.FileHash != nil {
		clauses = append(clauses, "file_hash = ?")
		args = append(args, nullOrEmpty(*updates.FileHash))
	}
	if updates.FileSize != nil {
		clauses = append(clauses, "file_size = ?")
		args = append(args, nullOrZeroInt64(*updates.FileSize))
	}
	if updates.CoverPath != nil {
		clauses = append(clauses, "cover_path = ?")
		args = append(args, nullOrEmpty(*updates.CoverPath))
	}
	if updates.Notes != nil {
		clauses = append(clauses, "notes = ?")
		args = append(args, nullOrEmpty(*updates.Notes))
	}
	if updates.Status != nil {
		clauses = append(clauses, "status = ?")
		args = append(args, *updates.Status)
	}

	if len(clauses) == 0 {
		return nil
	}

	clauses = append(clauses, "updated_at = datetime('now')")
	args = append(args, id)
	q := "UPDATE editions SET " + strings.Join(clauses, ", ") + " WHERE id = ?"
	res, err := s.db.Exec(q, args...)
	if err != nil {
		return err
	}
	return checkRowsAffected(res, "edition", id)
}

func (s *SQLiteStore) DeleteEdition(id int64) error {
	res, err := s.db.Exec("DELETE FROM editions WHERE id = ?", id)
	if err != nil {
		return err
	}
	return checkRowsAffected(res, "edition", id)
}

func (s *SQLiteStore) FindEditionByHash(hash string) (*domain.Edition, error) {
	if hash == "" {
		return nil, nil
	}
	e, err := s.scanEdition(
		`SELECT id, work_id, edition_type, format, isbn, publisher, published_year,
		        narrator, duration_seconds, file_path, file_hash, file_size,
		        cover_path, notes, status, created_at, updated_at
		 FROM editions WHERE file_hash = ? AND status = 'active' LIMIT 1`, hash)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return e, nil
}

// ── scan helpers ────────────────────────────────────────────────────

func (s *SQLiteStore) scanEdition(query string, args ...any) (*domain.Edition, error) {
	var e domain.Edition
	var format, isbn, publisher, narrator, filePath, fileHash, coverPath, notes sql.NullString
	var pubYear, duration sql.NullInt64
	var fileSize sql.NullInt64
	var createdAt, updatedAt string

	err := s.db.QueryRow(query, args...).Scan(
		&e.ID, &e.WorkID, &e.EditionType,
		&format, &isbn, &publisher, &pubYear,
		&narrator, &duration, &filePath, &fileHash, &fileSize,
		&coverPath, &notes, &e.Status,
		&createdAt, &updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, notFound("edition", "")
		}
		return nil, err
	}

	e.Format = format.String
	e.ISBN = isbn.String
	e.Publisher = publisher.String
	e.PublishedYear = int(pubYear.Int64)
	e.Narrator = narrator.String
	e.DurationSeconds = int(duration.Int64)
	e.FilePath = filePath.String
	e.FileHash = fileHash.String
	e.FileSize = fileSize.Int64
	e.CoverPath = coverPath.String
	e.Notes = notes.String
	e.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	e.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
	return &e, nil
}

func scanEditionRow(rows *sql.Rows) (*domain.Edition, error) {
	var e domain.Edition
	var format, isbn, publisher, narrator, filePath, fileHash, coverPath, notes sql.NullString
	var pubYear, duration, fileSize sql.NullInt64
	var createdAt, updatedAt string

	err := rows.Scan(
		&e.ID, &e.WorkID, &e.EditionType,
		&format, &isbn, &publisher, &pubYear,
		&narrator, &duration, &filePath, &fileHash, &fileSize,
		&coverPath, &notes, &e.Status,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}

	e.Format = format.String
	e.ISBN = isbn.String
	e.Publisher = publisher.String
	e.PublishedYear = int(pubYear.Int64)
	e.Narrator = narrator.String
	e.DurationSeconds = int(duration.Int64)
	e.FilePath = filePath.String
	e.FileHash = fileHash.String
	e.FileSize = fileSize.Int64
	e.CoverPath = coverPath.String
	e.Notes = notes.String
	e.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	e.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
	return &e, nil
}
