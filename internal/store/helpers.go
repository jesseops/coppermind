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
