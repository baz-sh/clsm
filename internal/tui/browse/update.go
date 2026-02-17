package browse

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/baz-sh/clsm/internal/session"
)

// Messages for async operations.
type projectsLoadedMsg []session.Project
type sessionsLoadedMsg []session.Session
type loadErrorMsg struct{ err error }

func loadProjectsCmd() tea.Cmd {
	return func() tea.Msg {
		projects, err := session.ListProjects()
		if err != nil {
			return loadErrorMsg{err}
		}
		return projectsLoadedMsg(projects)
	}
}

func loadSessionsCmd(projectDir string) tea.Cmd {
	return func() tea.Msg {
		sessions, err := session.ListSessions(projectDir)
		if err != nil {
			return loadErrorMsg{err}
		}
		return sessionsLoadedMsg(sessions)
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}
	}

	switch m.phase {
	case phaseLoadingProjects:
		return m.updateLoadingProjects(msg)
	case phaseProjects:
		return m.updateProjects(msg)
	case phaseLoadingSessions:
		return m.updateLoadingSessions(msg)
	case phaseSessions:
		return m.updateSessions(msg)
	}

	return m, nil
}

func (m Model) updateLoadingProjects(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case projectsLoadedMsg:
		m.projects = make([]projectItem, len(msg))
		for i, p := range msg {
			m.projects[i] = projectItem{project: p}
		}
		m.filteredProjs = allIndices(len(m.projects))
		m.projCursor = 0
		m.projOffset = 0
		m.phase = phaseProjects
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

func (m Model) updateProjects(msg tea.Msg) (tea.Model, tea.Cmd) {
	// If filtering, route to text input first.
	if m.filtering {
		return m.updateFilterInput(msg, true)
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
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
			m.projCursor -= m.visibleHeight() / 4
			if m.projCursor < 0 {
				m.projCursor = 0
			}
		case key.Matches(msg, m.keys.HalfDn):
			m.projCursor += m.visibleHeight() / 4
			if m.projCursor >= len(m.filteredProjs) {
				m.projCursor = len(m.filteredProjs) - 1
			}
			if m.projCursor < 0 {
				m.projCursor = 0
			}
		case key.Matches(msg, m.keys.Open):
			if len(m.filteredProjs) == 0 {
				return m, nil
			}
			idx := m.filteredProjs[m.projCursor]
			m.selectedProject = m.projects[idx].project
			m.phase = phaseLoadingSessions
			m.filtering = false
			m.filter.SetValue("")
			return m, tea.Batch(m.spinner.Tick, loadSessionsCmd(m.selectedProject.DirName))
		case key.Matches(msg, m.keys.Search):
			m.filtering = true
			m.filter.SetValue("")
			m.filter.Focus()
			return m, nil
		}
	}

	return m, nil
}

func (m Model) updateLoadingSessions(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case sessionsLoadedMsg:
		m.sessions = make([]sessionItem, len(msg))
		for i, s := range msg {
			m.sessions[i] = sessionItem{session: s}
		}
		m.filteredSess = allIndices(len(m.sessions))
		m.sessCursor = 0
		m.sessOffset = 0
		m.phase = phaseSessions
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

func (m Model) updateSessions(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.filtering {
		return m.updateFilterInput(msg, false)
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.Back):
			m.phase = phaseProjects
			m.sessions = nil
			m.filteredSess = nil
			m.sessCursor = 0
			m.sessOffset = 0
			m.filtering = false
			m.filter.SetValue("")
			return m, nil
		case key.Matches(msg, m.keys.Up):
			if m.sessCursor > 0 {
				m.sessCursor--
			}
		case key.Matches(msg, m.keys.Down):
			if m.sessCursor < len(m.filteredSess)-1 {
				m.sessCursor++
			}
		case key.Matches(msg, m.keys.Top):
			m.sessCursor = 0
		case key.Matches(msg, m.keys.Bottom):
			if len(m.filteredSess) > 0 {
				m.sessCursor = len(m.filteredSess) - 1
			}
		case key.Matches(msg, m.keys.HalfUp):
			m.sessCursor -= m.visibleHeight() / 6
			if m.sessCursor < 0 {
				m.sessCursor = 0
			}
		case key.Matches(msg, m.keys.HalfDn):
			m.sessCursor += m.visibleHeight() / 6
			if m.sessCursor >= len(m.filteredSess) {
				m.sessCursor = len(m.filteredSess) - 1
			}
			if m.sessCursor < 0 {
				m.sessCursor = 0
			}
		case key.Matches(msg, m.keys.Search):
			m.filtering = true
			m.filter.SetValue("")
			m.filter.Focus()
			return m, nil
		}
	}

	return m, nil
}

// updateFilterInput handles key input while the filter text input is focused.
func (m Model) updateFilterInput(msg tea.Msg, isProjects bool) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			m.filtering = false
			m.filter.Blur()
			m.applyFilter(isProjects)
			return m, nil
		case tea.KeyEsc:
			m.filtering = false
			m.filter.Blur()
			m.filter.SetValue("")
			// Reset filter — show all items.
			if isProjects {
				m.filteredProjs = allIndices(len(m.projects))
				m.projCursor = 0
			} else {
				m.filteredSess = allIndices(len(m.sessions))
				m.sessCursor = 0
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.filter, cmd = m.filter.Update(msg)

	// Live filter as user types.
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
				if strings.Contains(strings.ToLower(p.project.Path), term) {
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
			m.filteredSess = allIndices(len(m.sessions))
		} else {
			m.filteredSess = m.filteredSess[:0]
			for i, s := range m.sessions {
				searchable := strings.ToLower(
					s.session.Summary + " " + s.session.CustomTitle + " " + s.session.FirstPrompt,
				)
				if strings.Contains(searchable, term) {
					m.filteredSess = append(m.filteredSess, i)
				}
			}
		}
		if m.sessCursor >= len(m.filteredSess) {
			m.sessCursor = len(m.filteredSess) - 1
		}
		if m.sessCursor < 0 {
			m.sessCursor = 0
		}
	}
}

func allIndices(n int) []int {
	idx := make([]int, n)
	for i := range idx {
		idx[i] = i
	}
	return idx
}
