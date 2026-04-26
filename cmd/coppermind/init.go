package main

import (
	"fmt"
	"os"

	"github.com/jesseops/coppermind/internal/auth"
	"github.com/jesseops/coppermind/internal/domain"
	"github.com/jesseops/coppermind/internal/store"
	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	var (
		name     string
		username string
		password string
	)
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a new library database",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := loadConfig()
			if err != nil {
				return err
			}

			// Ensure data directory exists.
			if err := os.MkdirAll(c.DataDir, 0o755); err != nil {
				return fmt.Errorf("create data dir: %w", err)
			}

			s, err := store.NewSQLiteStore(c.DBPath)
			if err != nil {
				return fmt.Errorf("create database: %w", err)
			}
			defer s.Close()

			// Create library.
			lib, err := s.CreateLibrary(name)
			if err != nil {
				return fmt.Errorf("create library: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created library %q (id=%d)\n", lib.Name, lib.ID)

			// Create admin user if credentials provided.
			if username != "" && password != "" {
				hash, err := auth.HashPassword(password)
				if err != nil {
					return fmt.Errorf("hash password: %w", err)
				}
				user, err := s.CreateUser(username, username, hash, domain.RoleAdmin)
				if err != nil {
					return fmt.Errorf("create admin user: %w", err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Created admin user %q (id=%d)\n", user.Username, user.ID)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Database initialized at %s\n", c.DBPath)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "My Library", "Library name")
	cmd.Flags().StringVar(&username, "username", "", "Admin username")
	cmd.Flags().StringVar(&password, "password", "", "Admin password")
	return cmd
}
