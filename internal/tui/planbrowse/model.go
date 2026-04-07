package planbrowse

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/baz-sh/clsm/internal/plan"
	"github.com/baz-sh/clsm/internal/tui/theme"
)

type phase int

const (
	phaseLoading       phase = iota
	phasePlans
	phaseViewPlan
	phaseConfirmDelete
	phaseDeleting
	phaseDeleteResults
)

// Model is the Bubble Tea model for the plan browser TUI.
type Model struct {
	phase   phase
	keys    keyMap
	isDark  bool
	theme   theme.Theme
	spinner spinner.Model
	filter  textinput.Model
	filtering bool

	// Plans
	plans         []plan.Plan
	filteredPlans []int
	cursor        int
	selected      map[int]bool

	// View plan content
	viewingPlan     plan.Plan
	viewingContent  string // raw file content
	renderedContent string // glamour-rendered
	scrollOffset    int

	// Delete
	deleteResults []plan.DeleteResult

	BackToHome bool
	width      int
	height     int
}

// New creates a new plan browser Model.
func New() Model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot

	fi := textinput.New()
	fi.Placeholder = "filter..."
	fi.CharLimit = 256
	fi.SetWidth(40)

	return Model{
		phase:    phaseLoading,
		keys:     newKeyMap(),
		theme:    theme.New(true),
		spinner:  sp,
		filter:   fi,
		selected: make(map[int]bool),
		width:    80,
		height:   24,
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
	case phaseLoading:
		content = fmt.Sprintf("%s Loading plans...\n", m.spinner.View())
	case phasePlans:
		content = m.viewPlans()
	case phaseViewPlan:
		content = m.viewPlanContent()
	case phaseConfirmDelete:
		content = m.viewConfirmDelete()
	case phaseDeleting:
		content = fmt.Sprintf("%s Deleting plans...\n", m.spinner.View())
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

const maxPageSize = 15

func (m Model) pageSize() int {
	overhead := 5
	if m.filtering {
		overhead += 2
	}
	// Each plan takes 3 lines: title, detail, context.
	ps := (m.height - overhead) / 3
	if ps < 1 {
		ps = 1
	}
	if ps > maxPageSize {
		ps = maxPageSize
	}
	return ps
}

func (m Model) viewPlans() string {
	var b strings.Builder
	b.WriteString(m.theme.Title.Render("clsm — Plans"))
	b.WriteString("\n\n")

	if m.filtering {
		b.WriteString(m.filter.View())
		b.WriteString("\n\n")
	}

	items := m.filteredPlans
	cursor := m.cursor

	ps := m.pageSize()
	page := cursor / ps
	start := page * ps
	end := start + ps
	if end > len(items) {
		end = len(items)
	}

	for vi := start; vi < end; vi++ {
		planIdx := items[vi]
		p := m.plans[planIdx]

		check := m.theme.Uncheck.String()
		if m.selected[planIdx] {
			check = m.theme.Check.String()
		}

		prefix := "  "
		style := lipgloss.NewStyle()
		if vi == cursor {
			prefix = m.theme.Cursor.Render("> ")
			style = m.theme.Cursor
		}
		if m.selected[planIdx] {
			style = m.theme.Selected
		}

		b.WriteString(fmt.Sprintf("%s%s %s\n", prefix, check, style.Render(p.Title)))

		// Detail line: project hint + date + size.
		var details []string
		if p.ProjectHint != "" {
			details = append(details, p.ProjectHint)
		}
		mod := formatTime(p.ModTime)
		if mod != "" {
			details = append(details, mod)
		}
		details = append(details, formatSize(p.Size))
		b.WriteString(fmt.Sprintf("      %s\n", m.theme.Dim.Render(strings.Join(details, " • "))))

		// Context line (if available).
		if p.Context != "" {
			ctx := truncate(p.Context, m.width-8)
			b.WriteString(fmt.Sprintf("      %s\n", m.theme.Dim.Render(ctx)))
		}
	}

	if len(items) == 0 {
		b.WriteString(m.theme.Dim.Render("  No plans found."))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	selectedCount := len(m.selected)
	totalPages := (len(items) + ps - 1) / ps
	if totalPages < 1 {
		totalPages = 1
	}
	if selectedCount > 0 {
		b.WriteString(fmt.Sprintf(" %d plans • %d selected • Page %d/%d", len(items), selectedCount, page+1, totalPages))
	} else {
		b.WriteString(fmt.Sprintf(" %d plans • Page %d/%d", len(items), page+1, totalPages))
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

func (m Model) viewPlanContent() string {
	var b strings.Builder
	b.WriteString(m.theme.Title.Render(m.viewingPlan.Title))
	b.WriteString("\n")

	var meta []string
	mod := formatTime(m.viewingPlan.ModTime)
	if mod != "" {
		meta = append(meta, mod)
	}
	meta = append(meta, formatSize(m.viewingPlan.Size))
	if m.viewingPlan.ProjectHint != "" {
		meta = append(meta, m.viewingPlan.ProjectHint)
	}
	b.WriteString(m.theme.Dim.Render(strings.Join(meta, " • ")))
	b.WriteString("\n")
	b.WriteString(m.theme.Dim.Render("Plan file: " + m.viewingPlan.FullPath))
	b.WriteString("\n")
	b.WriteString(m.theme.Dim.Render(strings.Repeat("─", min(m.width, 60))))
	b.WriteString("\n")

	// Scrollable content.
	lines := strings.Split(m.renderedContent, "\n")
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
	b.WriteString(m.theme.Help.Render("j/k: scroll • e: open in $EDITOR • q/esc: back"))

	return b.String()
}

func (m Model) viewConfirmDelete() string {
	selected := m.selectedPlans()
	var b strings.Builder
	b.WriteString(m.theme.Title.Render("Confirm Deletion"))
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("Delete %d plan file(s)?\n\n", len(selected)))

	for _, p := range selected {
		b.WriteString(fmt.Sprintf("  • %s\n", p.Title))
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
	b.WriteString(m.theme.Help.Render("enter/esc: back to plans"))
	return b.String()
}

// selectedPlans returns the selected Plan objects.
func (m Model) selectedPlans() []plan.Plan {
	var plans []plan.Plan
	for idx := range m.selected {
		if idx < len(m.plans) {
			plans = append(plans, m.plans[idx])
		}
	}
	return plans
}

// --- Helpers ---

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

func formatSize(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%dB", bytes)
	}
	return fmt.Sprintf("%.0fKB", float64(bytes)/1024)
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
