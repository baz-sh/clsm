package browse

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/baz-sh/clsm/internal/session"
)

// startLoadMsg is sent by Init to kick off project loading from Update,
// where the model is returned and channel refs are preserved.
type startLoadMsg struct{}

// Messages for async operations.
type projectsResultMsg struct {
	projects []session.Project
	err      error
}
type loadProgressMsg session.LoadProgress
type sessionsLoadedMsg []session.Session
type loadErrorMsg struct{ err error }
type renameResultMsg struct{ err error }

// startLoadWithProgress launches the project loading goroutine and stores
// the channels on the model. Returns the initial listener command.
func startLoadWithProgress(m *Model) tea.Cmd {
	progressCh := make(chan session.LoadProgress, 10)
	resultCh := make(chan projectsResultMsg, 1)

	go func() {
		projects, err := session.ListProjectsWithProgress(progressCh)
		resultCh <- projectsResultMsg{projects: projects, err: err}
	}()

	m.progressCh = progressCh
	m.resultCh = resultCh

	return listenForLoadUpdates(progressCh, resultCh)
}

// listenForLoadUpdates returns a tea.Cmd that waits for either a progress
// update or the final load result.
func listenForLoadUpdates(progressCh <-chan session.LoadProgress, resultCh <-chan projectsResultMsg) tea.Cmd {
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
		m.progress.Width = msg.Width - 10
		if m.progress.Width > 60 {
			m.progress.Width = 60
		}
		if m.progress.Width < 20 {
			m.progress.Width = 20
		}
		return m, nil
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}
	case progress.FrameMsg:
		progressModel, cmd := m.progress.Update(msg)
		m.progress = progressModel.(progress.Model)
		return m, cmd
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
	case phaseRename:
		return m.updateRename(msg)
	}

	return m, nil
}

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
		m.projects = make([]projectItem, len(msg.projects))
		for i, p := range msg.projects {
			m.projects[i] = projectItem{project: p}
		}
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
	// If filtering, route to text input first.
	if m.filtering {
		return m.updateFilterInput(msg, true)
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
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
			m.projCursor -= pageSize / 2
			if m.projCursor < 0 {
				m.projCursor = 0
			}
		case key.Matches(msg, m.keys.HalfDn):
			m.projCursor += pageSize / 2
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
		case key.Matches(msg, m.keys.Quit), key.Matches(msg, m.keys.Back):
			m.phase = phaseProjects
			m.sessions = nil
			m.filteredSess = nil
			m.sessCursor = 0
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
			m.sessCursor -= pageSize / 2
			if m.sessCursor < 0 {
				m.sessCursor = 0
			}
		case key.Matches(msg, m.keys.HalfDn):
			m.sessCursor += pageSize / 2
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
		case key.Matches(msg, m.keys.Rename):
			if len(m.filteredSess) == 0 {
				return m, nil
			}
			idx := m.filteredSess[m.sessCursor]
			s := m.sessions[idx].session
			m.renameIdx = idx
			m.renameInput.SetValue(displayTitle(s))
			m.renameInput.Focus()
			m.renameInput.CursorEnd()
			m.status = ""
			m.phase = phaseRename
			return m, nil
		}
	}

	return m, nil
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

func renameCmd(s session.Session, newTitle string) tea.Cmd {
	return func() tea.Msg {
		err := session.Rename(s, newTitle)
		return renameResultMsg{err: err}
	}
}

func (m Model) updateRename(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			newTitle := strings.TrimSpace(m.renameInput.Value())
			if newTitle == "" {
				m.status = "Title cannot be empty."
				return m, nil
			}
			m.renameInput.Blur()
			return m, renameCmd(m.sessions[m.renameIdx].session, newTitle)
		case tea.KeyEsc:
			m.renameInput.Blur()
			m.status = ""
			m.phase = phaseSessions
			return m, nil
		}

	case renameResultMsg:
		if msg.err != nil {
			m.status = "Rename failed: " + msg.err.Error()
			m.phase = phaseSessions
			return m, nil
		}
		// Update the session in our local state.
		s := m.sessions[m.renameIdx].session
		s.CustomTitle = strings.TrimSpace(m.renameInput.Value())
		m.sessions[m.renameIdx] = sessionItem{session: s}
		m.status = ""
		m.phase = phaseSessions
		return m, nil
	}

	var cmd tea.Cmd
	m.renameInput, cmd = m.renameInput.Update(msg)
	return m, cmd
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
