package delete

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/baz-sh/clsm/internal/session"
	"github.com/baz-sh/clsm/internal/tui/theme"
)

// Phase represents the current TUI state.
type phase int

const (
	phaseSearch   phase = iota
	phaseLoading
	phaseSelect
	phaseConfirm
	phaseDeleting
	phaseResults
)

// sessionItem wraps a Session to work as a list item in the TUI.
type sessionItem struct {
	session  session.Session
	selected bool
}

// Model is the Bubble Tea model for the delete TUI.
type Model struct {
	phase      phase
	input      textinput.Model
	spinner    spinner.Model
	keys       keyMap
	items      []sessionItem
	cursor     int
	offset     int // scroll offset
	results    []session.DeleteResult
	status     string // status message shown in search phase
	searchTerm string // the term used for the current search
	width      int
	height     int
}


// New creates a new Model for the delete TUI.
func New() Model {
	ti := textinput.New()
	ti.Placeholder = "Enter search term..."
	ti.Focus()
	ti.CharLimit = 256
	ti.Width = 50

	sp := spinner.New()
	sp.Spinner = spinner.Dot

	return Model{
		phase:   phaseSearch,
		input:   ti,
		spinner: sp,
		keys:    newKeyMap(),
		width:   80,
		height:  24,
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

// View implements tea.Model.
func (m Model) View() string {
	switch m.phase {
	case phaseSearch:
		return m.viewSearch()
	case phaseLoading:
		return m.viewLoading()
	case phaseSelect:
		return m.viewSelect()
	case phaseConfirm:
		return m.viewConfirm()
	case phaseDeleting:
		return m.viewDeleting()
	case phaseResults:
		return m.viewResults()
	}
	return ""
}

func (m Model) viewSearch() string {
	var b strings.Builder
	b.WriteString(theme.Title.Render("clsm — Delete Sessions"))
	b.WriteString("\n\n")
	b.WriteString("Search for sessions to delete:\n\n")
	b.WriteString(m.input.View())
	b.WriteString("\n\n")
	if m.status != "" {
		b.WriteString(theme.Dim.Render(m.status))
		b.WriteString("\n\n")
	}
	b.WriteString(theme.Help.Render("enter: search • esc/q: quit"))
	return b.String()
}

func (m Model) viewLoading() string {
	return fmt.Sprintf("%s Searching...\n", m.spinner.View())
}

func (m Model) viewSelect() string {
	var b strings.Builder
	b.WriteString(theme.Title.Render("Select sessions to delete"))
	b.WriteString("\n\n")

	// Calculate visible area (reserve lines for header, footer, help).
	visibleHeight := m.height - 7
	if visibleHeight < 3 {
		visibleHeight = 3
	}

	// Each item takes 3 lines (title + detail + blank).
	itemsPerPage := visibleHeight / 3
	if itemsPerPage < 1 {
		itemsPerPage = 1
	}

	// Adjust scroll offset to keep cursor visible.
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+itemsPerPage {
		m.offset = m.cursor - itemsPerPage + 1
	}

	end := m.offset + itemsPerPage
	if end > len(m.items) {
		end = len(m.items)
	}

	for i := m.offset; i < end; i++ {
		item := m.items[i]
		title := displayTitle(item.session)

		check := theme.Uncheck.String()
		if item.selected {
			check = theme.Check.String()
		}

		prefix := "  "
		style := theme.Dim
		if i == m.cursor {
			prefix = theme.Cursor.Render("> ")
			style = lipgloss.NewStyle()
		}
		if item.selected {
			style = theme.Selected
		}

		highlightedTitle := highlightMatch(title, m.searchTerm)
		b.WriteString(fmt.Sprintf("%s%s %s\n", prefix, check, style.Render(highlightedTitle)))

		matchPreview := truncate(item.session.MatchValue, 80)
		highlightedMatch := highlightMatch(matchPreview, m.searchTerm)
		detail := fmt.Sprintf("     %s • %d msgs • matched %s: %s",
			item.session.ProjectPath, item.session.MsgCount, item.session.MatchSource, highlightedMatch)
		b.WriteString(theme.Dim.Render(detail))
		b.WriteString("\n")
	}

	selected := m.countSelected()
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf(" %d/%d selected", selected, len(m.items)))
	b.WriteString("\n")
	b.WriteString(theme.Help.Render("j/k: navigate • space: toggle • a/A: sel/desel all • d/enter: delete • /: search • q: quit"))

	return b.String()
}

func (m Model) viewConfirm() string {
	selected := m.countSelected()
	var b strings.Builder
	b.WriteString(theme.Title.Render("Confirm Deletion"))
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("Delete %d session(s)?\n\n", selected))

	for _, item := range m.items {
		if item.selected {
			title := displayTitle(item.session)
			b.WriteString(fmt.Sprintf("  • %s\n", title))
		}
	}

	b.WriteString("\n")
	b.WriteString(theme.Help.Render("y: yes • n: no"))
	return b.String()
}

func (m Model) viewDeleting() string {
	return fmt.Sprintf("%s Deleting sessions...\n", m.spinner.View())
}

func (m Model) viewResults() string {
	var b strings.Builder
	b.WriteString(theme.Title.Render("Results"))
	b.WriteString("\n\n")

	var succeeded, failed int
	for _, r := range m.results {
		if r.Success {
			b.WriteString(theme.Success.Render(fmt.Sprintf("  ✓ Deleted %s", r.SessionID)))
			succeeded++
		} else {
			b.WriteString(theme.Error.Render(fmt.Sprintf("  ✗ Failed %s: %s", r.SessionID, r.Error)))
			failed++
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  %d succeeded, %d failed\n\n", succeeded, failed))
	b.WriteString(theme.Help.Render("enter: new search • q: quit"))
	return b.String()
}

func (m Model) countSelected() int {
	n := 0
	for _, item := range m.items {
		if item.selected {
			n++
		}
	}
	return n
}

func (m Model) selectedSessions() []session.Session {
	var sessions []session.Session
	for _, item := range m.items {
		if item.selected {
			sessions = append(sessions, item.session)
		}
	}
	return sessions
}

// highlightMatch returns the string with the matched substring rendered bold.
func highlightMatch(s, term string) string {
	if term == "" {
		return s
	}
	lower := strings.ToLower(s)
	lowerTerm := strings.ToLower(term)
	idx := strings.Index(lower, lowerTerm)
	if idx < 0 {
		return s
	}
	before := s[:idx]
	match := s[idx : idx+len(term)]
	after := s[idx+len(term):]
	return before + theme.Bold.Render(match) + after
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

func displayTitle(s session.Session) string {
	if s.CustomTitle != "" {
		return s.CustomTitle
	}
	if s.Summary != "" {
		return s.Summary
	}
	if s.FirstPrompt != "" {
		return s.FirstPrompt
	}
	return s.SessionID
}
