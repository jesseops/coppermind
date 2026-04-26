package main

import (
	"fmt"

	"github.com/jesseops/coppermind/internal/store"
	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	var (
		kind      string
		libraryID int64
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List works, authors, or series",
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

			switch kind {
			case "works", "books":
				works, total, err := s.ListWorks(store.WorkFilter{
					LibraryID: libID,
					SortBy:    "title",
					SortOrder: "asc",
				})
				if err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%d works:\n", total)
				for _, w := range works {
					author := w.PrimaryAuthor()
					if author == "" {
						author = "(unknown)"
					}
					fmt.Fprintf(cmd.OutOrStdout(), "  %d\t%s\t%s\n", w.ID, w.Title, author)
				}
			case "authors":
				authors, err := s.ListAuthors(libID)
				if err != nil {
					return err
				}
				for _, a := range authors {
					fmt.Fprintf(cmd.OutOrStdout(), "  %d\t%s\t(%d works)\n", a.ID, a.Name, a.WorkCount)
				}
			case "series":
				series, err := s.ListSeries(libID)
				if err != nil {
					return err
				}
				for _, se := range series {
					fmt.Fprintf(cmd.OutOrStdout(), "  %d\t%s\t(%d works)\n", se.ID, se.Name, se.WorkCount)
				}
			default:
				return fmt.Errorf("unknown type: %s (use works, authors, or series)", kind)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&kind, "type", "works", "What to list: works, authors, series")
	cmd.Flags().Int64Var(&libraryID, "library-id", 0, "Library ID")
	return cmd
}

func newSearchCmd() *cobra.Command {
	var (
		query     string
		libraryID int64
	)
	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search for works",
		RunE: func(cmd *cobra.Command, args []string) error {
			if query == "" {
				return fmt.Errorf("--q is required")
			}
			s, err := openStore()
			if err != nil {
				return err
			}
			defer s.Close()

			libID, err := resolveLibraryID(s, libraryID)
			if err != nil {
				return err
			}

			works, total, err := s.ListWorks(store.WorkFilter{
				LibraryID: libID,
				Query:     query,
			})
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%d results:\n", total)
			for _, w := range works {
				author := w.PrimaryAuthor()
				if author == "" {
					author = "(unknown)"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  %d\t%s\t%s\n", w.ID, w.Title, author)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&query, "q", "", "Search query")
	cmd.Flags().Int64Var(&libraryID, "library-id", 0, "Library ID")
	return cmd
}
