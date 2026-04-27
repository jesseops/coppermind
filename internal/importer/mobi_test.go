package importer

import "testing"

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
		{"A Bathroom of Her Own", false},    // starts with "A"
		{"The Great Gatsby", false},          // starts with "The"
		{"An Introduction to Go", false},     // starts with "An"
		{"Dune", false},                      // single word
		{"", false},                          // empty
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := looksLikePersonName(tt.input); got != tt.want {
				t.Errorf("looksLikePersonName(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
