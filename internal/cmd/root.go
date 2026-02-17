package cmd

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/baz-sh/clsm/internal/tui/browse"
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
	for {
		m := home.New()
		p := tea.NewProgram(m, tea.WithAltScreen())
		result, err := p.Run()
		if err != nil {
			return err
		}

		choice := result.(home.Model).Result
		switch choice {
		case home.ChoiceProjects:
			if !runAndCheckBack(browse.New(browse.ModeProjects)) {
				return nil
			}
		case home.ChoiceSessions:
			if !runAndCheckBack(browse.New(browse.ModeSessions)) {
				return nil
			}
		case home.ChoiceSearch:
			if !runAndCheckBack(browse.New(browse.ModeSearch)) {
				return nil
			}
		case home.ChoiceNone:
			return nil
		default:
			return fmt.Errorf("unknown choice: %s", choice)
		}
	}
}

// backToHomer is implemented by TUI models that support returning to the home menu.
type backToHomer interface {
	tea.Model
	WantsBackToHome() bool
}

// runAndCheckBack runs a TUI in alt-screen and returns true if the user
// wants to go back to the home menu, false if they want to quit entirely.
func runAndCheckBack(model backToHomer) bool {
	p := tea.NewProgram(model, tea.WithAltScreen())
	result, err := p.Run()
	if err != nil {
		return false
	}
	if m, ok := result.(backToHomer); ok {
		return m.WantsBackToHome()
	}
	return false
}
