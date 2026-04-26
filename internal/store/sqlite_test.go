package store

import "testing"

// newTestStore creates an in-memory SQLiteStore for testing.
func newTestStore(t *testing.T) *SQLiteStore {
	t.Helper()
	s, err := NewMemoryStore()
	if err != nil {
		t.Fatalf("NewMemoryStore: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}
