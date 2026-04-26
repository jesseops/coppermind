package importer

import (
	"testing"

	"github.com/jesseops/coppermind/internal/domain"
	"github.com/jesseops/coppermind/internal/store"
)

func TestMatcherNewWork(t *testing.T) {
	s, err := store.NewMemoryStore()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	lib, _ := s.CreateLibrary("Test")

	m := NewMatcher(s)
	meta := &Extracted{
		Title:   "Dune",
		Authors: []string{"Frank Herbert"},
		Series:  "Dune Chronicles",
		SeriesIndex: 1,
	}

	result, err := m.Match(meta, lib.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsNewWork {
		t.Error("expected IsNewWork=true")
	}
	if result.Work == nil {
		t.Fatal("expected non-nil Work")
	}
	if result.Work.Title != "Dune" {
		t.Errorf("Title = %q", result.Work.Title)
	}
	if len(result.Authors) != 1 || result.Authors[0].Name != "Frank Herbert" {
		t.Errorf("Authors = %+v", result.Authors)
	}
	if result.Series == nil || result.Series.Name != "Dune Chronicles" {
		t.Errorf("Series = %+v", result.Series)
	}
}

func TestMatcherExistingWork(t *testing.T) {
	s, err := store.NewMemoryStore()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	lib, _ := s.CreateLibrary("Test")

	m := NewMatcher(s)

	// First import creates work.
	meta := &Extracted{Title: "Dune", Authors: []string{"Frank Herbert"}}
	result1, _ := m.Match(meta, lib.ID, "")

	// Second import should find existing.
	result2, _ := m.Match(meta, lib.ID, "")
	if result2.IsNewWork {
		t.Error("expected IsNewWork=false for second import")
	}
	if result2.Work.ID != result1.Work.ID {
		t.Errorf("work IDs differ: %d vs %d", result2.Work.ID, result1.Work.ID)
	}
}

func TestMatcherDuplicateHash(t *testing.T) {
	s, err := store.NewMemoryStore()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	lib, _ := s.CreateLibrary("Test")

	m := NewMatcher(s)
	meta := &Extracted{Title: "Dune", Authors: []string{"Frank Herbert"}}

	// First match and create edition with hash.
	result1, _ := m.Match(meta, lib.ID, "abc123hash")
	s.CreateEdition(&domain.Edition{
		WorkID:      result1.Work.ID,
		EditionType: "ebook",
		FileHash:    "abc123hash",
	})

	// Second match with same hash should detect duplicate.
	result2, err := m.Match(meta, lib.ID, "abc123hash")
	if err != nil {
		t.Fatal(err)
	}
	if !result2.IsDuplicate {
		t.Error("expected IsDuplicate=true")
	}
}

func TestMatcherAuthorDedup(t *testing.T) {
	s, err := store.NewMemoryStore()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	lib, _ := s.CreateLibrary("Test")

	m := NewMatcher(s)

	// Two imports with same author.
	m.Match(&Extracted{Title: "Book A", Authors: []string{"Brandon Sanderson"}}, lib.ID, "")
	m.Match(&Extracted{Title: "Book B", Authors: []string{"Brandon Sanderson"}}, lib.ID, "")

	// Should have only one author.
	authors, _ := s.ListAuthors(lib.ID)
	if len(authors) != 1 {
		t.Errorf("expected 1 author, got %d", len(authors))
	}
}
