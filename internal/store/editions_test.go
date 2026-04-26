package store

import (
	"testing"

	"github.com/jesseops/coppermind/internal/domain"
)

func TestEditionCRUD(t *testing.T) {
	s := newTestStore(t)
	lib, _ := s.CreateLibrary("Test")
	w := &domain.Work{LibraryID: lib.ID, Title: "Dune"}
	s.CreateWork(w)

	e := &domain.Edition{
		WorkID:      w.ID,
		EditionType: domain.EditionTypeEbook,
		Format:      "epub",
		FileHash:    "abc123",
	}
	if err := s.CreateEdition(e); err != nil {
		t.Fatalf("CreateEdition: %v", err)
	}
	if e.ID == 0 {
		t.Error("expected non-zero ID")
	}

	// Get.
	got, err := s.GetEdition(e.ID)
	if err != nil {
		t.Fatalf("GetEdition: %v", err)
	}
	if got.Format != "epub" || got.Status != "active" {
		t.Errorf("unexpected edition: %+v", got)
	}

	// List.
	editions, err := s.ListEditions(w.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(editions) != 1 {
		t.Errorf("len = %d, want 1", len(editions))
	}

	// Find by hash.
	found, err := s.FindEditionByHash("abc123")
	if err != nil {
		t.Fatal(err)
	}
	if found == nil || found.ID != e.ID {
		t.Error("expected to find by hash")
	}

	// Update.
	newISBN := "978-0441172719"
	err = s.UpdateEdition(e.ID, EditionUpdate{ISBN: &newISBN})
	if err != nil {
		t.Fatal(err)
	}
	got2, _ := s.GetEdition(e.ID)
	if got2.ISBN != "978-0441172719" {
		t.Errorf("ISBN = %q after update", got2.ISBN)
	}

	// Delete.
	err = s.DeleteEdition(e.ID)
	if err != nil {
		t.Fatal(err)
	}
	editions, _ = s.ListEditions(w.ID)
	if len(editions) != 0 {
		t.Error("expected 0 editions after delete")
	}
}

func TestEditionTypeBadges(t *testing.T) {
	s := newTestStore(t)
	lib, _ := s.CreateLibrary("Test")
	w := &domain.Work{LibraryID: lib.ID, Title: "Dune"}
	s.CreateWork(w)

	s.CreateEdition(&domain.Edition{WorkID: w.ID, EditionType: domain.EditionTypeEbook, Format: "epub"})
	s.CreateEdition(&domain.Edition{WorkID: w.ID, EditionType: domain.EditionTypeAudiobook, Format: "m4b"})

	got, _ := s.GetWork(w.ID)
	if !got.HasEbook || !got.HasAudiobook {
		t.Errorf("HasEbook=%v HasAudiobook=%v", got.HasEbook, got.HasAudiobook)
	}
	if got.EditionCount != 2 {
		t.Errorf("EditionCount = %d, want 2", got.EditionCount)
	}
}
