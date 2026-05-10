package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jesseops/coppermind/internal/domain"
)

func (s *SQLiteStore) CreateTrack(t *domain.Track) error {
	res, err := s.db.Exec(`
		INSERT INTO tracks (edition_id, track_index, title, duration_seconds, file_path, file_hash)
		VALUES (?, ?, ?, ?, ?, ?)`,
		t.EditionID, t.TrackIndex,
		nullOrEmpty(t.Title), nullOrZeroInt(t.DurationSeconds),
		t.FilePath, nullOrEmpty(t.FileHash),
	)
	if err != nil {
		return fmt.Errorf("insert track: %w", err)
	}
	t.ID, _ = res.LastInsertId()
	return nil
}

func (s *SQLiteStore) GetTrack(id int64) (*domain.Track, error) {
	var t domain.Track
	var title, fileHash sql.NullString
	var duration sql.NullInt64
	var createdAt string

	err := s.db.QueryRow(`
		SELECT id, edition_id, track_index, title, duration_seconds, file_path, file_hash, created_at
		FROM tracks WHERE id = ?`, id).Scan(
		&t.ID, &t.EditionID, &t.TrackIndex, &title, &duration,
		&t.FilePath, &fileHash, &createdAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, notFound("track", id)
		}
		return nil, err
	}
	t.Title = title.String
	t.DurationSeconds = int(duration.Int64)
	t.FileHash = fileHash.String
	t.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	return &t, nil
}

func (s *SQLiteStore) ListTracks(editionID int64) ([]domain.Track, error) {
	rows, err := s.db.Query(`
		SELECT id, edition_id, track_index, title, duration_seconds, file_path, file_hash, created_at
		FROM tracks WHERE edition_id = ? ORDER BY track_index`, editionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tracks []domain.Track
	for rows.Next() {
		var t domain.Track
		var title, fileHash sql.NullString
		var duration sql.NullInt64
		var createdAt string
		if err := rows.Scan(
			&t.ID, &t.EditionID, &t.TrackIndex, &title, &duration,
			&t.FilePath, &fileHash, &createdAt,
		); err != nil {
			return nil, err
		}
		t.Title = title.String
		t.DurationSeconds = int(duration.Int64)
		t.FileHash = fileHash.String
		t.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		tracks = append(tracks, t)
	}
	return tracks, rows.Err()
}

func (s *SQLiteStore) ReplaceTracksForEdition(editionID int64, tracks []domain.Track) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}

	if _, err := tx.Exec("DELETE FROM tracks WHERE edition_id = ?", editionID); err != nil {
		_ = tx.Rollback()
		return err
	}

	for i := range tracks {
		tracks[i].EditionID = editionID
		_, err := tx.Exec(`
			INSERT INTO tracks (edition_id, track_index, title, duration_seconds, file_path, file_hash)
			VALUES (?, ?, ?, ?, ?, ?)`,
			tracks[i].EditionID, tracks[i].TrackIndex,
			nullOrEmpty(tracks[i].Title), nullOrZeroInt(tracks[i].DurationSeconds),
			tracks[i].FilePath, nullOrEmpty(tracks[i].FileHash),
		)
		if err != nil {
			_ = tx.Rollback()
			return err
		}
	}

	return tx.Commit()
}
