package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newServeCmd() *cobra.Command {
	var (
		host string
		port int
	)
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the web server",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := loadConfig()
			if err != nil {
				return err
			}
			if host != "" {
				cfg.Host = host
			}
			if port > 0 {
				cfg.Port = port
			}

			// The actual web server will be implemented in task 18.
			// For now, just print the config.
			fmt.Fprintf(cmd.OutOrStdout(), "Starting server on %s (db: %s)\n", cfg.Addr(), cfg.DBPath)
			fmt.Fprintln(cmd.OutOrStdout(), "Web server not yet implemented — see task 18")
			return nil
		},
	}
	cmd.Flags().StringVar(&host, "host", "", "Bind host")
	cmd.Flags().IntVar(&port, "port", 0, "Bind port")
	return cmd
}
