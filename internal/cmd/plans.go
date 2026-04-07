package cmd

import (
	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"

	"github.com/baz-sh/clsm/internal/tui/planbrowse"
)

var plansCmd = &cobra.Command{
	Use:   "plans",
	Short: "Browse and clean up Claude Code plans",
	Long: `Browse all Claude Code plan files.

View plan contents, identify stale plans, and delete
ones that are no longer relevant.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		p := tea.NewProgram(planbrowse.New())
		_, err := p.Run()
		return err
	},
}
