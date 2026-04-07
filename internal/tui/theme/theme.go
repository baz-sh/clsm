package theme

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// Theme holds resolved styles for the current terminal background.
type Theme struct {
	IsDark     bool
	Title      lipgloss.Style
	Cursor     lipgloss.Style
	Selected   lipgloss.Style
	Dim        lipgloss.Style
	Count      lipgloss.Style
	Help       lipgloss.Style
	Breadcrumb lipgloss.Style
	Success    lipgloss.Style
	Error      lipgloss.Style
	Bold       lipgloss.Style
	Check      lipgloss.Style
	Uncheck    lipgloss.Style
}

// New creates a Theme resolved for the given dark-mode flag.
func New(isDark bool) Theme {
	ld := lipgloss.LightDark(isDark)
	resolve := func(light, dark string) color.Color {
		return ld(lipgloss.Color(light), lipgloss.Color(dark))
	}

	t := Theme{IsDark: isDark}
	t.Title = lipgloss.NewStyle().Bold(true).Foreground(resolve("57", "99"))
	t.Cursor = lipgloss.NewStyle().Foreground(resolve("30", "86"))
	t.Selected = lipgloss.NewStyle().Foreground(resolve("128", "170"))
	t.Dim = lipgloss.NewStyle().Foreground(resolve("247", "241"))
	t.Count = lipgloss.NewStyle().Foreground(resolve("128", "170"))
	t.Help = lipgloss.NewStyle().Foreground(resolve("247", "241"))
	t.Breadcrumb = lipgloss.NewStyle().Foreground(resolve("25", "63"))
	t.Success = lipgloss.NewStyle().Foreground(resolve("28", "42"))
	t.Error = lipgloss.NewStyle().Foreground(resolve("124", "196"))
	t.Bold = lipgloss.NewStyle().Bold(true)
	t.Check = lipgloss.NewStyle().Foreground(resolve("128", "170")).SetString("[x]")
	t.Uncheck = lipgloss.NewStyle().Foreground(resolve("247", "241")).SetString("[ ]")
	return t
}
