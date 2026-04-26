package store

import (
	"testing"

	"github.com/jesseops/coppermind/internal/domain"
)

func TestWorkCRUD(t *testing.T) {
	s := newTestStore(t)
	lib, _ := s.CreateLibrary("Test")

	w := &domain.Work{
		LibraryID: lib.ID,
		Title:     "The Way of Kings",
	}
	if err := s.CreateWork(w); err != nil {
		t.Fatalf("CreateWork: %v", err)
	}
	if w.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if w.SortTitle != "way of kings, the" {
		t.Errorf("SortTitle = %q, want %q", w.SortTitle, "way of kings, the")
	}

	// Get.
	got, err := s.GetWork(w.ID)
	if err != nil {
		t.Fatalf("GetWork: %v", err)
	}
	if got.Title != "The Way of Kings" {
		t.Errorf("Title = %q", got.Title)
	}

	// Update.
	newTitle := "Words of Radiance"
	err = s.UpdateWork(w.ID, WorkUpdate{Title: &newTitle})
	if err != nil {
		t.Fatalf("UpdateWork: %v", err)
	}
	got2, _ := s.GetWork(w.ID)
	if got2.Title != "Words of Radiance" {
		t.Errorf("Title after update = %q", got2.Title)
	}

	// Delete.
	err = s.DeleteWork(w.ID)
	if err != nil {
		t.Fatalf("DeleteWork: %v", err)
	}
	_, err = s.GetWork(w.ID)
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestListWorksFiltering(t *testing.T) {
	s := newTestStore(t)
	lib, _ := s.CreateLibrary("Test")

	// Create 3 works.
	for _, title := range []string{"Dune", "The Hobbit", "Neuromancer"} {
		w := &domain.Work{LibraryID: lib.ID, Title: title}
		s.CreateWork(w)
	}

	// List all.
	works, total, err := s.ListWorks(WorkFilter{LibraryID: lib.ID})
	if err != nil {
		t.Fatalf("ListWorks: %v", err)
	}
	if total != 3 || len(works) != 3 {
		t.Errorf("total=%d len=%d, want 3/3", total, len(works))
	}

	// Search.
	works, total, err = s.ListWorks(WorkFilter{LibraryID: lib.ID, Query: "dune"})
	if err != nil {
		t.Fatalf("ListWorks search: %v", err)
	}
	if total != 1 {
		t.Errorf("search total = %d, want 1", total)
	}

	// Pagination.
	works, _, err = s.ListWorks(WorkFilter{LibraryID: lib.ID, Limit: 2, Offset: 0})
	if err != nil {
		t.Fatal(err)
	}
	if len(works) != 2 {
		t.Errorf("paginated len = %d, want 2", len(works))
	}
}

func TestWorkAuthorRelationship(t *testing.T) {
	s := newTestStore(t)
	lib, _ := s.CreateLibrary("Test")

	author, _ := s.CreateAuthor("Brandon Sanderson", "sanderson, brandon")
	w := &domain.Work{LibraryID: lib.ID, Title: "Mistborn"}
	s.CreateWork(w)

	// Link.
	err := s.LinkWorkAuthor(w.ID, author.ID, "author")
	if err != nil {
		t.Fatalf("LinkWorkAuthor: %v", err)
	}

	// Get work with authors populated.
	got, _ := s.GetWork(w.ID)
	if len(got.Authors) != 1 || got.Authors[0].AuthorName != "Brandon Sanderson" {
		t.Errorf("unexpected authors: %+v", got.Authors)
	}

	// FindWorkByTitleAndAuthor.
	found, err := s.FindWorkByTitleAndAuthor(lib.ID, domain.GenerateSortTitle("Mistborn"), "sanderson, brandon")
	if err != nil {
		t.Fatalf("FindWorkByTitleAndAuthor: %v", err)
	}
	if found == nil || found.ID != w.ID {
		t.Errorf("expected to find work, got %v", found)
	}

	// Unlink.
	s.UnlinkWorkAuthor(w.ID, author.ID, "author")
	got2, _ := s.GetWork(w.ID)
	if len(got2.Authors) != 0 {
		t.Error("expected no authors after unlink")
	}
}
