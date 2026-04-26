package main

import (
	"fmt"
	"os"
)

func main() {
	if err := Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// Execute is the entry point for the CLI. It will be implemented in root.go.
func Execute() error {
	return nil
}
