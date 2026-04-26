package main

import (
	"fmt"

	"github.com/jesseops/coppermind/internal/importer"
	"github.com/spf13/cobra"
)

func newImportCmd() *cobra.Command {
	var (
		title     string
		author    string
		series    string
		libraryID int64
	)
	cmd := &cobra.Command{
		Use:   "import <path>",
		Short: "Import a file or directory into the library",
		Args:  cobra.ExactArgs(1),
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

			imp := importer.NewImporter(s, cfg.DataDir)
			input := importer.ImportInput{
				SourcePath: args[0],
				LibraryID:  libID,
				Title:      title,
				Series:     series,
			}
			if author != "" {
				input.Authors = []string{author}
			}

			result, err := imp.Import(input)
			if err != nil {
				return fmt.Errorf("import failed: %w", err)
			}
			if result.Skipped {
				fmt.Fprintf(cmd.OutOrStdout(), "Skipped: %s\n", result.SkipReason)
				return nil
			}
			action := "Updated"
			if result.IsNew {
				action = "Created"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s work %q (id=%d), edition id=%d\n",
				action, result.Work.Title, result.Work.ID, result.Edition.ID)
			return nil
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "Override title")
	cmd.Flags().StringVar(&author, "author", "", "Override author")
	cmd.Flags().StringVar(&series, "series", "", "Override series")
	cmd.Flags().Int64Var(&libraryID, "library-id", 0, "Library ID (defaults to first)")
	return cmd
}

func newBulkImportCmd() *cobra.Command {
	var libraryID int64
	cmd := &cobra.Command{
		Use:   "bulk-import <directory>",
		Short: "Import all supported files from a directory",
		Args:  cobra.ExactArgs(1),
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

			imp := importer.NewImporter(s, cfg.DataDir)
			results, err := imp.BulkImport(args[0], libID)
			if err != nil {
				return fmt.Errorf("bulk import failed: %w", err)
			}

			imported, skipped := 0, 0
			for _, r := range results {
				if r.Skipped {
					skipped++
					fmt.Fprintf(cmd.ErrOrStderr(), "  skip: %s\n", r.SkipReason)
				} else {
					imported++
				}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Bulk import complete: %d imported, %d skipped\n", imported, skipped)
			return nil
		},
	}
	cmd.Flags().Int64Var(&libraryID, "library-id", 0, "Library ID (defaults to first)")
	return cmd
}
