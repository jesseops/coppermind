package store

import (
	"testing"

	"github.com/jesseops/coppermind/internal/domain"
)

func TestTrackCRUD(t *testing.T) {
	s := newTestStore(t)
	lib, _ := s.CreateLibrary("Test")
	w := &domain.Work{LibraryID: lib.ID, Title: "Audiobook"}
	s.CreateWork(w)
	e := &domain.Edition{WorkID: w.ID, EditionType: domain.EditionTypeAudiobook}
	s.CreateEdition(e)

	tk := &domain.Track{
		EditionID:  e.ID,
		TrackIndex: 1,
		Title:      "Chapter 1",
		FilePath:   "/audio/ch1.mp3",
	}
	if err := s.CreateTrack(tk); err != nil {
		t.Fatal(err)
	}

	tracks, err := s.ListTracks(e.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(tracks) != 1 || tracks[0].Title != "Chapter 1" {
		t.Errorf("unexpected tracks: %+v", tracks)
	}

	// Get.
	got, err := s.GetTrack(tk.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.FilePath != "/audio/ch1.mp3" {
		t.Errorf("FilePath = %q", got.FilePath)
	}
}

func TestReplaceTracksForEdition(t *testing.T) {
	s := newTestStore(t)
	lib, _ := s.CreateLibrary("Test")
	w := &domain.Work{LibraryID: lib.ID, Title: "Audio"}
	s.CreateWork(w)
	e := &domain.Edition{WorkID: w.ID, EditionType: domain.EditionTypeAudiobook}
	s.CreateEdition(e)

	// Add initial track.
	s.CreateTrack(&domain.Track{EditionID: e.ID, TrackIndex: 1, FilePath: "/a.mp3"})

	// Replace with new tracks.
	err := s.ReplaceTracksForEdition(e.ID, []domain.Track{
		{TrackIndex: 1, FilePath: "/b.mp3", Title: "Part 1"},
		{TrackIndex: 2, FilePath: "/c.mp3", Title: "Part 2"},
	})
	if err != nil {
		t.Fatal(err)
	}

	tracks, _ := s.ListTracks(e.ID)
	if len(tracks) != 2 {
		t.Errorf("len = %d, want 2", len(tracks))
	}
}
