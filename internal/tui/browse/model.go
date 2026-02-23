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

// StartMode determines the initial view when launching the browse TUI.
type StartMode int

const (
	ModeProjects StartMode = iota
	ModeSessions
	ModeSearch
)

type phase int

const (
	phaseLoadingProjects phase = iota
	phaseProjects
	phaseLoadingSessions
	phaseLoadingAllSessions
	phaseSearchInput
	phaseSearching
	phaseSessions
	phaseRename
	phaseConfirmDelete
	phaseDeleting
	phaseDeleteResults
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
	startMode    StartMode
	phase        phase
	keys         keyMap
	spinner      spinner.Model
	progress     progress.Model
	progressPct  float64
	progressInfo string
	progressCh   <-chan session.LoadProgress
	resultCh     <-chan projectsResultMsg
	filter       textinput.Model
	filtering    bool

	// Projects
	projects      []projectItem
	filteredProjs []int // indices into projects
	projCursor    int

	// Sessions (shared across project/all/search sources)
	selectedProject session.Project
	sessionSource   string // "project", "all", "search"
	sessions        []sessionItem
	filteredSess    []int // indices into sessions
	sessCursor      int
	selected        map[int]bool // multi-select: keys are indices into sessions

	// Rename
	renameInput textinput.Model
	renameIdx   int // index into sessions being renamed

	// Search
	searchInput      textinput.Model
	searchTerm       string
	searchProgressCh <-chan session.SearchProgress
	searchResultCh   <-chan searchResultMsg

	// All-sessions loading
	allSessResultCh <-chan allSessionsResultMsg

	// Delete
	deleteResults []session.DeleteResult

	status     string
	BackToHome bool
	width      int
	height     int
}

// New creates a new browse Model with the given start mode.
func New(mode StartMode) Model {
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

	si := textinput.New()
	si.Placeholder = "Enter search term..."
	si.CharLimit = 256
	si.Width = 50

	prog := progress.New(
		progress.WithScaledGradient("#6C50A3", "#57CC99"),
		progress.WithWidth(40),
	)

	var initialPhase phase
	switch mode {
	case ModeProjects:
		initialPhase = phaseLoadingProjects
	case ModeSessions:
		initialPhase = phaseLoadingAllSessions
	case ModeSearch:
		initialPhase = phaseSearchInput
		si.Focus()
	}

	return Model{
		startMode:   mode,
		phase:       initialPhase,
		keys:        newKeyMap(),
		spinner:     sp,
		progress:    prog,
		filter:      fi,
		renameInput: ri,
		searchInput: si,
		selected:    make(map[int]bool),
		width:       80,
		height:      24,
	}
}

func (m Model) Init() tea.Cmd {
	switch m.startMode {
	case ModeProjects:
		return func() tea.Msg { return startLoadMsg{} }
	case ModeSessions:
		return func() tea.Msg { return startAllSessionsMsg{} }
	case ModeSearch:
		return textinput.Blink
	}
	return nil
}

func (m Model) View() string {
	switch m.phase {
	case phaseLoadingProjects:
		return m.viewLoading("Loading projects...")
	case phaseProjects:
		return m.viewProjects()
	case phaseLoadingSessions:
		return fmt.Sprintf("%s Loading sessions...\n", m.spinner.View())
	case phaseLoadingAllSessions:
		return m.viewLoading("Loading sessions...")
	case phaseSearchInput:
		return m.viewSearchInput()
	case phaseSearching:
		return m.viewLoading("Searching...")
	case phaseSessions:
		return m.viewSessions()
	case phaseRename:
		return m.viewRename()
	case phaseConfirmDelete:
		return m.viewConfirmDelete()
	case phaseDeleting:
		return fmt.Sprintf("%s Deleting sessions...\n", m.spinner.View())
	case phaseDeleteResults:
		return m.viewDeleteResults()
	}
	return ""
}

// viewLoading renders a progress bar with a message.
func (m Model) viewLoading(defaultMsg string) string {
	var b strings.Builder
	b.WriteString(theme.Title.Render("clsm — Browse"))
	b.WriteString("\n\n")
	b.WriteString(m.progress.ViewAs(m.progressPct))
	b.WriteString("\n\n")
	if m.progressInfo != "" {
		b.WriteString(theme.Dim.Render(m.progressInfo))
	} else {
		b.WriteString(theme.Dim.Render(defaultMsg))
	}
	b.WriteString("\n")
	return b.String()
}

const maxPageSize = 15

// projPageSize returns the number of project items that fit on screen.
// Each project takes 2 lines (name + detail).
func (m Model) projPageSize() int {
	overhead := 5
	if m.filtering {
		overhead += 2
	}
	ps := (m.height - overhead) / 2
	if ps < 1 {
		ps = 1
	}
	if ps > maxPageSize {
		ps = maxPageSize
	}
	return ps
}

// sessPageSize returns the number of session items that fit on screen.
// Each session takes up to 3 lines (title + date + optional prompt).
func (m Model) sessPageSize() int {
	overhead := 5
	if m.filtering {
		overhead += 2
	}
	ps := (m.height - overhead) / 3
	if ps < 1 {
		ps = 1
	}
	if ps > maxPageSize {
		ps = maxPageSize
	}
	return ps
}

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

	ps := m.projPageSize()
	page := cursor / ps
	start := page * ps
	end := start + ps
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
	totalPages := (len(items) + ps - 1) / ps
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

	// Title varies by source.
	switch m.sessionSource {
	case "project":
		b.WriteString(theme.Title.Render("clsm — Sessions"))
		b.WriteString("  ")
		b.WriteString(theme.Breadcrumb.Render(shortenPath(m.selectedProject.Path)))
	case "all":
		b.WriteString(theme.Title.Render("clsm — All Sessions"))
	case "search":
		b.WriteString(theme.Title.Render("clsm — Search Results"))
		if m.searchTerm != "" {
			b.WriteString("  ")
			b.WriteString(theme.Breadcrumb.Render("\"" + m.searchTerm + "\""))
		}
	default:
		b.WriteString(theme.Title.Render("clsm — Sessions"))
	}
	b.WriteString("\n\n")

	if m.filtering {
		b.WriteString(m.filter.View())
		b.WriteString("\n\n")
	}

	items := m.filteredSess
	cursor := m.sessCursor
	showProject := m.sessionSource == "all" || m.sessionSource == "search"

	ps := m.sessPageSize()
	page := cursor / ps
	start := page * ps
	end := start + ps
	if end > len(items) {
		end = len(items)
	}

	for vi := start; vi < end; vi++ {
		sessIdx := items[vi]
		s := m.sessions[sessIdx].session
		title := displayTitle(s)

		// Checkbox.
		check := theme.Uncheck.String()
		if m.selected[sessIdx] {
			check = theme.Check.String()
		}

		prefix := "  "
		style := lipgloss.NewStyle()
		if vi == cursor {
			prefix = theme.Cursor.Render("> ")
			style = theme.Cursor
		}
		if m.selected[sessIdx] {
			style = theme.Selected
		}

		msgs := theme.Count.Render(fmt.Sprintf("[%d msgs]", s.MsgCount))
		b.WriteString(fmt.Sprintf("%s%s %s %s\n", prefix, check, style.Render(title), msgs))

		// Detail line.
		mod := formatTime(s.Modified)
		var detail string
		if showProject && s.ProjectPath != "" {
			detail = shortenPath(s.ProjectPath) + " • " + mod
		} else {
			detail = mod
		}
		if s.GitBranch != "" {
			detail += " • " + s.GitBranch
		}
		b.WriteString(fmt.Sprintf("      %s\n", theme.Dim.Render(detail)))

		// Optional prompt line.
		prompt := truncate(s.FirstPrompt, m.width-8)
		if prompt != "" {
			b.WriteString(fmt.Sprintf("      %s\n", theme.Dim.Render(prompt)))
		}
	}

	if len(items) == 0 {
		b.WriteString(theme.Dim.Render("  No sessions found."))
		b.WriteString("\n")
	}

	// Footer.
	b.WriteString("\n")
	selectedCount := m.countSelected()
	totalPages := (len(items) + ps - 1) / ps
	if totalPages < 1 {
		totalPages = 1
	}
	if selectedCount > 0 {
		b.WriteString(fmt.Sprintf(" %d sessions • %d selected • Page %d/%d", len(items), selectedCount, page+1, totalPages))
	} else {
		b.WriteString(fmt.Sprintf(" %d sessions • Page %d/%d", len(items), page+1, totalPages))
	}
	b.WriteString("\n")

	if m.filtering {
		b.WriteString(theme.Help.Render("enter: apply filter • esc: clear filter"))
	} else if selectedCount > 0 {
		b.WriteString(theme.Help.Render("j/k: navigate • space: select • a/A: all/none • d: delete • /: filter • q/esc: back"))
	} else {
		b.WriteString(theme.Help.Render("j/k: navigate • space: select • r: rename • /: filter • q/esc: back"))
	}

	return b.String()
}

func (m Model) viewSearchInput() string {
	var b strings.Builder
	b.WriteString(theme.Title.Render("clsm — Search Sessions"))
	b.WriteString("\n\n")
	b.WriteString("Search for sessions:\n\n")
	b.WriteString(m.searchInput.View())
	b.WriteString("\n\n")
	if m.status != "" {
		b.WriteString(theme.Dim.Render(m.status))
		b.WriteString("\n\n")
	}
	b.WriteString(theme.Help.Render("enter: search • esc/q: back"))
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

func (m Model) viewConfirmDelete() string {
	selected := m.selectedSessions()
	var b strings.Builder
	b.WriteString(theme.Title.Render("Confirm Deletion"))
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("Delete %d session(s)?\n\n", len(selected)))

	for _, s := range selected {
		title := displayTitle(s)
		b.WriteString(fmt.Sprintf("  • %s\n", title))
	}

	b.WriteString("\n")
	b.WriteString(theme.Help.Render("y: confirm • n/esc: cancel"))
	return b.String()
}

func (m Model) viewDeleteResults() string {
	var b strings.Builder
	b.WriteString(theme.Title.Render("Delete Results"))
	b.WriteString("\n\n")

	var succeeded, failed int
	for _, r := range m.deleteResults {
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
	backLabel := "back to menu"
	switch m.sessionSource {
	case "project":
		backLabel = "back to projects"
	case "search":
		backLabel = "back to search"
	}
	b.WriteString(theme.Help.Render("enter: back to sessions • q/esc: " + backLabel))
	return b.String()
}

// countSelected returns the number of selected sessions.
func (m Model) countSelected() int {
	return len(m.selected)
}

// selectedSessions returns the selected Session objects.
func (m Model) selectedSessions() []session.Session {
	var sessions []session.Session
	for idx := range m.selected {
		if idx < len(m.sessions) {
			sessions = append(sessions, m.sessions[idx].session)
		}
	}
	return sessions
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
