package cmd

import (
	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"

	"github.com/baz-sh/clsm/internal/tui/memorybrowse"
)

var memoriesCmd = &cobra.Command{
	Use:   "memories",
	Short: "Browse and manage Claude Code memories",
	Long: `Browse all Claude Code memory files across projects.

View, filter, and delete memory files that Claude uses for
persistent context across conversations.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		p := tea.NewProgram(memorybrowse.New(memorybrowse.ModeProjects))
		_, err := p.Run()
		return err
	},
}
