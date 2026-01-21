package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	// Version is set at build time
	Version = "0.1.0"
	// Commit is set at build time
	Commit = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("juango version %s (commit: %s)\n", Version, Commit)
	},
}
