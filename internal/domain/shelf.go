package domain

import "time"

// Shelf represents a user-created collection of works.
type Shelf struct {
	ID          int64     `db:"id" json:"id"`
	UserID      int64     `db:"user_id" json:"user_id"`
	Name        string    `db:"name" json:"name"`
	Description string    `db:"description" json:"description,omitempty"`
	IsPublic    bool      `db:"is_public" json:"is_public"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`

	// Denormalized.
	WorkCount int `db:"-" json:"work_count,omitempty"`
}

// ShelfWork represents the many-to-many join between shelves and works.
type ShelfWork struct {
	ShelfID int64     `db:"shelf_id" json:"shelf_id"`
	WorkID  int64     `db:"work_id" json:"work_id"`
	AddedAt time.Time `db:"added_at" json:"added_at"`
}
