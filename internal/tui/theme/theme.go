package theme

import "github.com/charmbracelet/lipgloss"

// Adaptive colors that work on both light and dark terminal backgrounds.
var (
	Title      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "57", Dark: "99"})
	Cursor     = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "30", Dark: "86"})
	Selected   = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "128", Dark: "170"})
	Dim        = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "247", Dark: "241"})
	Count      = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "128", Dark: "170"})
	Help       = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "247", Dark: "241"})
	Breadcrumb = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "25", Dark: "63"})
	Success    = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "28", Dark: "42"})
	Error      = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "124", Dark: "196"})
	Bold       = lipgloss.NewStyle().Bold(true)
	Check      = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "128", Dark: "170"}).SetString("[x]")
	Uncheck    = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "247", Dark: "241"}).SetString("[ ]")
)
