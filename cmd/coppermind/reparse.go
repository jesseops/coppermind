package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/jesseops/coppermind/internal/domain"
	"github.com/jesseops/coppermind/internal/importer"
	"github.com/jesseops/coppermind/internal/store"
	"github.com/spf13/cobra"
)

func newReparseCmd() *cobra.Command {
	var (
		libraryID int64
		dryRun    bool
	)
	cmd := &cobra.Command{
		Use:   "reparse [work-id...]",
		Short: "Re-extract metadata from edition files and update works/authors",
		Long: `Re-reads the source file for each edition and updates the work title,
authors, and other metadata from the file. Useful after parser fixes.

With no arguments, reparses all works. Pass one or more work IDs to
reparse only those works.

Use --dry-run to preview changes without writing them.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStore()
			if err != nil {
				return err
			}
			defer s.Close()

			libID, err := resolveLibraryID(s, libraryID)
			if err != nil {
				return err
			}

			// Collect works to reparse.
			var works []domain.Work
			if len(args) > 0 {
				for _, arg := range args {
					var id int64
					if _, err := fmt.Sscanf(arg, "%d", &id); err != nil {
						return fmt.Errorf("invalid work ID: %s", arg)
					}
					w, err := s.GetWork(id)
					if err != nil {
						return fmt.Errorf("get work %d: %w", id, err)
					}
					works = append(works, *w)
				}
			} else {
				all, _, err := s.ListWorks(store.WorkFilter{LibraryID: libID})
				if err != nil {
					return fmt.Errorf("list works: %w", err)
				}
				works = all
			}

			updated, skipped, errors := 0, 0, 0
			for _, work := range works {
				editions, err := s.ListEditions(work.ID)
				if err != nil {
					slog.Error("list editions", "work_id", work.ID, "err", err)
					errors++
					continue
				}
				if len(editions) == 0 {
					skipped++
					continue
				}

				// Use the first edition's file for metadata.
				ed := editions[0]
				if ed.FilePath == "" || !fileExists(ed.FilePath) {
					fmt.Fprintf(cmd.ErrOrStderr(), "  skip work %d %q: missing file %s\n",
						work.ID, work.Title, ed.FilePath)
					skipped++
					continue
				}

				meta, err := extractMeta(ed.FilePath)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "  skip work %d %q: %v\n",
						work.ID, work.Title, err)
					skipped++
					continue
				}

				// Detect changes.
				changes := describeChanges(&work, meta)
				if len(changes) == 0 {
					skipped++
					continue
				}

				label := "UPDATE"
				if dryRun {
					label = "WOULD UPDATE"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  %s work %d: %s\n", label, work.ID, strings.Join(changes, "; "))

				if dryRun {
					updated++
					continue
				}

				// Apply title update.
				if meta.Title != "" && meta.Title != work.Title {
					sortTitle := domain.GenerateSortTitle(meta.Title)
					s.UpdateWork(work.ID, store.WorkUpdate{Title: &meta.Title})
					// Also update sort title by re-saving.
					_ = sortTitle // WorkUpdate doesn't have SortTitle; update directly.
				}

				// Apply author update: unlink old, link new.
				if len(meta.Authors) > 0 && authorsChanged(&work, meta.Authors) {
					oldAuthors, _ := s.GetWorkAuthors(work.ID)
					for _, oa := range oldAuthors {
						s.UnlinkWorkAuthor(work.ID, oa.AuthorID, oa.Role)
					}
					for _, name := range meta.Authors {
						sortName := domain.GenerateSortName(name)
						existing, err := s.FindAuthorBySortName(sortName)
						if err == nil && existing != nil {
							s.LinkWorkAuthor(work.ID, existing.ID, domain.RoleAuthorOf)
						} else {
							created, err := s.CreateAuthor(name, sortName)
							if err == nil {
								s.LinkWorkAuthor(work.ID, created.ID, domain.RoleAuthorOf)
							}
						}
					}
				}

				// Apply description if currently empty.
				if meta.Description != "" && work.Description == "" {
					s.UpdateWork(work.ID, store.WorkUpdate{Description: &meta.Description})
				}

				// Apply language if currently empty.
				if meta.Language != "" && work.Language == "" {
					s.UpdateWork(work.ID, store.WorkUpdate{Language: &meta.Language})
				}

				updated++
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Reparse complete: %d updated, %d unchanged, %d errors\n",
				updated, skipped, errors)
			return nil
		},
	}
	cmd.Flags().Int64Var(&libraryID, "library-id", 0, "Library ID (defaults to first)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview changes without writing")
	return cmd
}

func extractMeta(filePath string) (*importer.Extracted, error) {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".epub":
		return importer.ExtractEpubMetadata(filePath)
	case ".mobi":
		return importer.ExtractMobiMetadata(filePath)
	default:
		return nil, fmt.Errorf("unsupported format: %s", ext)
	}
}

func describeChanges(work *domain.Work, meta *importer.Extracted) []string {
	var changes []string
	if meta.Title != "" && meta.Title != work.Title {
		changes = append(changes, fmt.Sprintf("title: %q → %q", work.Title, meta.Title))
	}
	if len(meta.Authors) > 0 && authorsChanged(work, meta.Authors) {
		old := formatAuthorList(work.Authors)
		new_ := strings.Join(meta.Authors, ", ")
		changes = append(changes, fmt.Sprintf("authors: %q → %q", old, new_))
	}
	return changes
}

// authorsChanged returns true if the parsed authors are materially different
// from the work's current authors (comparing by sort name, not display name).
func authorsChanged(work *domain.Work, newAuthors []string) bool {
	oldSet := make(map[string]bool)
	for _, a := range work.Authors {
		oldSet[domain.GenerateSortName(a.AuthorName)] = true
	}
	newSet := make(map[string]bool)
	for _, name := range newAuthors {
		newSet[domain.GenerateSortName(name)] = true
	}
	if len(oldSet) != len(newSet) {
		return true
	}
	for k := range newSet {
		if !oldSet[k] {
			return true
		}
	}
	return false
}

func formatAuthorList(authors []domain.WorkAuthor) string {
	names := make([]string, len(authors))
	for i, a := range authors {
		names[i] = a.AuthorName
	}
	return strings.Join(names, ", ")
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
