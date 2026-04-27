package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jesseops/coppermind/internal/domain"
)

func (s *SQLiteStore) CreateWork(w *domain.Work) error {
	if w.SortTitle == "" {
		w.SortTitle = domain.GenerateSortTitle(w.Title)
	}
	res, err := s.db.Exec(`
		INSERT INTO works (library_id, title, sort_title, description, series_id, series_index,
		                    language, first_published, cover_path)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		w.LibraryID, w.Title, w.SortTitle,
		nullOrEmpty(w.Description),
		nullOrZeroInt64(w.SeriesID),
		nullOrZeroFloat(w.SeriesIndex),
		nullOrEmpty(w.Language),
		nullOrZeroInt(w.FirstPublished),
		nullOrEmpty(w.CoverPath),
	)
	if err != nil {
		return fmt.Errorf("insert work: %w", err)
	}
	w.ID, _ = res.LastInsertId()
	return nil
}

func (s *SQLiteStore) GetWork(id int64) (*domain.Work, error) {
	w, err := s.scanWork(
		`SELECT id, library_id, title, sort_title, description, series_id, series_index,
		        language, first_published, cover_path, created_at, updated_at, hidden
		 FROM works WHERE id = ?`, id)
	if err != nil {
		return nil, err
	}

	// Populate authors.
	authors, err := s.GetWorkAuthors(id)
	if err != nil {
		return nil, err
	}
	w.Authors = authors

	// Populate series name.
	if w.SeriesID > 0 {
		ser, err := s.GetSeries(w.SeriesID)
		if err == nil {
			w.SeriesName = ser.Name
		}
	}

	// Populate edition info.
	editions, err := s.ListEditions(id)
	if err != nil {
		return nil, err
	}
	w.EditionCount = len(editions)
	for _, e := range editions {
		if e.EditionType == domain.EditionTypeEbook {
			w.HasEbook = true
		}
		if e.EditionType == domain.EditionTypeAudiobook {
			w.HasAudiobook = true
		}
	}

	return w, nil
}

func (s *SQLiteStore) UpdateWork(id int64, updates WorkUpdate) error {
	clauses := []string{}
	args := []any{}

	if updates.Title != nil {
		title := strings.TrimSpace(*updates.Title)
		clauses = append(clauses, "title = ?", "sort_title = ?")
		args = append(args, title, domain.GenerateSortTitle(title))
	}
	if updates.Description != nil {
		clauses = append(clauses, "description = ?")
		args = append(args, nullOrEmpty(*updates.Description))
	}
	if updates.SeriesID != nil {
		clauses = append(clauses, "series_id = ?")
		args = append(args, nullOrZeroInt64(*updates.SeriesID))
	}
	if updates.SeriesIndex != nil {
		clauses = append(clauses, "series_index = ?")
		args = append(args, *updates.SeriesIndex)
	}
	if updates.Language != nil {
		clauses = append(clauses, "language = ?")
		args = append(args, nullOrEmpty(*updates.Language))
	}
	if updates.FirstPublished != nil {
		clauses = append(clauses, "first_published = ?")
		args = append(args, nullOrZeroInt(*updates.FirstPublished))
	}
	if updates.CoverPath != nil {
		clauses = append(clauses, "cover_path = ?")
		args = append(args, nullOrEmpty(*updates.CoverPath))
	}
	if updates.Hidden != nil {
		h := 0
		if *updates.Hidden {
			h = 1
		}
		clauses = append(clauses, "hidden = ?")
		args = append(args, h)
	}

	if len(clauses) == 0 {
		return nil
	}

	clauses = append(clauses, "updated_at = datetime('now')")
	args = append(args, id)
	q := "UPDATE works SET " + strings.Join(clauses, ", ") + " WHERE id = ?"
	res, err := s.db.Exec(q, args...)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("work %d not found", id)
	}
	return nil
}

func (s *SQLiteStore) DeleteWork(id int64) error {
	// CASCADE will handle editions, work_authors, work_tags, shelf_works.
	res, err := s.db.Exec("DELETE FROM works WHERE id = ?", id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("work %d not found", id)
	}
	return nil
}

// MergeWorks moves all editions and relationships from source works into targetID,
// then deletes the source works. Target metadata is preserved, with empty optional
// fields filled from source works when possible.
func (s *SQLiteStore) MergeWorks(targetID int64, sourceIDs []int64) error {
	if targetID == 0 {
		return fmt.Errorf("target work required")
	}
	if len(sourceIDs) == 0 {
		return nil
	}

	tx, err := s.db.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var targetLibraryID int64
	if err := tx.QueryRow("SELECT library_id FROM works WHERE id = ?", targetID).Scan(&targetLibraryID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("target work %d not found", targetID)
		}
		return err
	}

	for _, sourceID := range sourceIDs {
		if sourceID == 0 || sourceID == targetID {
			continue
		}

		var sourceLibraryID int64
		if err := tx.QueryRow("SELECT library_id FROM works WHERE id = ?", sourceID).Scan(&sourceLibraryID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("source work %d not found", sourceID)
			}
			return err
		}
		if sourceLibraryID != targetLibraryID {
			return fmt.Errorf("cannot merge works from different libraries")
		}

		// Fill missing optional metadata on the target from the source before deleting it.
		if _, err := tx.Exec(`
			UPDATE works SET
				description = CASE WHEN description IS NULL OR description = '' THEN (SELECT description FROM works WHERE id = ?) ELSE description END,
				series_id = CASE WHEN series_id IS NULL THEN (SELECT series_id FROM works WHERE id = ?) ELSE series_id END,
				series_index = CASE WHEN series_index IS NULL THEN (SELECT series_index FROM works WHERE id = ?) ELSE series_index END,
				language = CASE WHEN language IS NULL OR language = '' THEN (SELECT language FROM works WHERE id = ?) ELSE language END,
				first_published = CASE WHEN first_published IS NULL THEN (SELECT first_published FROM works WHERE id = ?) ELSE first_published END,
				cover_path = CASE WHEN cover_path IS NULL OR cover_path = '' THEN (SELECT cover_path FROM works WHERE id = ?) ELSE cover_path END,
				updated_at = datetime('now')
			WHERE id = ?`, sourceID, sourceID, sourceID, sourceID, sourceID, sourceID, targetID); err != nil {
			return err
		}

		if _, err := tx.Exec("UPDATE editions SET work_id = ?, updated_at = datetime('now') WHERE work_id = ?", targetID, sourceID); err != nil {
			return err
		}
		if _, err := tx.Exec("INSERT OR IGNORE INTO work_authors (work_id, author_id, role) SELECT ?, author_id, role FROM work_authors WHERE work_id = ?", targetID, sourceID); err != nil {
			return err
		}
		if _, err := tx.Exec("INSERT OR IGNORE INTO work_tags (work_id, tag) SELECT ?, tag FROM work_tags WHERE work_id = ?", targetID, sourceID); err != nil {
			return err
		}
		if _, err := tx.Exec("INSERT OR IGNORE INTO shelf_works (shelf_id, work_id, added_at) SELECT shelf_id, ?, added_at FROM shelf_works WHERE work_id = ?", targetID, sourceID); err != nil {
			return err
		}
		if _, err := tx.Exec("INSERT OR IGNORE INTO user_ratings (user_id, work_id, rating, review, created_at) SELECT user_id, ?, rating, review, created_at FROM user_ratings WHERE work_id = ?", targetID, sourceID); err != nil {
			return err
		}
		if _, err := tx.Exec("DELETE FROM works WHERE id = ?", sourceID); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *SQLiteStore) FindWorkByTitleAndAuthor(libraryID int64, sortTitle, authorSortName string) (*domain.Work, error) {
	var workID int64
	err := s.db.QueryRow(`
		SELECT w.id FROM works w
		JOIN work_authors wa ON wa.work_id = w.id
		JOIN authors a ON a.id = wa.author_id
		WHERE w.library_id = ? AND w.sort_title = ? AND a.sort_name = ?
		LIMIT 1`, libraryID, sortTitle, authorSortName).Scan(&workID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return s.GetWork(workID)
}

func (s *SQLiteStore) ListWorks(filter WorkFilter) ([]domain.Work, int, error) {
	where := []string{"w.library_id = ?"}
	args := []any{filter.LibraryID}

	if filter.Query != "" {
		pattern := "%" + filter.Query + "%"
		where = append(where, `(w.title LIKE ? OR w.sort_title LIKE ? OR EXISTS (
			SELECT 1 FROM work_authors wa2 JOIN authors a2 ON a2.id = wa2.author_id
			WHERE wa2.work_id = w.id AND a2.name LIKE ?
		))`)
		args = append(args, pattern, pattern, pattern)
	}
	if filter.Type != "" {
		where = append(where, `EXISTS (
			SELECT 1 FROM editions e WHERE e.work_id = w.id AND e.edition_type = ? AND e.status = 'active'
		)`)
		args = append(args, filter.Type)
	}
	if filter.SeriesID > 0 {
		where = append(where, "w.series_id = ?")
		args = append(args, filter.SeriesID)
	}
	if filter.AuthorID > 0 {
		where = append(where, `EXISTS (
			SELECT 1 FROM work_authors wa3 WHERE wa3.work_id = w.id AND wa3.author_id = ?
		)`)
		args = append(args, filter.AuthorID)
	}
	if filter.ShelfID > 0 {
		where = append(where, `EXISTS (
			SELECT 1 FROM shelf_works sw WHERE sw.work_id = w.id AND sw.shelf_id = ?
		)`)
		args = append(args, filter.ShelfID)
	}
	if !filter.IncludeHidden && !filter.OnlyHidden {
		where = append(where, "w.hidden = 0")
	}
	if filter.OnlyHidden {
		where = append(where, "w.hidden = 1")
	}
	if filter.MissingCover {
		where = append(where, "(w.cover_path IS NULL OR w.cover_path = '')")
	}
	if filter.MissingAuthor {
		where = append(where, `NOT EXISTS (
			SELECT 1 FROM work_authors wa4 WHERE wa4.work_id = w.id
		)`)
	}
	if filter.MissingDesc {
		where = append(where, "(w.description IS NULL OR w.description = '')")
	}
	if filter.Format != "" {
		where = append(where, `EXISTS (
			SELECT 1 FROM editions e2 WHERE e2.work_id = w.id AND e2.format = ? AND e2.status = 'active'
		)`)
		args = append(args, filter.Format)
	}

	whereClause := strings.Join(where, " AND ")

	// Count.
	countQ := "SELECT COUNT(*) FROM works w WHERE " + whereClause
	var total int
	if err := s.db.QueryRow(countQ, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Sort.
	orderBy := "w.created_at DESC"
	switch strings.ToLower(filter.SortBy) {
	case "title":
		orderBy = "w.sort_title ASC"
	case "author":
		orderBy = "w.sort_title ASC" // will be overridden by grouping in handler
	case "updated_at":
		orderBy = "w.updated_at DESC"
	case "series":
		orderBy = "w.series_id, w.series_index"
	case "year":
		orderBy = "w.first_published DESC"
	}
	if strings.EqualFold(filter.SortOrder, "asc") && !strings.Contains(orderBy, "ASC") && !strings.Contains(orderBy, "DESC") {
		orderBy += " ASC"
	} else if strings.EqualFold(filter.SortOrder, "desc") && !strings.Contains(orderBy, "ASC") && !strings.Contains(orderBy, "DESC") {
		orderBy += " DESC"
	}

	q := fmt.Sprintf(`
		SELECT w.id, w.library_id, w.title, w.sort_title, w.description, w.series_id, w.series_index,
		       w.language, w.first_published, w.cover_path, w.created_at, w.updated_at, w.hidden
		FROM works w
		WHERE %s
		ORDER BY %s`, whereClause, orderBy)

	if filter.Limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", filter.Limit)
	}
	if filter.Offset > 0 {
		q += fmt.Sprintf(" OFFSET %d", filter.Offset)
	}

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, 0, err
	}

	// Collect all works first, then close rows before doing sub-queries.
	// This avoids deadlock with MaxOpenConns(1).
	var works []domain.Work
	for rows.Next() {
		w, err := scanWorkRow(rows)
		if err != nil {
			rows.Close()
			return nil, 0, err
		}
		works = append(works, *w)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, 0, err
	}
	rows.Close()

	// Now populate denormalized fields with the connection free.
	for i := range works {
		w := &works[i]
		authors, _ := s.GetWorkAuthors(w.ID)
		w.Authors = authors
		if w.SeriesID > 0 {
			if ser, err := s.GetSeries(w.SeriesID); err == nil {
				w.SeriesName = ser.Name
			}
		}
		// Edition type flags.
		edRows, _ := s.db.Query(
			"SELECT DISTINCT edition_type FROM editions WHERE work_id = ? AND status = 'active'", w.ID)
		if edRows != nil {
			for edRows.Next() {
				var et string
				edRows.Scan(&et)
				if et == domain.EditionTypeEbook {
					w.HasEbook = true
				}
				if et == domain.EditionTypeAudiobook {
					w.HasAudiobook = true
				}
			}
			edRows.Close()
		}
	}
	return works, total, nil
}

// ── scan helpers ────────────────────────────────────────────────────

func (s *SQLiteStore) scanWork(query string, args ...any) (*domain.Work, error) {
	row := s.db.QueryRow(query, args...)
	w, err := scanWorkFromRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("work not found")
		}
		return nil, err
	}
	return w, nil
}

type singleRowScanner interface {
	Scan(dest ...any) error
}

func scanWorkFromRow(row singleRowScanner) (*domain.Work, error) {
	var w domain.Work
	var desc, language, coverPath sql.NullString
	var seriesID sql.NullInt64
	var seriesIndex sql.NullFloat64
	var firstPub sql.NullInt64
	var hidden int
	var createdAt, updatedAt string

	err := row.Scan(
		&w.ID, &w.LibraryID, &w.Title, &w.SortTitle,
		&desc, &seriesID, &seriesIndex,
		&language, &firstPub, &coverPath,
		&createdAt, &updatedAt, &hidden,
	)
	if err != nil {
		return nil, err
	}

	w.Description = desc.String
	w.SeriesID = seriesID.Int64
	w.SeriesIndex = seriesIndex.Float64
	w.Language = language.String
	w.FirstPublished = int(firstPub.Int64)
	w.CoverPath = coverPath.String
	w.Hidden = hidden != 0
	w.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	w.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
	return &w, nil
}

func scanWorkRow(rows *sql.Rows) (*domain.Work, error) {
	var w domain.Work
	var desc, language, coverPath sql.NullString
	var seriesID sql.NullInt64
	var seriesIndex sql.NullFloat64
	var firstPub sql.NullInt64
	var hidden int
	var createdAt, updatedAt string

	err := rows.Scan(
		&w.ID, &w.LibraryID, &w.Title, &w.SortTitle,
		&desc, &seriesID, &seriesIndex,
		&language, &firstPub, &coverPath,
		&createdAt, &updatedAt, &hidden,
	)
	if err != nil {
		return nil, err
	}

	w.Description = desc.String
	w.SeriesID = seriesID.Int64
	w.SeriesIndex = seriesIndex.Float64
	w.Language = language.String
	w.FirstPublished = int(firstPub.Int64)
	w.CoverPath = coverPath.String
	w.Hidden = hidden != 0
	w.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	w.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
	return &w, nil
}
