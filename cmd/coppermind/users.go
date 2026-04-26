package main

import (
	"fmt"

	"github.com/jesseops/coppermind/internal/auth"
	"github.com/jesseops/coppermind/internal/domain"
	"github.com/jesseops/coppermind/internal/store"
	"github.com/spf13/cobra"
)

func newAddUserCmd() *cobra.Command {
	var (
		username    string
		password    string
		displayName string
		role        string
	)
	cmd := &cobra.Command{
		Use:   "add-user",
		Short: "Add a new user",
		RunE: func(cmd *cobra.Command, args []string) error {
			if username == "" || password == "" {
				return fmt.Errorf("--username and --password are required")
			}
			s, err := openStore()
			if err != nil {
				return err
			}
			defer s.Close()

			if displayName == "" {
				displayName = username
			}
			if role == "" {
				role = domain.RoleViewer
			}

			hash, err := auth.HashPassword(password)
			if err != nil {
				return err
			}

			user, err := s.CreateUser(username, displayName, hash, role)
			if err != nil {
				return fmt.Errorf("create user: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created user %q (id=%d, role=%s)\n", user.Username, user.ID, user.Role)
			return nil
		},
	}
	cmd.Flags().StringVar(&username, "username", "", "Username")
	cmd.Flags().StringVar(&password, "password", "", "Password")
	cmd.Flags().StringVar(&displayName, "display-name", "", "Display name")
	cmd.Flags().StringVar(&role, "role", "viewer", "Role (admin or viewer)")
	return cmd
}

func newResetPasswordCmd() *cobra.Command {
	var (
		username string
		password string
	)
	cmd := &cobra.Command{
		Use:   "reset-password",
		Short: "Reset a user's password",
		RunE: func(cmd *cobra.Command, args []string) error {
			if username == "" || password == "" {
				return fmt.Errorf("--username and --password are required")
			}
			s, err := openStore()
			if err != nil {
				return err
			}
			defer s.Close()

			user, err := s.GetUserByUsername(username)
			if err != nil {
				return fmt.Errorf("user %q not found", username)
			}

			hash, err := auth.HashPassword(password)
			if err != nil {
				return err
			}

			if err := s.UpdateUser(user.ID, store.UserUpdate{PasswordHash: &hash}); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Password reset for user %q\n", username)
			return nil
		},
	}
	cmd.Flags().StringVar(&username, "username", "", "Username")
	cmd.Flags().StringVar(&password, "password", "", "New password")
	return cmd
}

func newListUsersCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list-users",
		Short: "List all users",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStore()
			if err != nil {
				return err
			}
			defer s.Close()

			users, err := s.ListUsers()
			if err != nil {
				return err
			}
			if len(users) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No users found.")
				return nil
			}
			for _, u := range users {
				fmt.Fprintf(cmd.OutOrStdout(), "%d\t%s\t%s\t%s\n", u.ID, u.Username, u.DisplayName, u.Role)
			}
			return nil
		},
	}
}
