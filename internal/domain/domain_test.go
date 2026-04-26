package domain

import "testing"

func TestGenerateSortTitle(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"The Way of Kings", "way of kings, the"},
		{"A Game of Thrones", "game of thrones, a"},
		{"An Ember in the Ashes", "ember in the ashes, an"},
		{"Dune", "dune"},
		{"", ""},
		{"  The Hobbit  ", "hobbit, the"},
	}
	for _, tt := range tests {
		got := GenerateSortTitle(tt.input)
		if got != tt.want {
			t.Errorf("GenerateSortTitle(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestGenerateSortName(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"Brandon Sanderson", "sanderson, brandon"},
		{"J.R.R. Tolkien", "tolkien, j.r.r."},
		{"Tolkien, J.R.R.", "tolkien, j.r.r."},
		{"Plato", "plato"},
		{"", ""},
		{"  Frank  Herbert  ", "herbert, frank"},
	}
	for _, tt := range tests {
		got := GenerateSortName(tt.input)
		if got != tt.want {
			t.Errorf("GenerateSortName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestUserIsAdmin(t *testing.T) {
	admin := &User{Role: RoleAdmin}
	viewer := &User{Role: RoleViewer}

	if !admin.IsAdmin() {
		t.Error("expected admin.IsAdmin() to be true")
	}
	if admin.IsViewer() {
		t.Error("expected admin.IsViewer() to be false")
	}
	if viewer.IsAdmin() {
		t.Error("expected viewer.IsAdmin() to be false")
	}
	if !viewer.IsViewer() {
		t.Error("expected viewer.IsViewer() to be true")
	}
}

func TestEditionHelpers(t *testing.T) {
	ebook := &Edition{EditionType: EditionTypeEbook, Status: EditionStatusActive, CoverPath: "/covers/1.jpg"}
	audio := &Edition{EditionType: EditionTypeAudiobook, Status: EditionStatusDeleted}

	if !ebook.IsEbook() {
		t.Error("expected IsEbook() true")
	}
	if ebook.IsAudiobook() {
		t.Error("expected IsAudiobook() false for ebook")
	}
	if !ebook.IsActive() {
		t.Error("expected IsActive() true")
	}
	if !ebook.HasCover() {
		t.Error("expected HasCover() true")
	}
	if !audio.IsAudiobook() {
		t.Error("expected IsAudiobook() true")
	}
	if audio.IsActive() {
		t.Error("expected IsActive() false for deleted")
	}
}

func TestWorkPrimaryAuthor(t *testing.T) {
	w := &Work{
		Authors: []WorkAuthor{
			{AuthorID: 1, Role: RoleNarratorOf, AuthorName: "Narrator"},
			{AuthorID: 2, Role: RoleAuthorOf, AuthorName: "Author"},
		},
	}
	if got := w.PrimaryAuthor(); got != "Author" {
		t.Errorf("PrimaryAuthor() = %q, want %q", got, "Author")
	}

	w2 := &Work{}
	if got := w2.PrimaryAuthor(); got != "" {
		t.Errorf("PrimaryAuthor() on empty = %q, want empty", got)
	}
}
