package importer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jesseops/coppermind/internal/store"
)

func TestImportEpub(t *testing.T) {
	s, err := store.NewMemoryStore()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	lib, _ := s.CreateLibrary("Test")

	dataDir := t.TempDir()
	srcDir := t.TempDir()
	epubPath := createTestEpub(t, srcDir, "Test Book", "Test Author", "", 0)

	imp := NewImporter(s, dataDir)
	result, err := imp.Import(ImportInput{
		SourcePath: epubPath,
		LibraryID:  lib.ID,
	})
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if result.Skipped {
		t.Fatalf("unexpected skip: %s", result.SkipReason)
	}
	if !result.IsNew {
		t.Error("expected IsNew=true")
	}
	if result.Work.Title != "Test Book" {
		t.Errorf("Title = %q", result.Work.Title)
	}
	if result.Edition.Format != "epub" {
		t.Errorf("Format = %q", result.Edition.Format)
	}
	if result.Edition.FilePath == "" {
		t.Error("FilePath should be set")
	}
}

func TestImportDuplicate(t *testing.T) {
	s, err := store.NewMemoryStore()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	lib, _ := s.CreateLibrary("Test")

	dataDir := t.TempDir()
	srcDir := t.TempDir()
	epubPath := createTestEpub(t, srcDir, "Test Book", "Test Author", "", 0)

	imp := NewImporter(s, dataDir)

	// First import.
	_, err = imp.Import(ImportInput{SourcePath: epubPath, LibraryID: lib.ID})
	if err != nil {
		t.Fatal(err)
	}

	// Second import of same file should be duplicate.
	result, err := imp.Import(ImportInput{SourcePath: epubPath, LibraryID: lib.ID})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Skipped {
		t.Error("expected duplicate to be skipped")
	}
}

func TestBulkImport(t *testing.T) {
	s, err := store.NewMemoryStore()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	lib, _ := s.CreateLibrary("Test")

	dataDir := t.TempDir()
	srcDir := t.TempDir()

	// Create two EPUBs.
	createTestEpub(t, srcDir, "Book A", "Author A", "", 0)
	// Need a different filename for the second.
	subDir := filepath.Join(srcDir, "sub")
	os.MkdirAll(subDir, 0o755)
	createTestEpub(t, subDir, "Book B", "Author B", "", 0)

	imp := NewImporter(s, dataDir)
	results, err := imp.BulkImport(srcDir, lib.ID)
	if err != nil {
		t.Fatal(err)
	}

	imported := 0
	for _, r := range results {
		if !r.Skipped {
			imported++
		}
	}
	// At least 1 should import (could be 2 depending on walk order and dedup).
	if imported < 1 {
		t.Errorf("expected at least 1 import, got %d", imported)
	}
}
