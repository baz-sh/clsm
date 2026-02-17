package home

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/baz-sh/clsm/internal/tui/theme"
)

type Choice string

const (
	ChoiceBrowse Choice = "browse"
	ChoiceDelete Choice = "delete"
	ChoiceNone   Choice = ""
)

type option struct {
	choice Choice
	label  string
	desc   string
}

var options = []option{
	{ChoiceBrowse, "Browse", "Browse projects and sessions"},
	{ChoiceDelete, "Delete", "Search and delete sessions"},
}

type keyMap struct {
	Up      key.Binding
	Down    key.Binding
	Select  key.Binding
	Quit    key.Binding
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
}


func New() Model {
	return Model{keys: newKeyMap()}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
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

func (m Model) View() string {
	var b strings.Builder
	b.WriteString(theme.Title.Render("clsm — Claude Session Manager"))
	b.WriteString("\n\n")

	for i, opt := range options {
		prefix := "  "
		label := opt.label
		if i == m.cursor {
			prefix = theme.Cursor.Render("> ")
			label = theme.Cursor.Render(label)
		}
		desc := theme.Dim.Render(fmt.Sprintf("  %s", opt.desc))
		b.WriteString(fmt.Sprintf("%s%s%s\n", prefix, label, desc))
	}

	b.WriteString("\n")
	b.WriteString(theme.Help.Render("j/k: navigate • enter: select • q: quit"))
	return b.String()
}
