package domain

import "time"

// Track represents a single audio file within an audiobook edition.
type Track struct {
	ID              int64     `db:"id" json:"id"`
	EditionID       int64     `db:"edition_id" json:"edition_id"`
	TrackIndex      int       `db:"track_index" json:"track_index"`
	Title           string    `db:"title" json:"title,omitempty"`
	DurationSeconds int       `db:"duration_seconds" json:"duration_seconds,omitempty"`
	FilePath        string    `db:"file_path" json:"file_path"`
	FileHash        string    `db:"file_hash" json:"file_hash,omitempty"`
	CreatedAt       time.Time `db:"created_at" json:"created_at"`
}
