package planbrowse

import (
	"os"
	"os/exec"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2"

	"github.com/baz-sh/clsm/internal/plan"
	"github.com/baz-sh/clsm/internal/tui/theme"
)

// Messages for async operations.
type startLoadMsg struct{}

type plansResultMsg struct {
	plans []plan.Plan
	err   error
}

type editorFinishedMsg struct{ err error }
type deleteResultMsg []plan.DeleteResult

// --- Async command launchers ---

func loadPlansCmd() tea.Cmd {
	return func() tea.Msg {
		plans, err := plan.ListPlans()
		return plansResultMsg{plans: plans, err: err}
	}
}

func deletePlansCmd(plans []plan.Plan) tea.Cmd {
	return func() tea.Msg {
		results := plan.Delete(plans)
		return deleteResultMsg(results)
	}
}

// --- Main Update ---

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.BackgroundColorMsg:
		m.isDark = msg.IsDark()
		m.theme = theme.New(m.isDark)
		return m, nil
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	}

	switch m.phase {
	case phaseLoading:
		return m.updateLoading(msg)
	case phasePlans:
		return m.updatePlans(msg)
	case phaseViewPlan:
		return m.updateViewPlan(msg)
	case phaseConfirmDelete:
		return m.updateConfirmDelete(msg)
	case phaseDeleting:
		return m.updateDeleting(msg)
	case phaseDeleteResults:
		return m.updateDeleteResults(msg)
	}

	return m, nil
}

// --- Phase handlers ---

func (m Model) updateLoading(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case startLoadMsg:
		return m, tea.Batch(m.spinner.Tick, loadPlansCmd())

	case plansResultMsg:
		if msg.err != nil {
			m.plans = nil
		} else {
			m.plans = msg.plans
		}
		m.filteredPlans = allIndices(len(m.plans))
		m.cursor = 0
		m.phase = phasePlans
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) updatePlans(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.filtering {
		return m.updateFilterInput(msg)
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keys.Quit), key.Matches(msg, m.keys.Back):
			m.BackToHome = true
			return m, tea.Quit
		case key.Matches(msg, m.keys.Up):
			if m.cursor > 0 {
				m.cursor--
			}
		case key.Matches(msg, m.keys.Down):
			if m.cursor < len(m.filteredPlans)-1 {
				m.cursor++
			}
		case key.Matches(msg, m.keys.Top):
			m.cursor = 0
		case key.Matches(msg, m.keys.Bottom):
			if len(m.filteredPlans) > 0 {
				m.cursor = len(m.filteredPlans) - 1
			}
		case key.Matches(msg, m.keys.HalfUp):
			m.cursor -= m.pageSize() / 2
			if m.cursor < 0 {
				m.cursor = 0
			}
		case key.Matches(msg, m.keys.HalfDn):
			m.cursor += m.pageSize() / 2
			if m.cursor >= len(m.filteredPlans) {
				m.cursor = len(m.filteredPlans) - 1
			}
			if m.cursor < 0 {
				m.cursor = 0
			}
		case key.Matches(msg, m.keys.Open):
			if len(m.filteredPlans) == 0 || len(m.selected) > 0 {
				return m, nil
			}
			idx := m.filteredPlans[m.cursor]
			m.viewingPlan = m.plans[idx]
			// Read full file content for viewing.
			data, err := os.ReadFile(m.viewingPlan.FullPath)
			if err != nil {
				m.viewingContent = "Error reading file: " + err.Error()
			} else {
				m.viewingContent = string(data)
			}
			m.renderedContent = renderMarkdown(m.viewingContent, m.isDark, m.width)
			m.scrollOffset = 0
			m.phase = phaseViewPlan
			return m, nil
		case key.Matches(msg, m.keys.Toggle):
			if len(m.filteredPlans) == 0 {
				return m, nil
			}
			planIdx := m.filteredPlans[m.cursor]
			if m.selected[planIdx] {
				delete(m.selected, planIdx)
			} else {
				m.selected[planIdx] = true
			}
			if m.cursor < len(m.filteredPlans)-1 {
				m.cursor++
			}
			return m, nil
		case key.Matches(msg, m.keys.SelAll):
			for _, idx := range m.filteredPlans {
				m.selected[idx] = true
			}
			return m, nil
		case key.Matches(msg, m.keys.DeselAll):
			m.selected = make(map[int]bool)
			return m, nil
		case key.Matches(msg, m.keys.Delete):
			if len(m.selected) == 0 {
				return m, nil
			}
			m.phase = phaseConfirmDelete
			return m, nil
		case key.Matches(msg, m.keys.Search):
			m.filtering = true
			m.filter.SetValue("")
			return m, m.filter.Focus()
		}
	}

	return m, nil
}

func (m Model) updateViewPlan(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case editorFinishedMsg:
		// Re-read file in case it was edited.
		data, err := os.ReadFile(m.viewingPlan.FullPath)
		if err == nil {
			m.viewingContent = string(data)
			m.renderedContent = renderMarkdown(m.viewingContent, m.isDark, m.width)
		}
		return m, nil
	case tea.KeyPressMsg:
		lines := strings.Split(m.renderedContent, "\n")
		viewHeight := m.height - 6
		if viewHeight < 1 {
			viewHeight = 1
		}
		maxScroll := len(lines) - viewHeight
		if maxScroll < 0 {
			maxScroll = 0
		}

		switch {
		case key.Matches(msg, m.keys.Quit), key.Matches(msg, m.keys.Back):
			m.phase = phasePlans
			return m, nil
		case key.Matches(msg, m.keys.Edit):
			editor := os.Getenv("EDITOR")
			if editor == "" {
				editor = "vi"
			}
			c := exec.Command(editor, m.viewingPlan.FullPath)
			return m, tea.ExecProcess(c, func(err error) tea.Msg {
				return editorFinishedMsg{err: err}
			})
		case key.Matches(msg, m.keys.Down):
			if m.scrollOffset < maxScroll {
				m.scrollOffset++
			}
		case key.Matches(msg, m.keys.Up):
			if m.scrollOffset > 0 {
				m.scrollOffset--
			}
		case key.Matches(msg, m.keys.HalfDn):
			m.scrollOffset += viewHeight / 2
			if m.scrollOffset > maxScroll {
				m.scrollOffset = maxScroll
			}
		case key.Matches(msg, m.keys.HalfUp):
			m.scrollOffset -= viewHeight / 2
			if m.scrollOffset < 0 {
				m.scrollOffset = 0
			}
		case key.Matches(msg, m.keys.Top):
			m.scrollOffset = 0
		case key.Matches(msg, m.keys.Bottom):
			m.scrollOffset = maxScroll
		}
	}

	return m, nil
}

func (m Model) updateConfirmDelete(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keys.Yes):
			m.phase = phaseDeleting
			return m, tea.Batch(m.spinner.Tick, deletePlansCmd(m.selectedPlans()))
		case key.Matches(msg, m.keys.No), key.Matches(msg, m.keys.Back):
			m.phase = phasePlans
			return m, nil
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m Model) updateDeleting(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case deleteResultMsg:
		m.deleteResults = []plan.DeleteResult(msg)
		m.phase = phaseDeleteResults
		return m, nil
	default:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
}

func (m Model) updateDeleteResults(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keys.Open), key.Matches(msg, m.keys.Back), key.Matches(msg, m.keys.Quit):
			// Reload plans.
			deletedFiles := make(map[string]bool)
			for _, r := range m.deleteResults {
				if r.Success {
					deletedFiles[r.FileName] = true
				}
			}
			var remaining []plan.Plan
			for _, p := range m.plans {
				if !deletedFiles[p.FileName] {
					remaining = append(remaining, p)
				}
			}
			m.plans = remaining
			m.filteredPlans = allIndices(len(m.plans))
			m.selected = make(map[int]bool)
			if m.cursor >= len(m.filteredPlans) {
				m.cursor = len(m.filteredPlans) - 1
			}
			if m.cursor < 0 {
				m.cursor = 0
			}
			m.deleteResults = nil
			m.phase = phasePlans
			return m, nil
		}
	}
	return m, nil
}

// --- Filter ---

func (m Model) updateFilterInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "enter":
			m.filtering = false
			m.filter.Blur()
			m.applyFilter()
			return m, nil
		case "esc":
			m.filtering = false
			m.filter.Blur()
			m.filter.SetValue("")
			m.filteredPlans = allIndices(len(m.plans))
			m.cursor = 0
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.filter, cmd = m.filter.Update(msg)
	m.applyFilter()

	return m, cmd
}

func (m *Model) applyFilter() {
	term := strings.ToLower(m.filter.Value())

	if term == "" {
		m.filteredPlans = allIndices(len(m.plans))
	} else {
		m.filteredPlans = m.filteredPlans[:0]
		for i, p := range m.plans {
			searchable := strings.ToLower(
				p.Title + " " + p.Context + " " + p.ProjectHint + " " + p.FileName,
			)
			if strings.Contains(searchable, term) {
				m.filteredPlans = append(m.filteredPlans, i)
			}
		}
	}
	if m.cursor >= len(m.filteredPlans) {
		m.cursor = len(m.filteredPlans) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

// --- Helpers ---

func renderMarkdown(content string, isDark bool, width int) string {
	style := "dark"
	if !isDark {
		style = "light"
	}

	w := width - 4
	if w < 40 {
		w = 40
	}

	r, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle(style),
		glamour.WithWordWrap(w),
	)
	if err != nil {
		return content
	}

	out, err := r.Render(content)
	if err != nil {
		return content
	}

	return strings.TrimRight(out, "\n")
}

func allIndices(n int) []int {
	idx := make([]int, n)
	for i := range idx {
		idx[i] = i
	}
	return idx
}
