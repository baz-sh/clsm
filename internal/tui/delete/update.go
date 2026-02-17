package delete

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/baz-sh/clsm/internal/session"
)

// Custom messages for async operations.
type searchResultMsg []session.Session
type searchErrorMsg error
type deleteResultMsg []session.DeleteResult
type deleteErrorMsg error

// searchCmd returns a tea.Cmd that searches for sessions.
func searchCmd(term string) tea.Cmd {
	return func() tea.Msg {
		results, err := session.Search(term)
		if err != nil {
			return searchErrorMsg(err)
		}
		return searchResultMsg(results)
	}
}

// deleteCmd returns a tea.Cmd that deletes sessions.
func deleteCmd(sessions []session.Session) tea.Cmd {
	return func() tea.Msg {
		results := session.Delete(sessions)
		return deleteResultMsg(results)
	}
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		// Global quit on ctrl+c.
		if msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}
	}

	switch m.phase {
	case phaseSearch:
		return m.updateSearch(msg)
	case phaseLoading:
		return m.updateLoading(msg)
	case phaseSelect:
		return m.updateSelect(msg)
	case phaseConfirm:
		return m.updateConfirm(msg)
	case phaseDeleting:
		return m.updateDeleting(msg)
	case phaseResults:
		return m.updateResults(msg)
	}

	return m, nil
}

func (m Model) updateSearch(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			if msg.String() == "q" && m.input.Focused() {
				// Let 'q' pass through to the text input when focused.
				break
			}
			m.BackToHome = true
			return m, tea.Quit
		case key.Matches(msg, m.keys.Back):
			m.BackToHome = true
			return m, tea.Quit
		case key.Matches(msg, m.keys.Confirm):
			term := m.input.Value()
			if term == "" {
				m.status = "Please enter a search term."
				return m, nil
			}
			m.phase = phaseLoading
			m.status = ""
			m.searchTerm = term
			return m, tea.Batch(m.spinner.Tick, searchCmd(term))
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m Model) updateLoading(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case searchResultMsg:
		if len(msg) == 0 {
			m.phase = phaseSearch
			m.status = "No sessions found. Try a different search term."
			m.input.Focus()
			return m, nil
		}
		m.items = make([]sessionItem, len(msg))
		for i, s := range msg {
			m.items[i] = sessionItem{session: s}
		}
		m.cursor = 0
		m.offset = 0
		m.phase = phaseSelect
		return m, nil

	case searchErrorMsg:
		m.phase = phaseSearch
		m.status = "Search error: " + msg.Error()
		m.input.Focus()
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) updateSelect(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.Back):
			m.phase = phaseSearch
			m.input.Focus()
			m.items = nil
			m.cursor = 0
			m.offset = 0
			return m, nil
		case key.Matches(msg, m.keys.Search):
			m.phase = phaseSearch
			m.input.Focus()
			m.items = nil
			m.cursor = 0
			m.offset = 0
			return m, nil
		case key.Matches(msg, m.keys.Up):
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case key.Matches(msg, m.keys.Down):
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
			return m, nil
		case key.Matches(msg, m.keys.Top):
			m.cursor = 0
			return m, nil
		case key.Matches(msg, m.keys.Bottom):
			m.cursor = len(m.items) - 1
			return m, nil
		case key.Matches(msg, m.keys.Select):
			m.items[m.cursor].selected = !m.items[m.cursor].selected
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
			return m, nil
		case key.Matches(msg, m.keys.SelAll):
			for i := range m.items {
				m.items[i].selected = true
			}
			return m, nil
		case key.Matches(msg, m.keys.DeselAll):
			for i := range m.items {
				m.items[i].selected = false
			}
			return m, nil
		case key.Matches(msg, m.keys.Delete), key.Matches(msg, m.keys.Confirm):
			if m.countSelected() == 0 {
				return m, nil
			}
			m.phase = phaseConfirm
			return m, nil
		}
	}

	return m, nil
}

func (m Model) updateConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Yes):
			m.phase = phaseDeleting
			return m, tea.Batch(m.spinner.Tick, deleteCmd(m.selectedSessions()))
		case key.Matches(msg, m.keys.No), key.Matches(msg, m.keys.Back):
			m.phase = phaseSelect
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
		m.results = msg
		m.phase = phaseResults
		return m, nil

	case deleteErrorMsg:
		m.results = []session.DeleteResult{{
			Success: false,
			Error:   msg.Error(),
		}}
		m.phase = phaseResults
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) updateResults(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.Confirm), key.Matches(msg, m.keys.Search):
			m.phase = phaseSearch
			m.input.SetValue("")
			m.input.Focus()
			m.items = nil
			m.cursor = 0
			m.offset = 0
			m.results = nil
			m.status = ""
			m.searchTerm = ""
			return m, nil
		}
	}

	return m, nil
}
