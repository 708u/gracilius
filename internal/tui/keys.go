package tui

import (
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
)

type keyMap struct {
	Quit       key.Binding
	Cancel     key.Binding
	SwitchPane key.Binding
	Enter      key.Binding
	Up         key.Binding
	Down       key.Binding
	Left       key.Binding
	Right      key.Binding
	CharSelect key.Binding
	LineSelect key.Binding
	Comment    key.Binding
	ClearAll   key.Binding
}

func newKeyMap() keyMap {
	return keyMap{
		Quit: key.NewBinding(
			key.WithKeys("ctrl+c"),
			key.WithHelp("Ctrl+C×2", "quit"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("Esc", "cancel"),
		),
		SwitchPane: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("Tab", "switch pane"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("Enter", "open/toggle"),
		),
		Up: key.NewBinding(
			key.WithKeys("up"),
			key.WithHelp("↑", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down"),
			key.WithHelp("↓", "down"),
		),
		Left: key.NewBinding(
			key.WithKeys("left"),
			key.WithHelp("←", "left"),
		),
		Right: key.NewBinding(
			key.WithKeys("right"),
			key.WithHelp("→", "right"),
		),
		CharSelect: key.NewBinding(
			key.WithKeys("v"),
			key.WithHelp("v", "select"),
		),
		LineSelect: key.NewBinding(
			key.WithKeys("V"),
			key.WithHelp("V", "select line"),
		),
		Comment: key.NewBinding(
			key.WithKeys("i"),
			key.WithHelp("i", "comment"),
		),
		ClearAll: key.NewBinding(
			key.WithKeys("D"),
			key.WithHelp("D", "clear comments"),
		),
	}
}

// ShortHelp returns key bindings for the short help view.
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		k.SwitchPane, k.Up, k.Down,
		k.CharSelect, k.LineSelect, k.Comment, k.ClearAll, k.Cancel, k.Quit,
	}
}

// FullHelp returns key bindings for the full help view.
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Left, k.Right},
		{k.Enter, k.SwitchPane, k.CharSelect, k.LineSelect, k.Comment, k.ClearAll, k.Cancel, k.Quit},
	}
}

// contextKeyMap returns a help.KeyMap with bindings enabled/disabled
// based on the current TUI state.
func (m *Model) contextKeyMap() help.KeyMap {
	km := m.keys
	km.CharSelect.SetEnabled(m.focusPane == paneEditor)
	km.LineSelect.SetEnabled(m.focusPane == paneEditor)
	km.Comment.SetEnabled(m.focusPane == paneEditor)
	km.ClearAll.SetEnabled(m.focusPane == paneEditor)
	return km
}
