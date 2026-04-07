package memorybrowse

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/baz-sh/clsm/internal/memory"
	"github.com/baz-sh/clsm/internal/tui/theme"
)

// StartMode determines the initial view.
type StartMode int

const (
	ModeProjects StartMode = iota
)

type phase int

const (
	phaseLoadingProjects phase = iota
	phaseProjects
	phaseLoadingMemories
	phaseMemories
	phaseViewMemory
	phaseConfirmDelete
	phaseDeleting
	phaseDeleteResults
)

// Model is the Bubble Tea model for the memory browser TUI.
type Model struct {
	startMode StartMode
	phase     phase
	keys      keyMap
	isDark    bool
	theme     theme.Theme
	spinner   spinner.Model
	progress  progress.Model
	progressPct  float64
	progressInfo string
	progressCh   <-chan memory.LoadProgress
	resultCh     <-chan projectsResultMsg
	filter    textinput.Model
	filtering bool

	// Projects with memories
	projects      []memory.MemoryProject
	filteredProjs []int
	projCursor    int

	// Memories within a project
	selectedProject memory.MemoryProject
	memories        []memory.Memory
	filteredMems    []int
	memCursor       int
	selected        map[int]bool

	// View memory content
	viewingMemory memory.Memory
	scrollOffset  int

	// Delete
	deleteResults []memory.DeleteResult

	status     string
	BackToHome bool
	width      int
	height     int
}

// New creates a new memory browser Model.
func New(mode StartMode) Model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot

	fi := textinput.New()
	fi.Placeholder = "filter..."
	fi.CharLimit = 256
	fi.SetWidth(40)

	prog := progress.New(
		progress.WithColors(lipgloss.Color("#6C50A3"), lipgloss.Color("#57CC99")),
		progress.WithWidth(40),
	)

	return Model{
		startMode: mode,
		phase:     phaseLoadingProjects,
		keys:      newKeyMap(),
		theme:     theme.New(true),
		spinner:   sp,
		progress:  prog,
		filter:    fi,
		selected:  make(map[int]bool),
		width:     80,
		height:    24,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tea.RequestBackgroundColor,
		func() tea.Msg { return startLoadMsg{} },
	)
}

func (m Model) View() tea.View {
	var content string
	switch m.phase {
	case phaseLoadingProjects:
		content = m.viewLoading("Loading memory projects...")
	case phaseProjects:
		content = m.viewProjects()
	case phaseLoadingMemories:
		content = fmt.Sprintf("%s Loading memories...\n", m.spinner.View())
	case phaseMemories:
		content = m.viewMemories()
	case phaseViewMemory:
		content = m.viewMemoryContent()
	case phaseConfirmDelete:
		content = m.viewConfirmDelete()
	case phaseDeleting:
		content = fmt.Sprintf("%s Deleting memories...\n", m.spinner.View())
	case phaseDeleteResults:
		content = m.viewDeleteResults()
	}
	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

// WantsBackToHome returns true if the user quit to return to the home menu.
func (m Model) WantsBackToHome() bool {
	return m.BackToHome
}

// --- View helpers ---

func (m Model) viewLoading(defaultMsg string) string {
	var b strings.Builder
	b.WriteString(m.theme.Title.Render("clsm — Memories"))
	b.WriteString("\n\n")
	b.WriteString(m.progress.ViewAs(m.progressPct))
	b.WriteString("\n\n")
	if m.progressInfo != "" {
		b.WriteString(m.theme.Dim.Render(m.progressInfo))
	} else {
		b.WriteString(m.theme.Dim.Render(defaultMsg))
	}
	b.WriteString("\n")
	return b.String()
}

const maxPageSize = 15

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

func (m Model) memPageSize() int {
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
	b.WriteString(m.theme.Title.Render("clsm — Memory Projects"))
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
		p := m.projects[items[vi]]
		path := shortenPath(p.Path)

		prefix := "  "
		style := lipgloss.NewStyle()
		if vi == cursor {
			prefix = m.theme.Cursor.Render("> ")
			style = m.theme.Cursor
		}

		count := m.theme.Count.Render(fmt.Sprintf("[%d memories]", p.MemoryCount))
		b.WriteString(fmt.Sprintf("%s%s %s\n", prefix, style.Render(path), count))

		mod := formatTime(p.LastModified)
		if mod != "" {
			b.WriteString(fmt.Sprintf("    %s\n", m.theme.Dim.Render("last modified: "+mod)))
		} else {
			b.WriteString(fmt.Sprintf("    %s\n", m.theme.Dim.Render("index only")))
		}
	}

	if len(items) == 0 {
		b.WriteString(m.theme.Dim.Render("  No projects with memories found."))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	if m.status != "" {
		b.WriteString(m.theme.Dim.Render(m.status))
		b.WriteString("\n")
	}
	totalPages := (len(items) + ps - 1) / ps
	if totalPages < 1 {
		totalPages = 1
	}
	b.WriteString(fmt.Sprintf(" %d projects • Page %d/%d", len(items), page+1, totalPages))
	b.WriteString("\n")
	if m.filtering {
		b.WriteString(m.theme.Help.Render("enter: apply filter • esc: clear filter"))
	} else {
		b.WriteString(m.theme.Help.Render("j/k: navigate • enter/l: open • /: filter • q/esc: back"))
	}

	return b.String()
}

func (m Model) viewMemories() string {
	var b strings.Builder
	b.WriteString(m.theme.Title.Render("clsm — Memories"))
	b.WriteString("  ")
	b.WriteString(m.theme.Breadcrumb.Render(shortenPath(m.selectedProject.Path)))
	b.WriteString("\n\n")

	if m.filtering {
		b.WriteString(m.filter.View())
		b.WriteString("\n\n")
	}

	items := m.filteredMems
	cursor := m.memCursor

	ps := m.memPageSize()
	page := cursor / ps
	start := page * ps
	end := start + ps
	if end > len(items) {
		end = len(items)
	}

	for vi := start; vi < end; vi++ {
		memIdx := items[vi]
		mem := m.memories[memIdx]

		check := m.theme.Uncheck.String()
		if m.selected[memIdx] {
			check = m.theme.Check.String()
		}

		prefix := "  "
		style := lipgloss.NewStyle()
		if vi == cursor {
			prefix = m.theme.Cursor.Render("> ")
			style = m.theme.Cursor
		}
		if m.selected[memIdx] {
			style = m.theme.Selected
		}

		typeBadge := ""
		if mem.Type != "" {
			typeBadge = " " + m.theme.Count.Render("["+mem.Type+"]")
		}

		b.WriteString(fmt.Sprintf("%s%s %s%s\n", prefix, check, style.Render(mem.Name), typeBadge))

		// Detail line: description or filename + date.
		mod := formatTime(mem.ModTime)
		detail := mem.FileName
		if mem.Description != "" {
			detail = truncate(mem.Description, m.width-8)
		}
		if mod != "" {
			detail += " • " + mod
		}
		b.WriteString(fmt.Sprintf("      %s\n", m.theme.Dim.Render(detail)))
	}

	if len(items) == 0 {
		b.WriteString(m.theme.Dim.Render("  No memories found."))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	selectedCount := len(m.selected)
	totalPages := (len(items) + ps - 1) / ps
	if totalPages < 1 {
		totalPages = 1
	}
	if selectedCount > 0 {
		b.WriteString(fmt.Sprintf(" %d memories • %d selected • Page %d/%d", len(items), selectedCount, page+1, totalPages))
	} else {
		b.WriteString(fmt.Sprintf(" %d memories • Page %d/%d", len(items), page+1, totalPages))
	}
	b.WriteString("\n")

	if m.filtering {
		b.WriteString(m.theme.Help.Render("enter: apply filter • esc: clear filter"))
	} else if selectedCount > 0 {
		b.WriteString(m.theme.Help.Render("j/k: navigate • space: select • a/A: all/none • d: delete • /: filter • q/esc: back"))
	} else {
		b.WriteString(m.theme.Help.Render("j/k: navigate • enter/l: view • space: select • /: filter • q/esc: back"))
	}

	return b.String()
}

func (m Model) viewMemoryContent() string {
	var b strings.Builder
	b.WriteString(m.theme.Title.Render(m.viewingMemory.Name))
	b.WriteString("\n")

	var meta []string
	if m.viewingMemory.Type != "" {
		meta = append(meta, "Type: "+m.viewingMemory.Type)
	}
	if m.viewingMemory.Description != "" {
		meta = append(meta, m.viewingMemory.Description)
	}
	mod := formatTime(m.viewingMemory.ModTime)
	if mod != "" {
		meta = append(meta, mod)
	}
	if len(meta) > 0 {
		b.WriteString(m.theme.Dim.Render(strings.Join(meta, " • ")))
		b.WriteString("\n")
	}
	b.WriteString(m.theme.Dim.Render(strings.Repeat("─", min(m.width, 60))))
	b.WriteString("\n\n")

	// Scrollable content.
	lines := strings.Split(m.viewingMemory.Content, "\n")
	viewHeight := m.height - 6
	if viewHeight < 1 {
		viewHeight = 1
	}

	end := m.scrollOffset + viewHeight
	if end > len(lines) {
		end = len(lines)
	}

	for i := m.scrollOffset; i < end; i++ {
		b.WriteString(lines[i])
		b.WriteString("\n")
	}

	b.WriteString("\n")
	if len(lines) > viewHeight {
		pct := float64(m.scrollOffset) / float64(len(lines)-viewHeight) * 100
		b.WriteString(m.theme.Dim.Render(fmt.Sprintf("Line %d/%d (%.0f%%)", m.scrollOffset+1, len(lines), pct)))
		b.WriteString("  ")
	}
	b.WriteString(m.theme.Help.Render("j/k: scroll • q/esc: back"))

	return b.String()
}

func (m Model) viewConfirmDelete() string {
	selected := m.selectedMemories()
	var b strings.Builder
	b.WriteString(m.theme.Title.Render("Confirm Deletion"))
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("Delete %d memory file(s)?\n\n", len(selected)))

	for _, mem := range selected {
		typeBadge := ""
		if mem.Type != "" {
			typeBadge = " [" + mem.Type + "]"
		}
		b.WriteString(fmt.Sprintf("  • %s%s\n", mem.Name, typeBadge))
	}

	b.WriteString("\n")
	b.WriteString(m.theme.Help.Render("y: confirm • n/esc: cancel"))
	return b.String()
}

func (m Model) viewDeleteResults() string {
	var b strings.Builder
	b.WriteString(m.theme.Title.Render("Delete Results"))
	b.WriteString("\n\n")

	var succeeded, failed int
	for _, r := range m.deleteResults {
		if r.Success {
			b.WriteString(m.theme.Success.Render(fmt.Sprintf("  ✓ Deleted %s", r.FileName)))
			succeeded++
		} else {
			b.WriteString(m.theme.Error.Render(fmt.Sprintf("  ✗ Failed %s: %s", r.FileName, r.Error)))
			failed++
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  %d succeeded, %d failed\n\n", succeeded, failed))
	b.WriteString(m.theme.Help.Render("enter: back to memories • q/esc: back to projects"))
	return b.String()
}

// selectedMemories returns the selected Memory objects.
func (m Model) selectedMemories() []memory.Memory {
	var mems []memory.Memory
	for idx := range m.selected {
		if idx < len(m.memories) {
			mems = append(mems, m.memories[idx])
		}
	}
	return mems
}

// --- Helpers ---

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

func truncate(s string, max int) string {
	if max < 4 {
		max = 4
	}
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}
