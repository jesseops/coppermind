package domain

import "time"

// Reading state statuses.
const (
	ReadingStatusUnread    = "unread"
	ReadingStatusReading   = "reading"
	ReadingStatusFinished  = "finished"
	ReadingStatusAbandoned = "abandoned"
)

// ReadingState tracks a user's progress through a specific edition.
type ReadingState struct {
	UserID          int64     `db:"user_id" json:"user_id"`
	EditionID       int64     `db:"edition_id" json:"edition_id"`
	Status          string    `db:"status" json:"status"`
	Progress        float64   `db:"progress" json:"progress"`
	ChapterIndex    int       `db:"chapter_index" json:"chapter_index,omitempty"`
	ScrollPosition  float64   `db:"scroll_position" json:"scroll_position,omitempty"`
	TrackIndex      int       `db:"track_index" json:"track_index,omitempty"`
	PositionSeconds float64   `db:"position_seconds" json:"position_seconds,omitempty"`
	StartedAt       time.Time `db:"started_at" json:"started_at,omitempty"`
	FinishedAt      time.Time `db:"finished_at" json:"finished_at,omitempty"`
	UpdatedAt       time.Time `db:"updated_at" json:"updated_at"`

	// Denormalized for views.
	WorkTitle  string `db:"-" json:"work_title,omitempty"`
	CoverPath  string `db:"-" json:"cover_path,omitempty"`
	WorkID     int64  `db:"-" json:"work_id,omitempty"`
	AuthorName string `db:"-" json:"author_name,omitempty"`
}

// UserRating represents a user's rating and optional review of a work.
type UserRating struct {
	UserID    int64     `db:"user_id" json:"user_id"`
	WorkID    int64     `db:"work_id" json:"work_id"`
	Rating    int       `db:"rating" json:"rating"`
	Review    string    `db:"review" json:"review,omitempty"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}
