package store

import (
	"database/sql"
	"errors"
	"time"

	"github.com/jesseops/coppermind/internal/domain"
)

func (s *SQLiteStore) GetReadingState(userID, editionID int64) (*domain.ReadingState, error) {
	var rs domain.ReadingState
	var startedAt, finishedAt, updatedAt sql.NullString
	var chapterIndex, trackIndex sql.NullInt64
	var scrollPos, posSeconds sql.NullFloat64

	err := s.db.QueryRow(`
		SELECT user_id, edition_id, status, progress,
		       chapter_index, scroll_position, track_index, position_seconds,
		       started_at, finished_at, updated_at
		FROM reading_states WHERE user_id = ? AND edition_id = ?`,
		userID, editionID,
	).Scan(
		&rs.UserID, &rs.EditionID, &rs.Status, &rs.Progress,
		&chapterIndex, &scrollPos, &trackIndex, &posSeconds,
		&startedAt, &finishedAt, &updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	rs.ChapterIndex = int(chapterIndex.Int64)
	rs.ScrollPosition = scrollPos.Float64
	rs.TrackIndex = int(trackIndex.Int64)
	rs.PositionSeconds = posSeconds.Float64
	if startedAt.Valid {
		rs.StartedAt, _ = time.Parse("2006-01-02 15:04:05", startedAt.String)
	}
	if finishedAt.Valid {
		rs.FinishedAt, _ = time.Parse("2006-01-02 15:04:05", finishedAt.String)
	}
	if updatedAt.Valid {
		rs.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt.String)
	}
	return &rs, nil
}

func (s *SQLiteStore) SaveReadingState(state *domain.ReadingState) error {
	var startedAt, finishedAt any
	if !state.StartedAt.IsZero() {
		startedAt = state.StartedAt.UTC().Format("2006-01-02 15:04:05")
	}
	if !state.FinishedAt.IsZero() {
		finishedAt = state.FinishedAt.UTC().Format("2006-01-02 15:04:05")
	}

	_, err := s.db.Exec(`
		INSERT INTO reading_states (user_id, edition_id, status, progress,
		                            chapter_index, scroll_position, track_index, position_seconds,
		                            started_at, finished_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'))
		ON CONFLICT(user_id, edition_id) DO UPDATE SET
			status = excluded.status,
			progress = excluded.progress,
			chapter_index = excluded.chapter_index,
			scroll_position = excluded.scroll_position,
			track_index = excluded.track_index,
			position_seconds = excluded.position_seconds,
			started_at = excluded.started_at,
			finished_at = excluded.finished_at,
			updated_at = datetime('now')`,
		state.UserID, state.EditionID, state.Status, state.Progress,
		nullOrZeroInt(state.ChapterIndex), nullOrZeroFloat(state.ScrollPosition),
		nullOrZeroInt(state.TrackIndex), nullOrZeroFloat(state.PositionSeconds),
		startedAt, finishedAt,
	)
	return err
}

func (s *SQLiteStore) ListReadingStates(userID int64, status string) ([]domain.ReadingState, error) {
	q := `
		SELECT rs.user_id, rs.edition_id, rs.status, rs.progress,
		       rs.chapter_index, rs.scroll_position, rs.track_index, rs.position_seconds,
		       rs.started_at, rs.finished_at, rs.updated_at,
		       w.title, w.cover_path, w.id
		FROM reading_states rs
		JOIN editions e ON e.id = rs.edition_id
		JOIN works w ON w.id = e.work_id
		WHERE rs.user_id = ?`
	args := []any{userID}
	if status != "" {
		q += " AND rs.status = ?"
		args = append(args, status)
	}
	q += " ORDER BY rs.updated_at DESC"

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var states []domain.ReadingState
	for rows.Next() {
		var rs domain.ReadingState
		var chapterIndex, trackIndex sql.NullInt64
		var scrollPos, posSeconds sql.NullFloat64
		var startedAt, finishedAt, updatedAt sql.NullString
		var workTitle, coverPath sql.NullString
		var workID int64

		if err := rows.Scan(
			&rs.UserID, &rs.EditionID, &rs.Status, &rs.Progress,
			&chapterIndex, &scrollPos, &trackIndex, &posSeconds,
			&startedAt, &finishedAt, &updatedAt,
			&workTitle, &coverPath, &workID,
		); err != nil {
			return nil, err
		}

		rs.ChapterIndex = int(chapterIndex.Int64)
		rs.ScrollPosition = scrollPos.Float64
		rs.TrackIndex = int(trackIndex.Int64)
		rs.PositionSeconds = posSeconds.Float64
		rs.WorkTitle = workTitle.String
		rs.CoverPath = coverPath.String
		rs.WorkID = workID
		if startedAt.Valid {
			rs.StartedAt, _ = time.Parse("2006-01-02 15:04:05", startedAt.String)
		}
		if finishedAt.Valid {
			rs.FinishedAt, _ = time.Parse("2006-01-02 15:04:05", finishedAt.String)
		}
		if updatedAt.Valid {
			rs.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt.String)
		}
		states = append(states, rs)
	}
	return states, rows.Err()
}

// ── Ratings ─────────────────────────────────────────────────────────

func (s *SQLiteStore) SaveRating(userID, workID int64, rating int, review string) error {
	_, err := s.db.Exec(`
		INSERT INTO user_ratings (user_id, work_id, rating, review)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(user_id, work_id) DO UPDATE SET
			rating = excluded.rating,
			review = excluded.review`,
		userID, workID, rating, nullOrEmpty(review),
	)
	return err
}

func (s *SQLiteStore) GetRating(userID, workID int64) (*domain.UserRating, error) {
	var ur domain.UserRating
	var review sql.NullString
	var rating sql.NullInt64
	var createdAt string

	err := s.db.QueryRow(
		"SELECT user_id, work_id, rating, review, created_at FROM user_ratings WHERE user_id = ? AND work_id = ?",
		userID, workID,
	).Scan(&ur.UserID, &ur.WorkID, &rating, &review, &createdAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	ur.Rating = int(rating.Int64)
	ur.Review = review.String
	ur.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	return &ur, nil
}

func (s *SQLiteStore) ListRatings(workID int64) ([]domain.UserRating, error) {
	rows, err := s.db.Query(
		"SELECT user_id, work_id, rating, review, created_at FROM user_ratings WHERE work_id = ? ORDER BY created_at DESC",
		workID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ratings []domain.UserRating
	for rows.Next() {
		var ur domain.UserRating
		var review sql.NullString
		var rating sql.NullInt64
		var createdAt string
		if err := rows.Scan(&ur.UserID, &ur.WorkID, &rating, &review, &createdAt); err != nil {
			return nil, err
		}
		ur.Rating = int(rating.Int64)
		ur.Review = review.String
		ur.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		ratings = append(ratings, ur)
	}
	return ratings, rows.Err()
}
