package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/jesseops/coppermind/internal/domain"
)

type editionRow struct {
	ID              int64          `db:"id"`
	WorkID          int64          `db:"work_id"`
	EditionType     string         `db:"edition_type"`
	Format          sql.NullString `db:"format"`
	ISBN            sql.NullString `db:"isbn"`
	Publisher       sql.NullString `db:"publisher"`
	PublishedYear   sql.NullInt64  `db:"published_year"`
	Narrator        sql.NullString `db:"narrator"`
	DurationSeconds sql.NullInt64  `db:"duration_seconds"`
	FilePath        sql.NullString `db:"file_path"`
	FileHash        sql.NullString `db:"file_hash"`
	FileSize        sql.NullInt64  `db:"file_size"`
	CoverPath       sql.NullString `db:"cover_path"`
	Notes           sql.NullString `db:"notes"`
	Status          string         `db:"status"`
	CreatedAt       string         `db:"created_at"`
	UpdatedAt       string         `db:"updated_at"`
}

func (r editionRow) toDomain() domain.Edition {
	return domain.Edition{
		ID:              r.ID,
		WorkID:          r.WorkID,
		EditionType:     r.EditionType,
		Format:          r.Format.String,
		ISBN:            r.ISBN.String,
		Publisher:       r.Publisher.String,
		PublishedYear:   int(r.PublishedYear.Int64),
		Narrator:        r.Narrator.String,
		DurationSeconds: int(r.DurationSeconds.Int64),
		FilePath:        r.FilePath.String,
		FileHash:        r.FileHash.String,
		FileSize:        r.FileSize.Int64,
		CoverPath:       r.CoverPath.String,
		Notes:           r.Notes.String,
		Status:          r.Status,
		CreatedAt:       parseDBTime(r.CreatedAt),
		UpdatedAt:       parseDBTime(r.UpdatedAt),
	}
}

const editionColumns = `id, work_id, edition_type, format, isbn, publisher, published_year,
	narrator, duration_seconds, file_path, file_hash, file_size,
	cover_path, notes, status, created_at, updated_at`

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
	return s.scanEdition("SELECT "+editionColumns+" FROM editions WHERE id = ?", id)
}

func (s *SQLiteStore) ListEditions(workID int64) ([]domain.Edition, error) {
	var rows []editionRow
	if err := s.db.Select(&rows, `SELECT `+editionColumns+`
		FROM editions WHERE work_id = ? AND status = 'active'
		ORDER BY edition_type, format`, workID); err != nil {
		return nil, err
	}

	editions := make([]domain.Edition, 0, len(rows))
	for _, row := range rows {
		editions = append(editions, row.toDomain())
	}
	return editions, nil
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
	e, err := s.scanEdition("SELECT "+editionColumns+" FROM editions WHERE file_hash = ? AND status = 'active' LIMIT 1", hash)
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
	var row editionRow
	if err := s.db.Get(&row, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, notFound("edition", "")
		}
		return nil, err
	}
	e := row.toDomain()
	return &e, nil
}
