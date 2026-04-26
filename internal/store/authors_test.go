package store

import (
	"testing"

	"github.com/jesseops/coppermind/internal/domain"
)

func TestAuthorCRUD(t *testing.T) {
	s := newTestStore(t)

	a, err := s.CreateAuthor("Brandon Sanderson", "sanderson, brandon")
	if err != nil {
		t.Fatalf("CreateAuthor: %v", err)
	}
	if a.Name != "Brandon Sanderson" || a.SortName != "sanderson, brandon" {
		t.Errorf("unexpected author: %+v", a)
	}

	// Get.
	got, err := s.GetAuthor(a.ID)
	if err != nil {
		t.Fatalf("GetAuthor: %v", err)
	}
	if got.Name != "Brandon Sanderson" {
		t.Errorf("Name = %q", got.Name)
	}

	// Find by sort name.
	found, err := s.FindAuthorBySortName("sanderson, brandon")
	if err != nil {
		t.Fatalf("FindAuthorBySortName: %v", err)
	}
	if found == nil || found.ID != a.ID {
		t.Errorf("expected to find author by sort_name")
	}

	// Not found.
	notFound, err := s.FindAuthorBySortName("nobody, nobody")
	if err != nil {
		t.Fatal(err)
	}
	if notFound != nil {
		t.Error("expected nil for non-existent sort_name")
	}
}

func TestListAuthorsWithCounts(t *testing.T) {
	s := newTestStore(t)
	lib, _ := s.CreateLibrary("Test")
	author, _ := s.CreateAuthor("Author A", "a, author")

	// No works yet.
	authors, err := s.ListAuthors(lib.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(authors) != 0 {
		t.Errorf("expected 0 authors, got %d", len(authors))
	}

	// Add a work and link.
	w := &domain.Work{LibraryID: lib.ID, Title: "Book One"}
	s.CreateWork(w)
	s.LinkWorkAuthor(w.ID, author.ID, "author")

	authors, _ = s.ListAuthors(lib.ID)
	if len(authors) != 1 || authors[0].WorkCount != 1 {
		t.Errorf("expected 1 author with count=1, got %+v", authors)
	}
}
