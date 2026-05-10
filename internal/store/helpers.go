package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

const dbTimeLayout = "2006-01-02 15:04:05"

// ErrNotFound is returned by Get-style methods when a row does not exist.
var ErrNotFound = errors.New("not found")

func parseDBTime(s string) time.Time {
	t, _ := time.Parse(dbTimeLayout, s)
	return t
}

func notFound(entity string, id any) error {
	return fmt.Errorf("%s %v: %w", entity, id, ErrNotFound)
}

func checkRowsAffected(res sql.Result, entity string, id any) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return notFound(entity, id)
	}
	return nil
}

func nullOrEmpty(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

func nullOrZeroInt64(v int64) any {
	if v == 0 {
		return nil
	}
	return v
}

func nullOrZeroFloat(v float64) any {
	if v == 0 {
		return nil
	}
	return v
}

func nullOrZeroInt(v int) any {
	if v == 0 {
		return nil
	}
	return v
}

func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.TrimRight(strings.Repeat("?,", n), ",")
}

type whereBuilder struct {
	clauses []string
	args    []any
}

func (b *whereBuilder) And(clause string, args ...any) {
	b.clauses = append(b.clauses, clause)
	b.args = append(b.args, args...)
}

func (b *whereBuilder) SQL() string {
	return strings.Join(b.clauses, " AND ")
}

func (b *whereBuilder) Args() []any {
	return b.args
}

type updateBuilder struct {
	table   string
	clauses []string
	args    []any
}

func newUpdateBuilder(table string) *updateBuilder {
	return &updateBuilder{table: table}
}

func (b *updateBuilder) Set(column string, value any) {
	b.clauses = append(b.clauses, column+" = ?")
	b.args = append(b.args, value)
}

func (b *updateBuilder) SQL(where string, whereArgs ...any) (string, []any, bool) {
	if len(b.clauses) == 0 {
		return "", nil, false
	}
	clauses := append([]string{}, b.clauses...)
	clauses = append(clauses, "updated_at = datetime('now')")
	args := append([]any{}, b.args...)
	args = append(args, whereArgs...)
	return "UPDATE " + b.table + " SET " + strings.Join(clauses, ", ") + " WHERE " + where, args, true
}
