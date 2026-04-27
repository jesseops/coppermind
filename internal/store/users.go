package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/jesseops/coppermind/internal/domain"
)

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
	return s.scanUser("SELECT id, username, display_name, password_hash, role, kindle_email, created_at, updated_at FROM users WHERE id = ?", id)
}

func (s *SQLiteStore) GetUserByUsername(username string) (*domain.User, error) {
	return s.scanUser("SELECT id, username, display_name, password_hash, role, kindle_email, created_at, updated_at FROM users WHERE username = ?", username)
}

func (s *SQLiteStore) ListUsers() ([]domain.User, error) {
	rows, err := s.db.Query("SELECT id, username, display_name, password_hash, role, kindle_email, created_at, updated_at FROM users ORDER BY username")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []domain.User
	for rows.Next() {
		u, err := scanUserRow(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, *u)
	}
	return users, rows.Err()
}

func (s *SQLiteStore) UpdateUser(id int64, updates UserUpdate) error {
	clauses := []string{}
	args := []any{}

	if updates.DisplayName != nil {
		clauses = append(clauses, "display_name = ?")
		args = append(args, *updates.DisplayName)
	}
	if updates.PasswordHash != nil {
		clauses = append(clauses, "password_hash = ?")
		args = append(args, *updates.PasswordHash)
	}
	if updates.Role != nil {
		clauses = append(clauses, "role = ?")
		args = append(args, *updates.Role)
	}
	if updates.KindleEmail != nil {
		clauses = append(clauses, "kindle_email = ?")
		args = append(args, *updates.KindleEmail)
	}

	if len(clauses) == 0 {
		return nil
	}

	clauses = append(clauses, "updated_at = datetime('now')")
	args = append(args, id)
	q := "UPDATE users SET " + strings.Join(clauses, ", ") + " WHERE id = ?"
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
	var u domain.User
	var createdAt, updatedAt string
	err := s.db.QueryRow(query, args...).Scan(
		&u.ID, &u.Username, &u.DisplayName, &u.PasswordHash, &u.Role, &u.KindleEmail,
		&createdAt, &updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, notFound("user", "")
		}
		return nil, err
	}
	u.CreatedAt = parseDBTime(createdAt)
	u.UpdatedAt = parseDBTime(updatedAt)
	return &u, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanUserRow(row rowScanner) (*domain.User, error) {
	var u domain.User
	var createdAt, updatedAt string
	err := row.Scan(
		&u.ID, &u.Username, &u.DisplayName, &u.PasswordHash, &u.Role, &u.KindleEmail,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}
	u.CreatedAt = parseDBTime(createdAt)
	u.UpdatedAt = parseDBTime(updatedAt)
	return &u, nil
}
