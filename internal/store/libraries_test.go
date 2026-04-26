package store

import "testing"

func TestLibraryCRUD(t *testing.T) {
	s := newTestStore(t)

	// Create.
	lib, err := s.CreateLibrary("My Library")
	if err != nil {
		t.Fatalf("CreateLibrary: %v", err)
	}
	if lib.ID == 0 || lib.Name != "My Library" {
		t.Errorf("unexpected library: %+v", lib)
	}

	// Get.
	got, err := s.GetLibrary(lib.ID)
	if err != nil {
		t.Fatalf("GetLibrary: %v", err)
	}
	if got.Name != "My Library" {
		t.Errorf("Name = %q, want %q", got.Name, "My Library")
	}

	// List.
	libs, err := s.ListLibraries()
	if err != nil {
		t.Fatalf("ListLibraries: %v", err)
	}
	if len(libs) != 1 {
		t.Errorf("len(libs) = %d, want 1", len(libs))
	}

	// Not found.
	_, err = s.GetLibrary(999)
	if err == nil {
		t.Error("expected error for non-existent library")
	}
}
