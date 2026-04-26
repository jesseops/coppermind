package domain

import (
	"strings"
	"time"
)

// User roles.
const (
	RoleAdmin  = "admin"
	RoleViewer = "viewer"
)

// User represents a registered user account.
type User struct {
	ID           int64     `db:"id" json:"id"`
	Username     string    `db:"username" json:"username"`
	DisplayName  string    `db:"display_name" json:"display_name"`
	PasswordHash string    `db:"password_hash" json:"-"`
	Role         string    `db:"role" json:"role"`
	KindleEmail  string    `db:"kindle_email" json:"kindle_email,omitempty"`
	CreatedAt    time.Time `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time `db:"updated_at" json:"updated_at"`
}

// IsAdmin returns true if the user has the admin role.
func (u *User) IsAdmin() bool {
	return strings.EqualFold(u.Role, RoleAdmin)
}

// IsViewer returns true if the user has the viewer role.
func (u *User) IsViewer() bool {
	return strings.EqualFold(u.Role, RoleViewer)
}
