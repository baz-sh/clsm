package cmd

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/baz-sh/clsm/internal/tui/browse"
	tuidelete "github.com/baz-sh/clsm/internal/tui/delete"
	"github.com/baz-sh/clsm/internal/tui/home"
)

// rootCmd is the base command for clsm.
var rootCmd = &cobra.Command{
	Use:   "clsm",
	Short: "Claude Session Manager",
	Long:  "A CLI/TUI tool for managing Claude Code sessions.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runHome()
	},
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(deleteCmd)
	rootCmd.AddCommand(browseCmd)
}

func runHome() error {
	m := home.New()
	p := tea.NewProgram(m)
	result, err := p.Run()
	if err != nil {
		return err
	}

	choice := result.(home.Model).Result
	switch choice {
	case home.ChoiceBrowse:
		return runAltScreen(browse.New())
	case home.ChoiceDelete:
		return runAltScreen(tuidelete.New())
	case home.ChoiceNone:
		return nil
	default:
		return fmt.Errorf("unknown choice: %s", choice)
	}
}

func runAltScreen(model tea.Model) error {
	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
