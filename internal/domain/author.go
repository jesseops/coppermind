package domain

import (
	"strings"
	"time"
)

// Author represents a normalized author entity.
type Author struct {
	ID        int64     `db:"id" json:"id"`
	Name      string    `db:"name" json:"name"`
	SortName  string    `db:"sort_name" json:"sort_name"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// Author roles in the work_authors join table.
const (
	RoleAuthorOf    = "author"
	RoleNarratorOf  = "narrator"
	RoleEditorOf    = "editor"
	RoleTranslator  = "translator"
)

// WorkAuthor represents the many-to-many relationship between works and authors.
type WorkAuthor struct {
	WorkID   int64  `db:"work_id" json:"work_id"`
	AuthorID int64  `db:"author_id" json:"author_id"`
	Role     string `db:"role" json:"role"`

	// Denormalized for convenience in views — not stored in work_authors table.
	AuthorName string `db:"-" json:"author_name,omitempty"`
}

// GenerateSortName converts a display name to a sort name.
// "Brandon Sanderson" → "sanderson, brandon"
// "J.R.R. Tolkien" → "tolkien, j.r.r."
// Already comma-formatted names are lowercased as-is.
func GenerateSortName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}

	// If already in "Last, First" format, just lowercase.
	if strings.Contains(name, ",") {
		return strings.ToLower(strings.TrimSpace(name))
	}

	parts := strings.Fields(name)
	if len(parts) == 1 {
		return strings.ToLower(parts[0])
	}

	last := parts[len(parts)-1]
	first := strings.Join(parts[:len(parts)-1], " ")
	return strings.ToLower(last + ", " + first)
}
