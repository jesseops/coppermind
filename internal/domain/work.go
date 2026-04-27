package domain

import (
	"strings"
	"time"
)

// Work represents an intellectual work (e.g., "Dune by Frank Herbert").
// A work may have multiple editions (EPUB, MOBI, audiobook, etc.).
type Work struct {
	ID             int64     `db:"id" json:"id"`
	LibraryID      int64     `db:"library_id" json:"library_id"`
	Title          string    `db:"title" json:"title"`
	SortTitle      string    `db:"sort_title" json:"sort_title"`
	Description    string    `db:"description" json:"description,omitempty"`
	SeriesID       int64     `db:"series_id" json:"series_id,omitempty"`
	SeriesIndex    float64   `db:"series_index" json:"series_index,omitempty"`
	Language       string    `db:"language" json:"language,omitempty"`
	FirstPublished int       `db:"first_published" json:"first_published,omitempty"`
	CoverPath      string    `db:"cover_path" json:"cover_path,omitempty"`
	CreatedAt      time.Time `db:"created_at" json:"created_at"`
	UpdatedAt      time.Time `db:"updated_at" json:"updated_at"`
	Hidden         bool      `db:"hidden" json:"hidden,omitempty"`

	// Denormalized fields for list views — populated by queries, not stored in works table.
	Authors      []WorkAuthor `db:"-" json:"authors,omitempty"`
	SeriesName   string       `db:"-" json:"series_name,omitempty"`
	EditionCount int          `db:"-" json:"edition_count,omitempty"`
	HasEbook     bool         `db:"-" json:"has_ebook,omitempty"`
	HasAudiobook bool         `db:"-" json:"has_audiobook,omitempty"`
}

// HasCover returns true if the work has a cover image path set.
func (w *Work) HasCover() bool {
	return w.CoverPath != ""
}

// PrimaryAuthor returns the first author's name, or empty string if none.
func (w *Work) PrimaryAuthor() string {
	for _, wa := range w.Authors {
		if wa.Role == RoleAuthorOf || wa.Role == "" {
			return wa.AuthorName
		}
	}
	if len(w.Authors) > 0 {
		return w.Authors[0].AuthorName
	}
	return ""
}

// GenerateSortTitle strips leading articles and lowercases for sorting.
// "The Way of Kings" → "way of kings, the"
// "A Game of Thrones" → "game of thrones, a"
func GenerateSortTitle(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return ""
	}
	lower := strings.ToLower(title)
	for _, article := range []string{"the ", "a ", "an "} {
		if strings.HasPrefix(lower, article) {
			rest := title[len(article):]
			art := title[:len(article)-1] // without trailing space
			return strings.ToLower(rest + ", " + art)
		}
	}
	return lower
}
