package cmd

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/baz-sh/clsm/internal/tui/browse"
)

var browseCmd = &cobra.Command{
	Use:   "browse",
	Short: "Browse Claude Code projects and sessions",
	Long: `Browse all Claude Code projects and their sessions interactively.

Navigate the project list, drill into a project to see its sessions,
and filter results with /.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		p := tea.NewProgram(browse.New(), tea.WithAltScreen())
		_, err := p.Run()
		return err
	},
}
