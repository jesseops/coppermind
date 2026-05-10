package store

import (
	"errors"
	"testing"
)

func TestUserCRUD(t *testing.T) {
	s := newTestStore(t)

	// Create.
	u, err := s.CreateUser("alice", "Alice", "$2a$10$hash", "admin")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if u.Username != "alice" || u.Role != "admin" {
		t.Errorf("unexpected user: %+v", u)
	}

	// Get by ID.
	got, err := s.GetUser(u.ID)
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if got.DisplayName != "Alice" {
		t.Errorf("DisplayName = %q, want %q", got.DisplayName, "Alice")
	}

	// Get by username.
	got2, err := s.GetUserByUsername("alice")
	if err != nil {
		t.Fatalf("GetUserByUsername: %v", err)
	}
	if got2.ID != u.ID {
		t.Errorf("ID mismatch: %d != %d", got2.ID, u.ID)
	}

	// Update.
	newName := "Alice W."
	err = s.UpdateUser(u.ID, UserUpdate{DisplayName: &newName})
	if err != nil {
		t.Fatalf("UpdateUser: %v", err)
	}
	got3, _ := s.GetUser(u.ID)
	if got3.DisplayName != "Alice W." {
		t.Errorf("DisplayName = %q after update", got3.DisplayName)
	}

	// Count.
	count, err := s.CountUsers()
	if err != nil {
		t.Fatalf("CountUsers: %v", err)
	}
	if count != 1 {
		t.Errorf("count = %d, want 1", count)
	}

	// List.
	users, err := s.ListUsers()
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if len(users) != 1 {
		t.Errorf("len(users) = %d, want 1", len(users))
	}

	// Delete.
	err = s.DeleteUser(u.ID)
	if err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}
	count, _ = s.CountUsers()
	if count != 0 {
		t.Errorf("count = %d after delete, want 0", count)
	}
}

func TestDuplicateUsername(t *testing.T) {
	s := newTestStore(t)

	_, err := s.CreateUser("alice", "Alice", "hash", "viewer")
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.CreateUser("alice", "Alice2", "hash", "viewer")
	if err == nil {
		t.Error("expected error for duplicate username")
	}
}

func TestUserNotFound(t *testing.T) {
	s := newTestStore(t)

	_, err := s.GetUser(999)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetUser error = %v, want ErrNotFound", err)
	}
	_, err = s.GetUserByUsername("nobody")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetUserByUsername error = %v, want ErrNotFound", err)
	}
}
