package home

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/baz-sh/clsm/internal/tui/theme"
)

type Choice string

const (
	ChoiceProjects Choice = "projects"
	ChoiceSessions Choice = "sessions"
	ChoiceSearch   Choice = "search"
	ChoicePrune    Choice = "prune"
	ChoiceNone     Choice = ""
)

type option struct {
	choice Choice
	label  string
	desc   string
}

var options = []option{
	{ChoiceProjects, "Projects", "Browse projects and their sessions"},
	{ChoiceSessions, "Sessions", "Browse all sessions"},
	{ChoiceSearch, "Search", "Search across all sessions"},
	{ChoicePrune, "Prune", "Delete sessions with no messages"},
}

type keyMap struct {
	Up     key.Binding
	Down   key.Binding
	Select key.Binding
	Quit   key.Binding
}

func newKeyMap() keyMap {
	return keyMap{
		Up: key.NewBinding(
			key.WithKeys("k", "up"),
			key.WithHelp("k/↑", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("j", "down"),
			key.WithHelp("j/↓", "down"),
		),
		Select: key.NewBinding(
			key.WithKeys("enter", "l"),
			key.WithHelp("enter/l", "select"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "esc", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
	}
}

type Model struct {
	cursor int
	keys   keyMap
	Result Choice
	isDark bool
	theme  theme.Theme
}

func New() Model {
	return Model{
		keys:  newKeyMap(),
		theme: theme.New(true),
	}
}

func (m Model) Init() tea.Cmd { return tea.RequestBackgroundColor }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.BackgroundColorMsg:
		m.isDark = msg.IsDark()
		m.theme = theme.New(m.isDark)
		return m, nil
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.Up):
			if m.cursor > 0 {
				m.cursor--
			}
		case key.Matches(msg, m.keys.Down):
			if m.cursor < len(options)-1 {
				m.cursor++
			}
		case key.Matches(msg, m.keys.Select):
			m.Result = options[m.cursor].choice
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m Model) View() tea.View {
	var b strings.Builder
	b.WriteString(m.theme.Title.Render("clsm — Claude Session Manager"))
	b.WriteString("\n\n")

	for i, opt := range options {
		prefix := "  "
		label := opt.label
		if i == m.cursor {
			prefix = m.theme.Cursor.Render("> ")
			label = m.theme.Cursor.Render(label)
		}
		desc := m.theme.Dim.Render(fmt.Sprintf("  %s", opt.desc))
		b.WriteString(fmt.Sprintf("%s%s%s\n", prefix, label, desc))
	}

	b.WriteString("\n")
	b.WriteString(m.theme.Help.Render("j/k: navigate • enter/l: select • q/esc: quit"))

	v := tea.NewView(b.String())
	v.AltScreen = true
	return v
}
