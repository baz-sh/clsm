package memorybrowse

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2"

	"github.com/baz-sh/clsm/internal/memory"
	"github.com/baz-sh/clsm/internal/tui/theme"
)

// Messages for async operations.
type startLoadMsg struct{}

type projectsResultMsg struct {
	projects []memory.MemoryProject
	err      error
}

type loadProgressMsg memory.LoadProgress
type memoriesLoadedMsg []memory.Memory
type loadErrorMsg struct{ err error }
type deleteResultMsg []memory.DeleteResult

// --- Async command launchers ---

func startLoadWithProgress(m *Model) tea.Cmd {
	progressCh := make(chan memory.LoadProgress, 10)
	resultCh := make(chan projectsResultMsg, 1)

	go func() {
		projects, err := memory.ListProjectsWithProgress(progressCh)
		resultCh <- projectsResultMsg{projects: projects, err: err}
	}()

	m.progressCh = progressCh
	m.resultCh = resultCh

	return listenForLoadUpdates(m.progressCh, m.resultCh)
}

func listenForLoadUpdates(progressCh <-chan memory.LoadProgress, resultCh <-chan projectsResultMsg) tea.Cmd {
	return func() tea.Msg {
		select {
		case p, ok := <-progressCh:
			if !ok {
				return <-resultCh
			}
			return loadProgressMsg(p)
		case r := <-resultCh:
			return r
		}
	}
}

func loadMemoriesCmd(projectDir string) tea.Cmd {
	return func() tea.Msg {
		memories, err := memory.ListMemories(projectDir)
		if err != nil {
			return loadErrorMsg{err}
		}
		return memoriesLoadedMsg(memories)
	}
}

func deleteMemoriesCmd(memories []memory.Memory) tea.Cmd {
	return func() tea.Msg {
		results := memory.Delete(memories)
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
		pw := msg.Width - 10
		if pw > 60 {
			pw = 60
		}
		if pw < 20 {
			pw = 20
		}
		m.progress.SetWidth(pw)
		return m, nil
	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case progress.FrameMsg:
		var cmd tea.Cmd
		m.progress, cmd = m.progress.Update(msg)
		return m, cmd
	}

	switch m.phase {
	case phaseLoadingProjects:
		return m.updateLoadingProjects(msg)
	case phaseProjects:
		return m.updateProjects(msg)
	case phaseLoadingMemories:
		return m.updateLoadingMemories(msg)
	case phaseMemories:
		return m.updateMemories(msg)
	case phaseViewMemory:
		return m.updateViewMemory(msg)
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

func (m Model) updateLoadingProjects(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case startLoadMsg:
		cmd := startLoadWithProgress(&m)
		return m, cmd

	case loadProgressMsg:
		m.progressPct = msg.Percent
		m.progressInfo = fmt.Sprintf("Scanning projects %d/%d...", msg.Current, msg.Total)
		progCmd := m.progress.SetPercent(msg.Percent)
		listenCmd := listenForLoadUpdates(m.progressCh, m.resultCh)
		return m, tea.Batch(progCmd, listenCmd)

	case projectsResultMsg:
		if msg.err != nil {
			m.status = "Error: " + msg.err.Error()
			m.phase = phaseProjects
			return m, nil
		}
		m.projects = msg.projects
		m.filteredProjs = allIndices(len(m.projects))
		m.projCursor = 0
		m.phase = phaseProjects
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) updateProjects(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.filtering {
		return m.updateFilterInput(msg, true)
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keys.Quit), key.Matches(msg, m.keys.Back):
			m.BackToHome = true
			return m, tea.Quit
		case key.Matches(msg, m.keys.Up):
			if m.projCursor > 0 {
				m.projCursor--
			}
		case key.Matches(msg, m.keys.Down):
			if m.projCursor < len(m.filteredProjs)-1 {
				m.projCursor++
			}
		case key.Matches(msg, m.keys.Top):
			m.projCursor = 0
		case key.Matches(msg, m.keys.Bottom):
			if len(m.filteredProjs) > 0 {
				m.projCursor = len(m.filteredProjs) - 1
			}
		case key.Matches(msg, m.keys.HalfUp):
			m.projCursor -= m.projPageSize() / 2
			if m.projCursor < 0 {
				m.projCursor = 0
			}
		case key.Matches(msg, m.keys.HalfDn):
			m.projCursor += m.projPageSize() / 2
			if m.projCursor >= len(m.filteredProjs) {
				m.projCursor = len(m.filteredProjs) - 1
			}
			if m.projCursor < 0 {
				m.projCursor = 0
			}
		case key.Matches(msg, m.keys.Open), key.Matches(msg, m.keys.Toggle):
			if len(m.filteredProjs) == 0 {
				return m, nil
			}
			idx := m.filteredProjs[m.projCursor]
			m.selectedProject = m.projects[idx]
			m.phase = phaseLoadingMemories
			m.filtering = false
			m.filter.SetValue("")
			return m, tea.Batch(m.spinner.Tick, loadMemoriesCmd(m.selectedProject.DirName))
		case key.Matches(msg, m.keys.Search):
			m.filtering = true
			m.filter.SetValue("")
			return m, m.filter.Focus()
		}
	}

	return m, nil
}

func (m Model) updateLoadingMemories(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case memoriesLoadedMsg:
		m.memories = []memory.Memory(msg)
		m.filteredMems = allIndices(len(m.memories))
		m.memCursor = 0
		m.selected = make(map[int]bool)
		m.phase = phaseMemories
		return m, nil

	case loadErrorMsg:
		m.status = "Error: " + msg.err.Error()
		m.phase = phaseProjects
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) updateMemories(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.filtering {
		return m.updateFilterInput(msg, false)
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.Back):
			m.memories = nil
			m.filteredMems = nil
			m.memCursor = 0
			m.selected = make(map[int]bool)
			m.filtering = false
			m.filter.SetValue("")
			m.phase = phaseLoadingProjects
			return m, func() tea.Msg { return startLoadMsg{} }
		case key.Matches(msg, m.keys.Up):
			if m.memCursor > 0 {
				m.memCursor--
			}
		case key.Matches(msg, m.keys.Down):
			if m.memCursor < len(m.filteredMems)-1 {
				m.memCursor++
			}
		case key.Matches(msg, m.keys.Top):
			m.memCursor = 0
		case key.Matches(msg, m.keys.Bottom):
			if len(m.filteredMems) > 0 {
				m.memCursor = len(m.filteredMems) - 1
			}
		case key.Matches(msg, m.keys.HalfUp):
			m.memCursor -= m.memPageSize() / 2
			if m.memCursor < 0 {
				m.memCursor = 0
			}
		case key.Matches(msg, m.keys.HalfDn):
			m.memCursor += m.memPageSize() / 2
			if m.memCursor >= len(m.filteredMems) {
				m.memCursor = len(m.filteredMems) - 1
			}
			if m.memCursor < 0 {
				m.memCursor = 0
			}
		case key.Matches(msg, m.keys.Open):
			if len(m.filteredMems) == 0 || len(m.selected) > 0 {
				return m, nil
			}
			idx := m.filteredMems[m.memCursor]
			m.viewingMemory = m.memories[idx]
			m.renderedContent = renderMarkdown(m.viewingMemory.Content, m.isDark, m.width)
			m.scrollOffset = 0
			m.phase = phaseViewMemory
			return m, nil
		case key.Matches(msg, m.keys.Toggle):
			if len(m.filteredMems) == 0 {
				return m, nil
			}
			memIdx := m.filteredMems[m.memCursor]
			if m.selected[memIdx] {
				delete(m.selected, memIdx)
			} else {
				m.selected[memIdx] = true
			}
			if m.memCursor < len(m.filteredMems)-1 {
				m.memCursor++
			}
			return m, nil
		case key.Matches(msg, m.keys.SelAll):
			for _, idx := range m.filteredMems {
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

func (m Model) updateViewMemory(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
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
			m.phase = phaseMemories
			return m, nil
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
			return m, tea.Batch(m.spinner.Tick, deleteMemoriesCmd(m.selectedMemories()))
		case key.Matches(msg, m.keys.No), key.Matches(msg, m.keys.Back):
			m.phase = phaseMemories
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
		m.deleteResults = []memory.DeleteResult(msg)
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
		case key.Matches(msg, m.keys.Open): // enter — back to memories
			// Remove successfully deleted memories from local state.
			deletedFiles := make(map[string]bool)
			for _, r := range m.deleteResults {
				if r.Success {
					deletedFiles[r.FileName] = true
				}
			}
			var remaining []memory.Memory
			for _, mem := range m.memories {
				if !deletedFiles[mem.FileName] {
					remaining = append(remaining, mem)
				}
			}
			m.memories = remaining
			m.filteredMems = allIndices(len(m.memories))
			m.selected = make(map[int]bool)
			if m.memCursor >= len(m.filteredMems) {
				m.memCursor = len(m.filteredMems) - 1
			}
			if m.memCursor < 0 {
				m.memCursor = 0
			}
			m.deleteResults = nil
			m.phase = phaseMemories
			return m, nil
		case key.Matches(msg, m.keys.Quit), key.Matches(msg, m.keys.Back):
			// Back to projects — reload.
			m.memories = nil
			m.filteredMems = nil
			m.memCursor = 0
			m.selected = make(map[int]bool)
			m.deleteResults = nil
			m.phase = phaseLoadingProjects
			return m, func() tea.Msg { return startLoadMsg{} }
		}
	}
	return m, nil
}

// --- Filter ---

func (m Model) updateFilterInput(msg tea.Msg, isProjects bool) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "enter":
			m.filtering = false
			m.filter.Blur()
			m.applyFilter(isProjects)
			return m, nil
		case "esc":
			m.filtering = false
			m.filter.Blur()
			m.filter.SetValue("")
			if isProjects {
				m.filteredProjs = allIndices(len(m.projects))
				m.projCursor = 0
			} else {
				m.filteredMems = allIndices(len(m.memories))
				m.memCursor = 0
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.filter, cmd = m.filter.Update(msg)
	m.applyFilter(isProjects)

	return m, cmd
}

func (m *Model) applyFilter(isProjects bool) {
	term := strings.ToLower(m.filter.Value())

	if isProjects {
		if term == "" {
			m.filteredProjs = allIndices(len(m.projects))
		} else {
			m.filteredProjs = m.filteredProjs[:0]
			for i, p := range m.projects {
				if strings.Contains(strings.ToLower(p.Path), term) {
					m.filteredProjs = append(m.filteredProjs, i)
				}
			}
		}
		if m.projCursor >= len(m.filteredProjs) {
			m.projCursor = len(m.filteredProjs) - 1
		}
		if m.projCursor < 0 {
			m.projCursor = 0
		}
	} else {
		if term == "" {
			m.filteredMems = allIndices(len(m.memories))
		} else {
			m.filteredMems = m.filteredMems[:0]
			for i, mem := range m.memories {
				searchable := strings.ToLower(
					mem.Name + " " + mem.Description + " " + mem.Type + " " + mem.FileName,
				)
				if strings.Contains(searchable, term) {
					m.filteredMems = append(m.filteredMems, i)
				}
			}
		}
		if m.memCursor >= len(m.filteredMems) {
			m.memCursor = len(m.filteredMems) - 1
		}
		if m.memCursor < 0 {
			m.memCursor = 0
		}
	}
}

// renderMarkdown renders markdown content using glamour.
// Falls back to raw content on error.
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
