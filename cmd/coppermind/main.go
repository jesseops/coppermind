package main

import (
	"fmt"
	"os"

	"github.com/jesseops/coppermind/internal/config"
	"github.com/jesseops/coppermind/internal/store"
	"github.com/spf13/cobra"
)

var (
	cfgPath string
	dbPath  string
	cfg     *config.Config
)

func Execute() error {
	rootCmd := newRootCmd()
	return rootCmd.Execute()
}

func newRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "coppermind",
		Short: "Your personal Plex, but for books",
		Long:  "Coppermind is a self-hosted digital library for ebooks and audiobooks.",
		SilenceUsage: true,
	}

	rootCmd.PersistentFlags().StringVar(&cfgPath, "config", "", "Path to config file")
	rootCmd.PersistentFlags().StringVar(&dbPath, "db", "", "Path to SQLite database")

	rootCmd.AddCommand(
		newInitCmd(),
		newServeCmd(),
		newImportCmd(),
		newBulkImportCmd(),
		newReparseCmd(),
		newAddUserCmd(),
		newResetPasswordCmd(),
		newListUsersCmd(),
		newListCmd(),
		newSearchCmd(),
	)

	return rootCmd
}

// loadConfig loads configuration, applying flag overrides.
func loadConfig() (*config.Config, error) {
	c, err := config.Load(cfgPath)
	if err != nil {
		return nil, err
	}
	if dbPath != "" {
		c.DBPath = dbPath
	}
	cfg = c
	return c, nil
}

// openStore opens the database store using the resolved config.
func openStore() (*store.SQLiteStore, error) {
	c, err := loadConfig()
	if err != nil {
		return nil, err
	}
	s, err := store.NewSQLiteStore(c.DBPath)
	if err != nil {
		return nil, fmt.Errorf("open database %s: %w", c.DBPath, err)
	}
	return s, nil
}

// resolveLibraryID returns the library ID, defaulting to the first library.
func resolveLibraryID(s *store.SQLiteStore, flagID int64) (int64, error) {
	if flagID > 0 {
		return flagID, nil
	}
	libs, err := s.ListLibraries()
	if err != nil {
		return 0, err
	}
	if len(libs) == 0 {
		return 0, fmt.Errorf("no libraries found; run 'coppermind init' first")
	}
	return libs[0].ID, nil
}

func main() {
	if err := Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
