package browse

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	Up       key.Binding
	Down     key.Binding
	Top      key.Binding
	Bottom   key.Binding
	Open     key.Binding
	Back     key.Binding
	Search   key.Binding
	Quit     key.Binding
	HalfUp   key.Binding
	HalfDn   key.Binding
	Rename   key.Binding
	Toggle   key.Binding
	SelAll   key.Binding
	DeselAll key.Binding
	Delete   key.Binding
	Yes      key.Binding
	No       key.Binding
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
		Top: key.NewBinding(
			key.WithKeys("g"),
			key.WithHelp("g", "top"),
		),
		Bottom: key.NewBinding(
			key.WithKeys("G"),
			key.WithHelp("G", "bottom"),
		),
		HalfUp: key.NewBinding(
			key.WithKeys("ctrl+u"),
			key.WithHelp("ctrl+u", "half page up"),
		),
		HalfDn: key.NewBinding(
			key.WithKeys("ctrl+d"),
			key.WithHelp("ctrl+d", "half page down"),
		),
		Open: key.NewBinding(
			key.WithKeys("enter", "l"),
			key.WithHelp("enter/l", "open"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc", "h", "backspace"),
			key.WithHelp("esc/h", "back"),
		),
		Search: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "filter"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Rename: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "rename"),
		),
		Toggle: key.NewBinding(
			key.WithKeys(" "),
			key.WithHelp("space", "toggle select"),
		),
		SelAll: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "select all"),
		),
		DeselAll: key.NewBinding(
			key.WithKeys("A"),
			key.WithHelp("A", "deselect all"),
		),
		Delete: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "delete selected"),
		),
		Yes: key.NewBinding(
			key.WithKeys("y"),
			key.WithHelp("y", "yes"),
		),
		No: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "no"),
		),
	}
}
