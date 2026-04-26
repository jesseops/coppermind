package domain

import "time"

// Edition types.
const (
	EditionTypeEbook     = "ebook"
	EditionTypeAudiobook = "audiobook"
)

// Common formats.
const (
	FormatEPUB = "epub"
	FormatMOBI = "mobi"
	FormatPDF  = "pdf"
	FormatMP3  = "mp3"
	FormatM4B  = "m4b"
)

// Edition statuses.
const (
	EditionStatusActive  = "active"
	EditionStatusDeleted = "deleted"
)

// Edition represents a specific version of a work (e.g., "the 2019 Ace EPUB").
type Edition struct {
	ID              int64     `db:"id" json:"id"`
	WorkID          int64     `db:"work_id" json:"work_id"`
	EditionType     string    `db:"edition_type" json:"edition_type"`
	Format          string    `db:"format" json:"format,omitempty"`
	ISBN            string    `db:"isbn" json:"isbn,omitempty"`
	Publisher       string    `db:"publisher" json:"publisher,omitempty"`
	PublishedYear   int       `db:"published_year" json:"published_year,omitempty"`
	Narrator        string    `db:"narrator" json:"narrator,omitempty"`
	DurationSeconds int       `db:"duration_seconds" json:"duration_seconds,omitempty"`
	FilePath        string    `db:"file_path" json:"file_path,omitempty"`
	FileHash        string    `db:"file_hash" json:"file_hash,omitempty"`
	FileSize        int64     `db:"file_size" json:"file_size,omitempty"`
	CoverPath       string    `db:"cover_path" json:"cover_path,omitempty"`
	Notes           string    `db:"notes" json:"notes,omitempty"`
	Status          string    `db:"status" json:"status"`
	CreatedAt       time.Time `db:"created_at" json:"created_at"`
	UpdatedAt       time.Time `db:"updated_at" json:"updated_at"`
}

// IsEbook returns true if this is an ebook edition.
func (e *Edition) IsEbook() bool {
	return e.EditionType == EditionTypeEbook
}

// IsAudiobook returns true if this is an audiobook edition.
func (e *Edition) IsAudiobook() bool {
	return e.EditionType == EditionTypeAudiobook
}

// IsActive returns true if the edition has active status.
func (e *Edition) IsActive() bool {
	return e.Status == EditionStatusActive
}

// HasCover returns true if the edition has a cover image path set.
func (e *Edition) HasCover() bool {
	return e.CoverPath != ""
}
