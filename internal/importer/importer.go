package importer

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jesseops/coppermind/internal/domain"
	"github.com/jesseops/coppermind/internal/store"
)

// Importer orchestrates the full import pipeline.
type Importer struct {
	store   store.ImportStore
	dataDir string
}

// NewImporter creates a new Importer.
func NewImporter(s store.ImportStore, dataDir string) *Importer {
	return &Importer{store: s, dataDir: dataDir}
}

// ImportInput specifies what to import.
type ImportInput struct {
	SourcePath  string
	LibraryID   int64
	Title       string
	Authors     []string
	Series      string
	SeriesIndex *float64
}

// ImportResult describes the outcome of an import.
type ImportResult struct {
	Work       *domain.Work
	Edition    *domain.Edition
	IsNew      bool
	Skipped    bool
	SkipReason string
}

// Import imports a single file or directory.
func (imp *Importer) Import(input ImportInput) (*ImportResult, error) {
	srcPath := filepath.Clean(input.SourcePath)
	info, err := os.Stat(srcPath)
	if err != nil {
		return nil, fmt.Errorf("stat source: %w", err)
	}

	// 1. Detect format and extract metadata.
	meta, err := extractMetadata(srcPath, info)
	if err != nil {
		return nil, fmt.Errorf("extract metadata: %w", err)
	}

	// Apply overrides.
	if input.Title != "" {
		meta.Title = input.Title
	}
	if len(input.Authors) > 0 {
		meta.Authors = input.Authors
	}
	if input.Series != "" {
		meta.Series = input.Series
	}
	if input.SeriesIndex != nil {
		meta.SeriesIndex = *input.SeriesIndex
	}

	if meta.Title == "" {
		return nil, fmt.Errorf("unable to determine title from %s", srcPath)
	}

	// 2. Compute hash.
	fileHash := ""
	if !info.IsDir() {
		fileHash, _ = hashFile(srcPath)
	}

	// 3. Match.
	matcher := NewMatcher(imp.store)
	match, err := matcher.Match(meta, input.LibraryID, fileHash)
	if err != nil {
		return nil, fmt.Errorf("match: %w", err)
	}
	if match.IsDuplicate {
		return &ImportResult{
			Skipped:    true,
			SkipReason: fmt.Sprintf("duplicate of edition %d", match.DuplicateEditionID),
		}, nil
	}

	// 4. Copy file to library directory.
	editionType := InferEditionType(srcPath)
	if editionType == "" {
		editionType = domain.EditionTypeEbook
	}

	itemDir := filepath.Join(
		imp.dataDir,
		fmt.Sprintf("library-%d", input.LibraryID),
		editionType,
		fmt.Sprintf("%d-%s", match.Work.ID, safePathSegment(meta.Title)),
	)

	var destPath string
	if info.IsDir() {
		if err := copyDir(srcPath, itemDir); err != nil {
			return nil, fmt.Errorf("copy dir: %w", err)
		}
		destPath = itemDir
	} else {
		destPath = filepath.Join(itemDir, filepath.Base(srcPath))
		if err := copyFile(srcPath, destPath); err != nil {
			return nil, fmt.Errorf("copy file: %w", err)
		}
	}

	// 5. Write cover.
	coverPath := ""
	if len(meta.CoverData) > 0 && meta.CoverExt != "" {
		coverFile := filepath.Join(itemDir, "cover"+meta.CoverExt)
		if err := os.MkdirAll(itemDir, 0o755); err == nil {
			if err := os.WriteFile(coverFile, meta.CoverData, 0o644); err == nil {
				coverPath = coverFile
			}
		}
	}

	// Set work cover if not already set.
	if coverPath != "" && match.Work.CoverPath == "" {
		imp.store.UpdateWork(match.Work.ID, store.WorkUpdate{CoverPath: &coverPath})
		match.Work.CoverPath = coverPath
	}

	// 6. Create edition.
	edition := &domain.Edition{
		WorkID:        match.Work.ID,
		EditionType:   editionType,
		Format:        meta.Format,
		ISBN:          meta.ISBN,
		Publisher:     meta.Publisher,
		PublishedYear: meta.PublishedYear,
		FilePath:      destPath,
		FileHash:      fileHash,
		FileSize:      fileSize(info),
		CoverPath:     coverPath,
		Status:        domain.EditionStatusActive,
	}
	if err := imp.store.CreateEdition(edition); err != nil {
		return nil, fmt.Errorf("create edition: %w", err)
	}

	// 7. Build tracks for audiobooks.
	if editionType == domain.EditionTypeAudiobook {
		tracks, totalDuration, err := BuildAudiobookTracks(destPath)
		if err == nil && len(tracks) > 0 {
			imp.store.ReplaceTracksForEdition(edition.ID, tracks)
			if totalDuration > 0 {
				imp.store.UpdateEdition(edition.ID, store.EditionUpdate{DurationSeconds: &totalDuration})
			}
		}
	}

	return &ImportResult{
		Work:    match.Work,
		Edition: edition,
		IsNew:   match.IsNewWork,
	}, nil
}

// BulkImport imports all supported files from a directory.
func (imp *Importer) BulkImport(dirPath string, libraryID int64) ([]ImportResult, error) {
	var results []ImportResult

	err := filepath.WalkDir(dirPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip errors
		}
		if d.IsDir() {
			// Check if directory is an audiobook.
			if ContainsAudioFiles(path) {
				result, err := imp.Import(ImportInput{
					SourcePath: path,
					LibraryID:  libraryID,
				})
				if err != nil {
					results = append(results, ImportResult{Skipped: true, SkipReason: err.Error()})
				} else {
					results = append(results, *result)
				}
				return filepath.SkipDir
			}
			return nil
		}
		if !IsImportableExt(path) {
			return nil
		}
		result, err := imp.Import(ImportInput{
			SourcePath: path,
			LibraryID:  libraryID,
		})
		if err != nil {
			results = append(results, ImportResult{Skipped: true, SkipReason: err.Error()})
		} else {
			results = append(results, *result)
		}
		return nil
	})

	return results, err
}

// ── helpers ─────────────────────────────────────────────────────────

func extractMetadata(path string, info os.FileInfo) (*Extracted, error) {
	if info.IsDir() {
		return ExtractAudioMetadata(path)
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".epub":
		return ExtractEpubMetadata(path)
	case ".mobi":
		return ExtractMobiMetadata(path)
	case ".mp3", ".m4b":
		return ExtractAudioMetadata(path)
	case ".pdf":
		title := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		return &Extracted{Title: title, Format: "pdf"}, nil
	default:
		return &Extracted{}, nil
	}
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func fileSize(info os.FileInfo) int64 {
	if info.IsDir() {
		return 0
	}
	return info.Size()
}

func safePathSegment(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "item"
	}
	var b strings.Builder
	for _, r := range trimmed {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_':
			b.WriteRune(r)
		case r == ' ' || r == '.':
			b.WriteRune('_')
		}
	}
	s := strings.Trim(b.String(), "_")
	if s == "" {
		return "item"
	}
	return s
}

func copyFile(srcPath, destPath string) error {
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return err
	}
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()
	dest, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer dest.Close()
	if _, err := io.Copy(dest, src); err != nil {
		return err
	}
	return dest.Sync()
}

func copyDir(srcDir, destDir string) error {
	return filepath.WalkDir(srcDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return os.MkdirAll(destDir, 0o755)
		}
		targetPath := filepath.Join(destDir, rel)
		if entry.IsDir() {
			return os.MkdirAll(targetPath, 0o755)
		}
		return copyFile(path, targetPath)
	})
}
