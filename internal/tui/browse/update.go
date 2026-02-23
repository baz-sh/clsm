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

// Messages for async operations.
type startLoadMsg struct{}
type startAllSessionsMsg struct{}

type projectsResultMsg struct {
	projects []session.Project
	err      error
}

type loadProgressMsg session.LoadProgress
type sessionsLoadedMsg []session.Session
type loadErrorMsg struct{ err error }
type renameResultMsg struct{ err error }

type allSessionsResultMsg struct {
	sessions []session.Session
	err      error
}

type searchResultMsg struct {
	sessions []session.Session
	err      error
}

type searchProgressMsg session.SearchProgress
type deleteResultMsg []session.DeleteResult

// --- Async command launchers ---

func startLoadWithProgress(m *Model) tea.Cmd {
	progressCh := make(chan session.LoadProgress, 10)
	resultCh := make(chan projectsResultMsg, 1)

	go func() {
		projects, err := session.ListProjectsWithProgress(progressCh)
		resultCh <- projectsResultMsg{projects: projects, err: err}
	}()

	m.progressCh = progressCh
	m.resultCh = resultCh

	return listenForLoadUpdates(m.progressCh, m.resultCh)
}

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

func startAllSessionsLoad(m *Model) tea.Cmd {
	progressCh := make(chan session.LoadProgress, 10)
	resultCh := make(chan allSessionsResultMsg, 1)

	go func() {
		sessions, err := session.ListAllSessionsWithProgress(progressCh)
		resultCh <- allSessionsResultMsg{sessions: sessions, err: err}
	}()

	m.progressCh = progressCh
	m.allSessResultCh = resultCh

	return listenForAllSessUpdates(m.progressCh, m.allSessResultCh)
}

func listenForAllSessUpdates(progressCh <-chan session.LoadProgress, resultCh <-chan allSessionsResultMsg) tea.Cmd {
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

func startSearchCmd(m *Model, term string) tea.Cmd {
	progressCh := make(chan session.SearchProgress, 10)
	resultCh := make(chan searchResultMsg, 1)

	go func() {
		sessions, err := session.SearchWithProgress(term, progressCh)
		resultCh <- searchResultMsg{sessions: sessions, err: err}
	}()

	m.searchProgressCh = progressCh
	m.searchResultCh = resultCh

	return listenForSearchUpdates(m.searchProgressCh, m.searchResultCh)
}

func listenForSearchUpdates(progressCh <-chan session.SearchProgress, resultCh <-chan searchResultMsg) tea.Cmd {
	return func() tea.Msg {
		select {
		case p, ok := <-progressCh:
			if !ok {
				return <-resultCh
			}
			return searchProgressMsg(p)
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

func renameCmd(s session.Session, newTitle string) tea.Cmd {
	return func() tea.Msg {
		err := session.Rename(s, newTitle)
		return renameResultMsg{err: err}
	}
}

func deleteSessCmd(sessions []session.Session) tea.Cmd {
	return func() tea.Msg {
		results := session.Delete(sessions)
		return deleteResultMsg(results)
	}
}

// --- Main Update ---

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
	case phaseLoadingAllSessions:
		return m.updateLoadingAllSessions(msg)
	case phaseSearchInput:
		return m.updateSearchInput(msg)
	case phaseSearching:
		return m.updateSearching(msg)
	case phaseSessions:
		return m.updateSessions(msg)
	case phaseRename:
		return m.updateRename(msg)
	case phaseConfirmDelete:
		return m.updateConfirmDelete(msg)
	case phaseDeleting:
		return m.updateDeleting(msg)
	case phaseDeleteResults:
		return m.updateDeleteResults(msg)
	case phasePruneLoading:
		return m.updatePruneLoading(msg)
	case phasePrunePreview:
		return m.updatePrunePreview(msg)
	case phasePruning:
		return m.updatePruning(msg)
	case phasePruneResults:
		return m.updatePruneResults(msg)
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
			// Both enter/l and space open a project.
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
		m.selected = make(map[int]bool)
		m.sessionSource = "project"
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

func (m Model) updateLoadingAllSessions(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case startAllSessionsMsg:
		cmd := startAllSessionsLoad(&m)
		return m, cmd

	case loadProgressMsg:
		m.progressPct = msg.Percent
		m.progressInfo = fmt.Sprintf("Loading sessions %d/%d...", msg.Current, msg.Total)
		progCmd := m.progress.SetPercent(msg.Percent)
		listenCmd := listenForAllSessUpdates(m.progressCh, m.allSessResultCh)
		return m, tea.Batch(progCmd, listenCmd)

	case allSessionsResultMsg:
		if msg.err != nil {
			m.status = "Error: " + msg.err.Error()
			m.BackToHome = true
			return m, tea.Quit
		}
		m.sessions = make([]sessionItem, len(msg.sessions))
		for i, s := range msg.sessions {
			m.sessions[i] = sessionItem{session: s}
		}
		m.filteredSess = allIndices(len(m.sessions))
		m.sessCursor = 0
		m.selected = make(map[int]bool)
		m.sessionSource = "all"
		m.phase = phaseSessions
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) updateSearchInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			m.BackToHome = true
			return m, tea.Quit
		case tea.KeyEsc:
			m.BackToHome = true
			return m, tea.Quit
		case tea.KeyEnter:
			term := strings.TrimSpace(m.searchInput.Value())
			if term == "" {
				m.status = "Please enter a search term."
				return m, nil
			}
			m.phase = phaseSearching
			m.status = ""
			m.searchTerm = term
			m.progressPct = 0
			m.progressInfo = "Starting search..."
			cmd := startSearchCmd(&m, term)
			return m, cmd
		}
	}

	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)
	return m, cmd
}

func (m Model) updateSearching(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case searchProgressMsg:
		m.progressPct = msg.Percent
		switch msg.Phase {
		case "indexes":
			m.progressInfo = fmt.Sprintf("Scanning indexes %d/%d...", msg.Current, msg.Total)
		case "sessions":
			m.progressInfo = fmt.Sprintf("Scanning sessions %d/%d...", msg.Current, msg.Total)
		}
		progCmd := m.progress.SetPercent(msg.Percent)
		listenCmd := listenForSearchUpdates(m.searchProgressCh, m.searchResultCh)
		return m, tea.Batch(progCmd, listenCmd)

	case searchResultMsg:
		if msg.err != nil {
			m.phase = phaseSearchInput
			m.status = "Search error: " + msg.err.Error()
			m.searchInput.Focus()
			return m, nil
		}
		if len(msg.sessions) == 0 {
			m.phase = phaseSearchInput
			m.status = "No sessions found. Try a different search term."
			m.searchInput.Focus()
			return m, nil
		}
		m.sessions = make([]sessionItem, len(msg.sessions))
		for i, s := range msg.sessions {
			m.sessions[i] = sessionItem{session: s}
		}
		m.filteredSess = allIndices(len(m.sessions))
		m.sessCursor = 0
		m.selected = make(map[int]bool)
		m.sessionSource = "search"
		m.phase = phaseSessions
		return m, nil
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
			switch m.sessionSource {
			case "project":
				m.sessions = nil
				m.filteredSess = nil
				m.sessCursor = 0
				m.selected = make(map[int]bool)
				m.filtering = false
				m.filter.SetValue("")
				m.phase = phaseLoadingProjects
				return m, func() tea.Msg { return startLoadMsg{} }
			case "all":
				m.BackToHome = true
				return m, tea.Quit
			case "search":
				m.phase = phaseSearchInput
				m.searchInput.Focus()
				m.sessions = nil
				m.filteredSess = nil
				m.sessCursor = 0
				m.selected = make(map[int]bool)
				return m, nil
			}
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
			m.sessCursor -= m.sessPageSize() / 2
			if m.sessCursor < 0 {
				m.sessCursor = 0
			}
		case key.Matches(msg, m.keys.HalfDn):
			m.sessCursor += m.sessPageSize() / 2
			if m.sessCursor >= len(m.filteredSess) {
				m.sessCursor = len(m.filteredSess) - 1
			}
			if m.sessCursor < 0 {
				m.sessCursor = 0
			}
		case key.Matches(msg, m.keys.Toggle):
			if len(m.filteredSess) == 0 {
				return m, nil
			}
			sessIdx := m.filteredSess[m.sessCursor]
			if m.selected[sessIdx] {
				delete(m.selected, sessIdx)
			} else {
				m.selected[sessIdx] = true
			}
			// Auto-advance cursor.
			if m.sessCursor < len(m.filteredSess)-1 {
				m.sessCursor++
			}
			return m, nil
		case key.Matches(msg, m.keys.SelAll):
			for _, idx := range m.filteredSess {
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
			m.filter.Focus()
			return m, nil
		case key.Matches(msg, m.keys.Rename):
			// Rename only works when nothing is selected.
			if len(m.filteredSess) == 0 || len(m.selected) > 0 {
				return m, nil
			}
			idx := m.filteredSess[m.sessCursor]
			m.renameIdx = idx
			m.renameInput.SetValue("")
			m.renameInput.Focus()
			m.status = ""
			m.phase = phaseRename
			return m, nil
		}
	}

	return m, nil
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

func (m Model) updateConfirmDelete(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Yes):
			m.phase = phaseDeleting
			return m, tea.Batch(m.spinner.Tick, deleteSessCmd(m.selectedSessions()))
		case key.Matches(msg, m.keys.No), key.Matches(msg, m.keys.Back):
			m.phase = phaseSessions
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
		m.deleteResults = []session.DeleteResult(msg)
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
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Open): // enter — back to sessions
			// Remove successfully deleted sessions from local state.
			deletedIDs := make(map[string]bool)
			for _, r := range m.deleteResults {
				if r.Success {
					deletedIDs[r.SessionID] = true
				}
			}
			var remaining []sessionItem
			for _, item := range m.sessions {
				if !deletedIDs[item.session.SessionID] {
					remaining = append(remaining, item)
				}
			}
			m.sessions = remaining
			m.filteredSess = allIndices(len(m.sessions))
			m.selected = make(map[int]bool)
			if m.sessCursor >= len(m.filteredSess) {
				m.sessCursor = len(m.filteredSess) - 1
			}
			if m.sessCursor < 0 {
				m.sessCursor = 0
			}
			m.deleteResults = nil
			m.phase = phaseSessions
			return m, nil
		case key.Matches(msg, m.keys.Quit), key.Matches(msg, m.keys.Back):
			switch m.sessionSource {
			case "project":
				m.sessions = nil
				m.filteredSess = nil
				m.sessCursor = 0
				m.selected = make(map[int]bool)
				m.deleteResults = nil
				m.phase = phaseLoadingProjects
				return m, func() tea.Msg { return startLoadMsg{} }
			case "search":
				m.phase = phaseSearchInput
				m.searchInput.Focus()
				m.sessions = nil
				m.filteredSess = nil
				m.sessCursor = 0
				m.selected = make(map[int]bool)
				m.deleteResults = nil
				return m, nil
			default:
				m.BackToHome = true
				return m, tea.Quit
			}
		}
	}
	return m, nil
}

// --- Prune phase handlers ---

func (m Model) updatePruneLoading(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case startAllSessionsMsg:
		cmd := startAllSessionsLoad(&m)
		return m, cmd

	case loadProgressMsg:
		m.progressPct = msg.Percent
		m.progressInfo = fmt.Sprintf("Loading sessions %d/%d...", msg.Current, msg.Total)
		progCmd := m.progress.SetPercent(msg.Percent)
		listenCmd := listenForAllSessUpdates(m.progressCh, m.allSessResultCh)
		return m, tea.Batch(progCmd, listenCmd)

	case allSessionsResultMsg:
		if msg.err != nil {
			m.status = "Error: " + msg.err.Error()
			m.BackToHome = true
			return m, tea.Quit
		}
		// Filter to 0-message sessions.
		var empty []session.Session
		for _, s := range msg.sessions {
			if s.MsgCount == 0 {
				empty = append(empty, s)
			}
		}
		m.pruneSessions = empty
		m.phase = phasePrunePreview
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) updatePrunePreview(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if len(m.pruneSessions) == 0 {
			// No sessions to prune — any key goes back to home.
			m.BackToHome = true
			return m, tea.Quit
		}
		switch {
		case key.Matches(msg, m.keys.Yes):
			m.phase = phasePruning
			return m, tea.Batch(m.spinner.Tick, deleteSessCmd(m.pruneSessions))
		case key.Matches(msg, m.keys.No), key.Matches(msg, m.keys.Back), key.Matches(msg, m.keys.Quit):
			m.BackToHome = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m Model) updatePruning(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case deleteResultMsg:
		m.deleteResults = []session.DeleteResult(msg)
		m.phase = phasePruneResults
		return m, nil
	default:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
}

func (m Model) updatePruneResults(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case tea.KeyMsg:
		m.BackToHome = true
		return m, tea.Quit
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
