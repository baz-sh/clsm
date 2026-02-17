package browse

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/baz-sh/clsm/internal/session"
	"github.com/baz-sh/clsm/internal/tui/theme"
)

type phase int

const (
	phaseLoadingProjects phase = iota
	phaseProjects
	phaseLoadingSessions
	phaseSessions
	phaseRename
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
	progress progress.Model
	progressPct  float64
	progressInfo string
	progressCh   <-chan session.LoadProgress
	resultCh     <-chan projectsResultMsg
	filter   textinput.Model
	filtering bool

	projects       []projectItem
	filteredProjs  []int // indices into projects
	projCursor     int

	selectedProject session.Project
	sessions       []sessionItem
	filteredSess   []int // indices into sessions
	sessCursor     int

	renameInput  textinput.Model
	renameIdx    int // index into sessions being renamed
	status       string
	BackToHome   bool
	width        int
	height       int
}


// New creates a new browse Model.
func New() Model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot

	fi := textinput.New()
	fi.Placeholder = "filter..."
	fi.CharLimit = 256
	fi.Width = 40

	ri := textinput.New()
	ri.Placeholder = "new title..."
	ri.CharLimit = 256
	ri.Width = 50

	prog := progress.New(
		progress.WithScaledGradient("#6C50A3", "#57CC99"),
		progress.WithWidth(40),
	)

	return Model{
		phase:       phaseLoadingProjects,
		keys:        newKeyMap(),
		spinner:     sp,
		progress:    prog,
		filter:      fi,
		renameInput: ri,
		width:       80,
		height:      24,
	}
}

func (m Model) Init() tea.Cmd {
	return func() tea.Msg { return startLoadMsg{} }
}

func (m Model) View() string {
	switch m.phase {
	case phaseLoadingProjects:
		var b strings.Builder
		b.WriteString(theme.Title.Render("clsm — Browse Projects"))
		b.WriteString("\n\n")
		b.WriteString(m.progress.ViewAs(m.progressPct))
		b.WriteString("\n\n")
		if m.progressInfo != "" {
			b.WriteString(theme.Dim.Render(m.progressInfo))
		} else {
			b.WriteString(theme.Dim.Render("Loading projects..."))
		}
		b.WriteString("\n")
		return b.String()
	case phaseProjects:
		return m.viewProjects()
	case phaseLoadingSessions:
		return fmt.Sprintf("%s Loading sessions...\n", m.spinner.View())
	case phaseSessions:
		return m.viewSessions()
	case phaseRename:
		return m.viewRename()
	}
	return ""
}

const pageSize = 15

func (m Model) viewProjects() string {
	var b strings.Builder
	b.WriteString(theme.Title.Render("clsm — Browse Projects"))
	b.WriteString("\n\n")

	if m.filtering {
		b.WriteString(m.filter.View())
		b.WriteString("\n\n")
	}

	items := m.filteredProjs
	cursor := m.projCursor

	page := cursor / pageSize
	start := page * pageSize
	end := start + pageSize
	if end > len(items) {
		end = len(items)
	}

	for vi := start; vi < end; vi++ {
		p := m.projects[items[vi]].project
		path := shortenPath(p.Path)

		prefix := "  "
		style := lipgloss.NewStyle()
		if vi == cursor {
			prefix = theme.Cursor.Render("> ")
			style = theme.Cursor
		}

		count := theme.Count.Render(fmt.Sprintf("[%d]", p.SessionCount))
		b.WriteString(fmt.Sprintf("%s%s %s\n", prefix, style.Render(path), count))

		mod := formatTime(p.LastModified)
		detail := "last modified: " + mod
		if p.LastPrompt != "" {
			detail += " • " + truncate(p.LastPrompt, m.width-len(mod)-22)
		}
		b.WriteString(fmt.Sprintf("    %s\n", theme.Dim.Render(detail)))
	}

	if len(items) == 0 {
		b.WriteString(theme.Dim.Render("  No projects found."))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	if m.status != "" {
		b.WriteString(theme.Dim.Render(m.status))
		b.WriteString("\n")
	}
	totalPages := (len(items) + pageSize - 1) / pageSize
	if totalPages < 1 {
		totalPages = 1
	}
	b.WriteString(fmt.Sprintf(" %d projects • Page %d/%d", len(items), page+1, totalPages))
	b.WriteString("\n")
	if m.filtering {
		b.WriteString(theme.Help.Render("enter: apply filter • esc: clear filter"))
	} else {
		b.WriteString(theme.Help.Render("j/k: navigate • enter/l: open • /: filter • q/esc: back"))
	}

	return b.String()
}

func (m Model) viewSessions() string {
	var b strings.Builder
	b.WriteString(theme.Title.Render("clsm — Browse Sessions"))
	b.WriteString("  ")
	b.WriteString(theme.Breadcrumb.Render(shortenPath(m.selectedProject.Path)))
	b.WriteString("\n\n")

	if m.filtering {
		b.WriteString(m.filter.View())
		b.WriteString("\n\n")
	}

	items := m.filteredSess
	cursor := m.sessCursor

	page := cursor / pageSize
	start := page * pageSize
	end := start + pageSize
	if end > len(items) {
		end = len(items)
	}

	for vi := start; vi < end; vi++ {
		s := m.sessions[items[vi]].session
		title := displayTitle(s)

		prefix := "  "
		style := lipgloss.NewStyle()
		if vi == cursor {
			prefix = theme.Cursor.Render("> ")
			style = theme.Cursor
		}

		msgs := theme.Count.Render(fmt.Sprintf("[%d msgs]", s.MsgCount))
		b.WriteString(fmt.Sprintf("%s%s %s\n", prefix, style.Render(title), msgs))

		mod := formatTime(s.Modified)
		branch := ""
		if s.GitBranch != "" {
			branch = " • " + s.GitBranch
		}
		b.WriteString(fmt.Sprintf("    %s\n", theme.Dim.Render(mod+branch)))

		prompt := truncate(s.FirstPrompt, m.width-6)
		if prompt != "" {
			b.WriteString(fmt.Sprintf("    %s\n", theme.Dim.Render(prompt)))
		}
	}

	if len(items) == 0 {
		b.WriteString(theme.Dim.Render("  No sessions found."))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	totalPages := (len(items) + pageSize - 1) / pageSize
	if totalPages < 1 {
		totalPages = 1
	}
	b.WriteString(fmt.Sprintf(" %d sessions • Page %d/%d", len(items), page+1, totalPages))
	b.WriteString("\n")
	if m.filtering {
		b.WriteString(theme.Help.Render("enter: apply filter • esc: clear filter"))
	} else {
		b.WriteString(theme.Help.Render("j/k: navigate • /: filter • r: rename • q/esc/h: back"))
	}

	return b.String()
}

func (m Model) viewRename() string {
	var b strings.Builder
	b.WriteString(theme.Title.Render("clsm — Rename Session"))
	b.WriteString("\n\n")

	s := m.sessions[m.renameIdx].session
	current := displayTitle(s)
	b.WriteString(fmt.Sprintf("Current: %s\n\n", theme.Dim.Render(current)))
	b.WriteString("New title:\n\n")
	b.WriteString(m.renameInput.View())
	b.WriteString("\n\n")
	if m.status != "" {
		b.WriteString(theme.Dim.Render(m.status))
		b.WriteString("\n\n")
	}
	b.WriteString(theme.Help.Render("enter: save • esc: cancel"))
	return b.String()
}

// WantsBackToHome returns true if the user quit to return to the home menu.
func (m Model) WantsBackToHome() bool {
	return m.BackToHome
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
