package importer

import (
	"os"
	"testing"
)

func TestFixMobiAuthorTitleSwap(t *testing.T) {
	tests := []struct {
		name       string
		title      string
		author     string
		wantTitle  string
		wantAuthor string
	}{
		{
			name:       "normal - no swap needed",
			title:      "Dune",
			author:     "Frank Herbert",
			wantTitle:  "Dune",
			wantAuthor: "Frank Herbert",
		},
		{
			name:       "combined Author - Title with swapped author field",
			title:      "Robert A Heinlein - Bathroom Of Her Own",
			author:     "A Bathroom of Her Own",
			wantTitle:  "Bathroom Of Her Own",
			wantAuthor: "Robert A Heinlein",
		},
		{
			name:       "combined Author - Title with empty author field",
			title:      "Isaac Asimov - Foundation",
			author:     "",
			wantTitle:  "Foundation",
			wantAuthor: "Isaac Asimov",
		},
		{
			name:       "title with hyphen that is not Author - Title",
			title:      "Self-Reliance",
			author:     "Ralph Waldo Emerson",
			wantTitle:  "Self-Reliance",
			wantAuthor: "Ralph Waldo Emerson",
		},
		{
			name:       "simple swap - title looks like a name, author looks like a title",
			title:      "Robert A Heinlein",
			author:     "A Bathroom of Her Own",
			wantTitle:  "A Bathroom of Her Own",
			wantAuthor: "Robert A Heinlein",
		},
		{
			name:       "both empty",
			title:      "",
			author:     "",
			wantTitle:  "",
			wantAuthor: "",
		},
		{
			name:       "title starts with The - not a name",
			title:      "The Great Gatsby",
			author:     "F Scott Fitzgerald",
			wantTitle:  "The Great Gatsby",
			wantAuthor: "F Scott Fitzgerald",
		},
		{
			name:       "numeric title with title in author field",
			title:      "115",
			author:     "Sabotage at Sports city",
			wantTitle:  "Sabotage at Sports city",
			wantAuthor: "",
		},
		{
			name:       "short code title with title in author field",
			title:      "IV",
			author:     "The Broken Earth",
			wantTitle:  "The Broken Earth",
			wantAuthor: "",
		},
		{
			name:       "author has prepositions - looks like title",
			title:      "Murder on the Orient Express",
			author:     "Agatha Christie",
			wantTitle:  "Murder on the Orient Express",
			wantAuthor: "Agatha Christie",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTitle, gotAuthor := fixMobiAuthorTitleSwap(tt.title, tt.author)
			if gotTitle != tt.wantTitle || gotAuthor != tt.wantAuthor {
				t.Errorf("fixMobiAuthorTitleSwap(%q, %q) = (%q, %q), want (%q, %q)",
					tt.title, tt.author, gotTitle, gotAuthor, tt.wantTitle, tt.wantAuthor)
			}
		})
	}
}

func TestLooksLikePersonName(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"Robert A Heinlein", true},
		{"Frank Herbert", true},
		{"Isaac Asimov", true},
		{"F Scott Fitzgerald", true},
		{"Christie, Agatha", true},           // Last, First format
		{"A Bathroom of Her Own", false},      // starts with "A"
		{"The Great Gatsby", false},           // starts with "The"
		{"An Introduction to Go", false},      // starts with "An"
		{"Sabotage at Sports city", false},    // contains "at"
		{"Murder on the Orient Express", false}, // contains "on", "the"
		{"Dune", false},                       // single word
		{"", false},                           // empty
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := looksLikePersonName(tt.input); got != tt.want {
				t.Errorf("looksLikePersonName(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestLooksLikeSeriesNumber(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"115", true},
		{"3", true},
		{"IV", true},
		{"", false},
		{"Dune", false},
		{"The Great Gatsby", false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := looksLikeSeriesNumber(tt.input); got != tt.want {
				t.Errorf("looksLikeSeriesNumber(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsImageData(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{"JPEG", []byte{0xFF, 0xD8, 0xFF, 0xE0}, true},
		{"PNG", []byte{0x89, 0x50, 0x4E, 0x47}, true},
		{"GIF", []byte{0x47, 0x49, 0x46, 0x38}, true},
		{"HTML", []byte("<html>"), false},
		{"empty", []byte{}, false},
		{"short", []byte{0xFF}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isImageData(tt.data); got != tt.want {
				t.Errorf("isImageData() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestExtractMobiMetadataReal tests against an actual MOBI file if available.
// Skipped in CI (no test fixtures checked in).
func TestExtractMobiMetadataReal(t *testing.T) {
	path := "/home/jesse/ebooks/Robert A Heinlein Bibliography/RH67 - A Bathroom of Her Own.mobi"
	if _, err := os.Stat(path); err != nil {
		t.Skip("test MOBI file not available")
	}

	meta, err := ExtractMobiMetadata(path)
	if err != nil {
		t.Fatalf("ExtractMobiMetadata: %v", err)
	}

	// Title should be extracted and un-swapped.
	if meta.Title == "" {
		t.Error("Title is empty")
	}
	if meta.Title == "Robert A Heinlein" || meta.Title == "Robert A Heinlein - Bathroom Of Her Own" {
		t.Errorf("Title not properly cleaned: %q", meta.Title)
	}

	// Should have at least one author.
	if len(meta.Authors) == 0 {
		t.Error("No authors extracted")
	}

	// Author should not be the title.
	for _, a := range meta.Authors {
		if a == "A Bathroom of Her Own" || a == "Bathroom Of Her Own" {
			t.Errorf("Author is actually the title: %q", a)
		}
	}

	// Format should be mobi.
	if meta.Format != "mobi" {
		t.Errorf("Format = %q, want mobi", meta.Format)
	}

	t.Logf("Title:       %q", meta.Title)
	t.Logf("Authors:     %v", meta.Authors)
	t.Logf("Publisher:   %q", meta.Publisher)
	t.Logf("Description: %q", meta.Description)
	t.Logf("ISBN:        %q", meta.ISBN)
	t.Logf("Year:        %d", meta.PublishedYear)
	t.Logf("Language:    %q", meta.Language)
	t.Logf("Subjects:    %v", meta.Subjects)
	t.Logf("Cover:       %d bytes, ext=%s", len(meta.CoverData), meta.CoverExt)
}

// TestMobiEXTHConstants verifies the EXTH constants match the MobileRead wiki spec.
func TestMobiEXTHConstants(t *testing.T) {
	if exthAuthor != 100 {
		t.Errorf("exthAuthor = %d, want 100", exthAuthor)
	}
	if exthPublisher != 101 {
		t.Errorf("exthPublisher = %d, want 101", exthPublisher)
	}
	if exthDescription != 103 {
		t.Errorf("exthDescription = %d, want 103", exthDescription)
	}
	if exthISBN != 104 {
		t.Errorf("exthISBN = %d, want 104", exthISBN)
	}
	if exthSubject != 105 {
		t.Errorf("exthSubject = %d, want 105", exthSubject)
	}
	if exthPublishingDate != 106 {
		t.Errorf("exthPublishingDate = %d, want 106", exthPublishingDate)
	}
	if exthCoverOffset != 201 {
		t.Errorf("exthCoverOffset = %d, want 201", exthCoverOffset)
	}
	if exthUpdatedTitle != 503 {
		t.Errorf("exthUpdatedTitle = %d, want 503", exthUpdatedTitle)
	}
	if exthLanguage != 524 {
		t.Errorf("exthLanguage = %d, want 524", exthLanguage)
	}
}
