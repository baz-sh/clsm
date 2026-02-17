package browse

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/baz-sh/clsm/internal/session"
)

type phase int

const (
	phaseLoadingProjects phase = iota
	phaseProjects
	phaseLoadingSessions
	phaseSessions
)

// projectItem wraps a Project for display.
type projectItem struct {
	project session.Project
}

// sessionItem wraps a Session for display.
type sessionItem struct {
	session session.Session
}

// Model is the Bubble Tea model for the browse TUI.
type Model struct {
	phase    phase
	keys     keyMap
	spinner  spinner.Model
	filter   textinput.Model
	filtering bool

	projects       []projectItem
	filteredProjs  []int // indices into projects
	projCursor     int
	projOffset     int

	selectedProject session.Project
	sessions       []sessionItem
	filteredSess   []int // indices into sessions
	sessCursor     int
	sessOffset     int

	status string
	width  int
	height int
}

var (
	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99"))
	cursorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("86"))
	dimStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	countStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("170"))
	helpStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	breadcrumb   = lipgloss.NewStyle().Foreground(lipgloss.Color("63"))
)

// New creates a new browse Model.
func New() Model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot

	fi := textinput.New()
	fi.Placeholder = "filter..."
	fi.CharLimit = 256
	fi.Width = 40

	return Model{
		phase:   phaseLoadingProjects,
		keys:    newKeyMap(),
		spinner: sp,
		filter:  fi,
		width:   80,
		height:  24,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, loadProjectsCmd())
}

func (m Model) View() string {
	switch m.phase {
	case phaseLoadingProjects:
		return fmt.Sprintf("%s Loading projects...\n", m.spinner.View())
	case phaseProjects:
		return m.viewProjects()
	case phaseLoadingSessions:
		return fmt.Sprintf("%s Loading sessions...\n", m.spinner.View())
	case phaseSessions:
		return m.viewSessions()
	}
	return ""
}

func (m Model) viewProjects() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("clsm — Browse Projects"))
	b.WriteString("\n\n")

	if m.filtering {
		b.WriteString(m.filter.View())
		b.WriteString("\n\n")
	}

	visible := m.visibleHeight()
	itemsPerPage := visible / 2
	if itemsPerPage < 1 {
		itemsPerPage = 1
	}

	items := m.filteredProjs
	cursor := m.projCursor
	offset := m.projOffset

	if cursor < offset {
		offset = cursor
	}
	if cursor >= offset+itemsPerPage {
		offset = cursor - itemsPerPage + 1
	}
	m.projOffset = offset

	end := offset + itemsPerPage
	if end > len(items) {
		end = len(items)
	}

	for vi := offset; vi < end; vi++ {
		p := m.projects[items[vi]].project
		path := shortenPath(p.Path)

		prefix := "  "
		style := lipgloss.NewStyle()
		if vi == cursor {
			prefix = cursorStyle.Render("> ")
			style = cursorStyle
		}

		count := countStyle.Render(fmt.Sprintf("[%d]", p.SessionCount))
		b.WriteString(fmt.Sprintf("%s%s %s\n", prefix, style.Render(path), count))

		mod := formatTime(p.LastModified)
		b.WriteString(fmt.Sprintf("    %s\n", dimStyle.Render("last modified: "+mod)))
	}

	if len(items) == 0 {
		b.WriteString(dimStyle.Render("  No projects found."))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	if m.status != "" {
		b.WriteString(dimStyle.Render(m.status))
		b.WriteString("\n")
	}
	b.WriteString(fmt.Sprintf(" %d projects", len(items)))
	b.WriteString("\n")
	if m.filtering {
		b.WriteString(helpStyle.Render("enter: apply filter • esc: clear filter"))
	} else {
		b.WriteString(helpStyle.Render("j/k: navigate • enter/l: open • /: filter • q: quit"))
	}

	return b.String()
}

func (m Model) viewSessions() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("clsm — Browse Sessions"))
	b.WriteString("  ")
	b.WriteString(breadcrumb.Render(shortenPath(m.selectedProject.Path)))
	b.WriteString("\n\n")

	if m.filtering {
		b.WriteString(m.filter.View())
		b.WriteString("\n\n")
	}

	visible := m.visibleHeight()
	itemsPerPage := visible / 3
	if itemsPerPage < 1 {
		itemsPerPage = 1
	}

	items := m.filteredSess
	cursor := m.sessCursor
	offset := m.sessOffset

	if cursor < offset {
		offset = cursor
	}
	if cursor >= offset+itemsPerPage {
		offset = cursor - itemsPerPage + 1
	}
	m.sessOffset = offset

	end := offset + itemsPerPage
	if end > len(items) {
		end = len(items)
	}

	for vi := offset; vi < end; vi++ {
		s := m.sessions[items[vi]].session
		title := displayTitle(s)

		prefix := "  "
		style := lipgloss.NewStyle()
		if vi == cursor {
			prefix = cursorStyle.Render("> ")
			style = cursorStyle
		}

		msgs := countStyle.Render(fmt.Sprintf("[%d msgs]", s.MsgCount))
		b.WriteString(fmt.Sprintf("%s%s %s\n", prefix, style.Render(title), msgs))

		mod := formatTime(s.Modified)
		branch := ""
		if s.GitBranch != "" {
			branch = " • " + s.GitBranch
		}
		b.WriteString(fmt.Sprintf("    %s\n", dimStyle.Render(mod+branch)))

		prompt := truncate(s.FirstPrompt, m.width-6)
		if prompt != "" {
			b.WriteString(fmt.Sprintf("    %s\n", dimStyle.Render(prompt)))
		}
	}

	if len(items) == 0 {
		b.WriteString(dimStyle.Render("  No sessions found."))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(fmt.Sprintf(" %d sessions", len(items)))
	b.WriteString("\n")
	if m.filtering {
		b.WriteString(helpStyle.Render("enter: apply filter • esc: clear filter"))
	} else {
		b.WriteString(helpStyle.Render("j/k: navigate • esc/h: back • /: filter • q: quit"))
	}

	return b.String()
}

func (m Model) visibleHeight() int {
	h := m.height - 8
	if m.filtering {
		h -= 2
	}
	if h < 3 {
		h = 3
	}
	return h
}

func displayTitle(s session.Session) string {
	if s.CustomTitle != "" {
		return s.CustomTitle
	}
	if s.Summary != "" {
		return s.Summary
	}
	if s.FirstPrompt != "" {
		return truncate(s.FirstPrompt, 60)
	}
	return s.SessionID
}

func truncate(s string, max int) string {
	if max < 4 {
		max = 4
	}
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

func shortenPath(path string) string {
	home, _ := strings.CutPrefix(path, "/Users/")
	if home != path {
		parts := strings.SplitN(home, "/", 2)
		if len(parts) == 2 {
			return "~/" + parts[1]
		}
		return "~"
	}
	return path
}

func formatTime(ts string) string {
	if ts == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		t, err = time.Parse(time.RFC3339, ts)
	}
	if err != nil {
		return ts
	}
	return t.Local().Format("2006-01-02 15:04")
}
