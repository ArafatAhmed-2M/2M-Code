// Package cli implements the Cobra command structure for 2M Code.
//
// Root command provides version info, help, and global flags.
// All subcommands are registered in their respective files.
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

const version = "1.0.0"

var (
	// verbose enables verbose logging output
	verbose bool
)

// rootCmd is the top-level Cobra command for 2M Code.
var rootCmd = &cobra.Command{
	Use:   "2m",
	Short: "2M Code — The AI coding platform that thinks in teams",
	Long: `2M Code (Multi-Mind) is a terminal-native AI coding platform where
multiple AI agents — each with a distinct role, model, and provider — 
collaborate on your codebase like a real engineering team.

Instead of one AI assistant, you deploy a configurable team of agents
that plan, implement, and review code together.

Quick start:
  2m new-team              Create a new team interactively
  2m run <team> "<task>"   Run a one-shot task with a team
  2m chat <team>           Start an interactive REPL with a team

Learn more: https://github.com/ArafatAhmed-2M/2M-Code`,
	Version: version,
}

// Execute runs the root command. Called from main.go.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")

	// Set version template
	rootCmd.SetVersionTemplate(fmt.Sprintf("2M Code v%s\n", version))
}
