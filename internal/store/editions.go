package store

import (
	"database/sql"
	"errors"
	"fmt"

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
	ub := newUpdateBuilder("editions")

	if updates.Format != nil {
		ub.Set("format", nullOrEmpty(*updates.Format))
	}
	if updates.ISBN != nil {
		ub.Set("isbn", nullOrEmpty(*updates.ISBN))
	}
	if updates.Publisher != nil {
		ub.Set("publisher", nullOrEmpty(*updates.Publisher))
	}
	if updates.PublishedYear != nil {
		ub.Set("published_year", nullOrZeroInt(*updates.PublishedYear))
	}
	if updates.Narrator != nil {
		ub.Set("narrator", nullOrEmpty(*updates.Narrator))
	}
	if updates.DurationSeconds != nil {
		ub.Set("duration_seconds", nullOrZeroInt(*updates.DurationSeconds))
	}
	if updates.FilePath != nil {
		ub.Set("file_path", nullOrEmpty(*updates.FilePath))
	}
	if updates.FileHash != nil {
		ub.Set("file_hash", nullOrEmpty(*updates.FileHash))
	}
	if updates.FileSize != nil {
		ub.Set("file_size", nullOrZeroInt64(*updates.FileSize))
	}
	if updates.CoverPath != nil {
		ub.Set("cover_path", nullOrEmpty(*updates.CoverPath))
	}
	if updates.Notes != nil {
		ub.Set("notes", nullOrEmpty(*updates.Notes))
	}
	if updates.Status != nil {
		ub.Set("status", *updates.Status)
	}

	q, args, ok := ub.SQL("id = ?", id)
	if !ok {
		return nil
	}
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
