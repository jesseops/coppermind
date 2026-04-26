package store

import (
	"testing"

	"github.com/jesseops/coppermind/internal/domain"
)

func TestShelfCRUD(t *testing.T) {
	s := newTestStore(t)
	lib, _ := s.CreateLibrary("Test")
	user, _ := s.CreateUser("alice", "Alice", "hash", "viewer")

	// Create shelf.
	sh, err := s.CreateShelf(user.ID, "Favorites", "My favorites", true)
	if err != nil {
		t.Fatal(err)
	}
	if sh.Name != "Favorites" || !sh.IsPublic {
		t.Errorf("unexpected shelf: %+v", sh)
	}

	// List.
	shelves, _ := s.ListShelves(user.ID)
	if len(shelves) != 1 {
		t.Errorf("len = %d", len(shelves))
	}

	// Add work to shelf.
	w := &domain.Work{LibraryID: lib.ID, Title: "Book"}
	s.CreateWork(w)
	err = s.AddToShelf(sh.ID, w.ID)
	if err != nil {
		t.Fatal(err)
	}

	// List shelf works.
	works, _ := s.ListShelfWorks(sh.ID)
	if len(works) != 1 {
		t.Errorf("shelf works len = %d", len(works))
	}

	// Get shelf shows count.
	sh2, _ := s.GetShelf(sh.ID)
	if sh2.WorkCount != 1 {
		t.Errorf("WorkCount = %d", sh2.WorkCount)
	}

	// Remove from shelf.
	s.RemoveFromShelf(sh.ID, w.ID)
	works, _ = s.ListShelfWorks(sh.ID)
	if len(works) != 0 {
		t.Error("expected 0 works after remove")
	}

	// Delete shelf.
	err = s.DeleteShelf(sh.ID)
	if err != nil {
		t.Fatal(err)
	}
	shelves, _ = s.ListShelves(user.ID)
	if len(shelves) != 0 {
		t.Error("expected 0 shelves after delete")
	}
}

func TestTagsCRUD(t *testing.T) {
	s := newTestStore(t)
	lib, _ := s.CreateLibrary("Test")
	w := &domain.Work{LibraryID: lib.ID, Title: "Book"}
	s.CreateWork(w)

	// Add tags.
	s.AddTag(w.ID, "sci-fi")
	s.AddTag(w.ID, "classic")
	s.AddTag(w.ID, "sci-fi") // duplicate, should be ignored

	tags, _ := s.ListTags(w.ID)
	if len(tags) != 2 {
		t.Errorf("len = %d, want 2", len(tags))
	}

	// Remove.
	s.RemoveTag(w.ID, "classic")
	tags, _ = s.ListTags(w.ID)
	if len(tags) != 1 || tags[0] != "sci-fi" {
		t.Errorf("tags after remove: %v", tags)
	}

	// List all tags.
	allTags, _ := s.ListAllTags(lib.ID)
	if len(allTags) != 1 || allTags[0].Tag != "sci-fi" || allTags[0].Count != 1 {
		t.Errorf("all tags: %+v", allTags)
	}
}

func TestSeriesCRUD(t *testing.T) {
	s := newTestStore(t)
	lib, _ := s.CreateLibrary("Test")

	ser, err := s.CreateSeries("The Stormlight Archive", "An epic fantasy series")
	if err != nil {
		t.Fatal(err)
	}

	got, _ := s.GetSeries(ser.ID)
	if got.Name != "The Stormlight Archive" {
		t.Errorf("Name = %q", got.Name)
	}

	// Find by name.
	found, _ := s.FindSeriesByName("the stormlight archive")
	if found == nil || found.ID != ser.ID {
		t.Error("FindSeriesByName failed")
	}

	// Not found.
	notFound, _ := s.FindSeriesByName("nonexistent")
	if notFound != nil {
		t.Error("expected nil")
	}

	// List with counts (no works yet).
	series, _ := s.ListSeries(lib.ID)
	if len(series) != 0 {
		t.Errorf("expected 0, got %d (no works linked)", len(series))
	}

	// Link a work.
	w := &domain.Work{LibraryID: lib.ID, Title: "The Way of Kings", SeriesID: ser.ID, SeriesIndex: 1}
	s.CreateWork(w)
	series, _ = s.ListSeries(lib.ID)
	if len(series) != 1 || series[0].WorkCount != 1 {
		t.Errorf("series with counts: %+v", series)
	}
}
