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

func TestListWorksEnrichment(t *testing.T) {
	s := newTestStore(t)
	lib, _ := s.CreateLibrary("Test")
	series, _ := s.CreateSeries("The Stormlight Archive", "")
	author, _ := s.CreateAuthor("Brandon Sanderson", "sanderson, brandon")

	w := &domain.Work{LibraryID: lib.ID, Title: "The Way of Kings", SeriesID: series.ID}
	if err := s.CreateWork(w); err != nil {
		t.Fatal(err)
	}
	if err := s.LinkWorkAuthor(w.ID, author.ID, domain.RoleAuthorOf); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateEdition(&domain.Edition{WorkID: w.ID, EditionType: domain.EditionTypeEbook, Format: domain.FormatEPUB}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateEdition(&domain.Edition{WorkID: w.ID, EditionType: domain.EditionTypeAudiobook, Format: domain.FormatM4B}); err != nil {
		t.Fatal(err)
	}

	works, total, err := s.ListWorks(WorkFilter{LibraryID: lib.ID})
	if err != nil {
		t.Fatalf("ListWorks: %v", err)
	}
	if total != 1 || len(works) != 1 {
		t.Fatalf("total=%d len=%d, want 1/1", total, len(works))
	}
	got := works[0]
	if got.SeriesName != "The Stormlight Archive" {
		t.Fatalf("SeriesName = %q", got.SeriesName)
	}
	if len(got.Authors) != 1 || got.Authors[0].AuthorName != "Brandon Sanderson" {
		t.Fatalf("Authors = %+v", got.Authors)
	}
	if !got.HasEbook || !got.HasAudiobook || got.EditionCount != 2 {
		t.Fatalf("edition flags/count wrong: ebook=%v audiobook=%v count=%d", got.HasEbook, got.HasAudiobook, got.EditionCount)
	}
}

func TestMergeWorks(t *testing.T) {
	s := newTestStore(t)
	lib, _ := s.CreateLibrary("Test")

	target := &domain.Work{LibraryID: lib.ID, Title: "Dune"}
	if err := s.CreateWork(target); err != nil {
		t.Fatal(err)
	}
	sourceDesc := "A desert planet."
	source := &domain.Work{LibraryID: lib.ID, Title: "Dune", Description: sourceDesc, CoverPath: "covers/dune.jpg"}
	if err := s.CreateWork(source); err != nil {
		t.Fatal(err)
	}

	author, _ := s.CreateAuthor("Frank Herbert", "herbert, frank")
	if err := s.LinkWorkAuthor(source.ID, author.ID, domain.RoleAuthorOf); err != nil {
		t.Fatal(err)
	}
	if err := s.AddTag(source.ID, "sci-fi"); err != nil {
		t.Fatal(err)
	}
	edition := &domain.Edition{WorkID: source.ID, EditionType: domain.EditionTypeEbook, Format: domain.FormatEPUB, FileHash: "hash"}
	if err := s.CreateEdition(edition); err != nil {
		t.Fatal(err)
	}

	if err := s.MergeWorks(target.ID, []int64{source.ID}); err != nil {
		t.Fatalf("MergeWorks: %v", err)
	}

	merged, err := s.GetWork(target.ID)
	if err != nil {
		t.Fatal(err)
	}
	if merged.Description != sourceDesc || merged.CoverPath != "covers/dune.jpg" {
		t.Fatalf("metadata not copied: %+v", merged)
	}
	if len(merged.Authors) != 1 || merged.Authors[0].AuthorName != "Frank Herbert" {
		t.Fatalf("authors not merged: %+v", merged.Authors)
	}
	tags, _ := s.ListTags(target.ID)
	if len(tags) != 1 || tags[0] != "sci-fi" {
		t.Fatalf("tags not merged: %+v", tags)
	}
	editions, _ := s.ListEditions(target.ID)
	if len(editions) != 1 || editions[0].ID != edition.ID {
		t.Fatalf("editions not moved: %+v", editions)
	}
	if _, err := s.GetWork(source.ID); err == nil {
		t.Fatal("source work still exists")
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
