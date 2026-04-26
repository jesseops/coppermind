package store

import (
	"testing"

	"github.com/jesseops/coppermind/internal/domain"
)

func TestReadingStateUpsert(t *testing.T) {
	s := newTestStore(t)
	lib, _ := s.CreateLibrary("Test")
	user, _ := s.CreateUser("alice", "Alice", "hash", "viewer")
	w := &domain.Work{LibraryID: lib.ID, Title: "Book"}
	s.CreateWork(w)
	e := &domain.Edition{WorkID: w.ID, EditionType: domain.EditionTypeEbook}
	s.CreateEdition(e)

	// Initially nil.
	rs, err := s.GetReadingState(user.ID, e.ID)
	if err != nil {
		t.Fatal(err)
	}
	if rs != nil {
		t.Error("expected nil reading state")
	}

	// Save.
	err = s.SaveReadingState(&domain.ReadingState{
		UserID:       user.ID,
		EditionID:    e.ID,
		Status:       domain.ReadingStatusReading,
		Progress:     0.5,
		ChapterIndex: 3,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Get.
	rs, _ = s.GetReadingState(user.ID, e.ID)
	if rs == nil {
		t.Fatal("expected non-nil state")
	}
	if rs.Status != "reading" || rs.Progress != 0.5 || rs.ChapterIndex != 3 {
		t.Errorf("unexpected state: %+v", rs)
	}

	// Update (upsert).
	err = s.SaveReadingState(&domain.ReadingState{
		UserID:       user.ID,
		EditionID:    e.ID,
		Status:       domain.ReadingStatusFinished,
		Progress:     1.0,
		ChapterIndex: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	rs, _ = s.GetReadingState(user.ID, e.ID)
	if rs.Status != "finished" || rs.Progress != 1.0 {
		t.Errorf("status=%q progress=%f", rs.Status, rs.Progress)
	}

	// List.
	states, err := s.ListReadingStates(user.ID, "finished")
	if err != nil {
		t.Fatal(err)
	}
	if len(states) != 1 {
		t.Errorf("len = %d, want 1", len(states))
	}
}

func TestRatings(t *testing.T) {
	s := newTestStore(t)
	lib, _ := s.CreateLibrary("Test")
	user, _ := s.CreateUser("alice", "Alice", "hash", "viewer")
	w := &domain.Work{LibraryID: lib.ID, Title: "Book"}
	s.CreateWork(w)

	// No rating yet.
	r, _ := s.GetRating(user.ID, w.ID)
	if r != nil {
		t.Error("expected nil")
	}

	// Save.
	err := s.SaveRating(user.ID, w.ID, 5, "Great!")
	if err != nil {
		t.Fatal(err)
	}

	r, _ = s.GetRating(user.ID, w.ID)
	if r == nil || r.Rating != 5 || r.Review != "Great!" {
		t.Errorf("unexpected rating: %+v", r)
	}

	// Update.
	s.SaveRating(user.ID, w.ID, 4, "Pretty good")
	r, _ = s.GetRating(user.ID, w.ID)
	if r.Rating != 4 {
		t.Errorf("rating = %d, want 4", r.Rating)
	}

	// List.
	ratings, _ := s.ListRatings(w.ID)
	if len(ratings) != 1 {
		t.Errorf("len = %d", len(ratings))
	}
}
