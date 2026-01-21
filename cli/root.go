// Package cli provides the juango CLI commands.
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "juango",
	Short: "juango - scaffolding CLI for Go + Vite/React projects",
	Long: `juango is a CLI tool for creating and managing full-stack web applications
using Go for the backend and Vite/React for the frontend.

It provides:
  - Project scaffolding with 'juango init'
  - Development server with 'juango dev'
  - Reusable Go libraries for common patterns (auth, admin, database, etc.)
  - React components and utilities via @juango/ui`,
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(devCmd)
	rootCmd.AddCommand(versionCmd)
}
