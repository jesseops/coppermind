package store

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/jesseops/coppermind/internal/domain"
)

type userRow struct {
	ID           int64  `db:"id"`
	Username     string `db:"username"`
	DisplayName  string `db:"display_name"`
	PasswordHash string `db:"password_hash"`
	Role         string `db:"role"`
	KindleEmail  string `db:"kindle_email"`
	CreatedAt    string `db:"created_at"`
	UpdatedAt    string `db:"updated_at"`
}

func (r userRow) toDomain() domain.User {
	return domain.User{
		ID:           r.ID,
		Username:     r.Username,
		DisplayName:  r.DisplayName,
		PasswordHash: r.PasswordHash,
		Role:         r.Role,
		KindleEmail:  r.KindleEmail,
		CreatedAt:    parseDBTime(r.CreatedAt),
		UpdatedAt:    parseDBTime(r.UpdatedAt),
	}
}

const userColumns = `id, username, display_name, password_hash, role, kindle_email, created_at, updated_at`

func (s *SQLiteStore) CreateUser(username, displayName, passwordHash, role string) (*domain.User, error) {
	res, err := s.db.Exec(
		`INSERT INTO users (username, display_name, password_hash, role) VALUES (?, ?, ?, ?)`,
		username, displayName, passwordHash, role,
	)
	if err != nil {
		return nil, fmt.Errorf("insert user: %w", err)
	}
	id, _ := res.LastInsertId()
	return s.GetUser(id)
}

func (s *SQLiteStore) GetUser(id int64) (*domain.User, error) {
	return s.scanUser("SELECT "+userColumns+" FROM users WHERE id = ?", id)
}

func (s *SQLiteStore) GetUserByUsername(username string) (*domain.User, error) {
	return s.scanUser("SELECT "+userColumns+" FROM users WHERE username = ?", username)
}

func (s *SQLiteStore) ListUsers() ([]domain.User, error) {
	var rows []userRow
	if err := s.db.Select(&rows, "SELECT "+userColumns+" FROM users ORDER BY username"); err != nil {
		return nil, err
	}

	users := make([]domain.User, 0, len(rows))
	for _, row := range rows {
		users = append(users, row.toDomain())
	}
	return users, nil
}

func (s *SQLiteStore) UpdateUser(id int64, updates UserUpdate) error {
	ub := newUpdateBuilder("users")

	if updates.DisplayName != nil {
		ub.Set("display_name", *updates.DisplayName)
	}
	if updates.PasswordHash != nil {
		ub.Set("password_hash", *updates.PasswordHash)
	}
	if updates.Role != nil {
		ub.Set("role", *updates.Role)
	}
	if updates.KindleEmail != nil {
		ub.Set("kindle_email", *updates.KindleEmail)
	}

	q, args, ok := ub.SQL("id = ?", id)
	if !ok {
		return nil
	}
	res, err := s.db.Exec(q, args...)
	if err != nil {
		return err
	}
	return checkRowsAffected(res, "user", id)
}

func (s *SQLiteStore) DeleteUser(id int64) error {
	res, err := s.db.Exec("DELETE FROM users WHERE id = ?", id)
	if err != nil {
		return err
	}
	return checkRowsAffected(res, "user", id)
}

func (s *SQLiteStore) CountUsers() (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	return count, err
}

// ── scan helpers ────────────────────────────────────────────────────

func (s *SQLiteStore) scanUser(query string, args ...any) (*domain.User, error) {
	var row userRow
	if err := s.db.Get(&row, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, notFound("user", "")
		}
		return nil, err
	}
	u := row.toDomain()
	return &u, nil
}
