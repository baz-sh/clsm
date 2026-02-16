package cmd

import (
	"github.com/spf13/cobra"
)

// rootCmd is the base command for clsm.
var rootCmd = &cobra.Command{
	Use:   "clsm",
	Short: "Claude Session Manager",
	Long:  "A CLI/TUI tool for managing Claude Code sessions.",
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(deleteCmd)
}
