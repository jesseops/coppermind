package main

import (
	"fmt"

	"github.com/jesseops/coppermind/internal/store"
	"github.com/jesseops/coppermind/internal/web"
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
			c, err := loadConfig()
			if err != nil {
				return err
			}
			if host != "" {
				c.Host = host
			}
			if port > 0 {
				c.Port = port
			}
			cfg = c

			s, err := store.NewSQLiteStore(c.DBPath)
			if err != nil {
				return fmt.Errorf("open database: %w", err)
			}
			defer s.Close()

			srv, err := web.NewServer(s, c)
			if err != nil {
				return fmt.Errorf("create server: %w", err)
			}

			return srv.Run()
		},
	}
	cmd.Flags().StringVar(&host, "host", "", "Bind host")
	cmd.Flags().IntVar(&port, "port", 0, "Bind port")
	return cmd
}
