package tui

import (
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
)

type keyMap struct {
	Quit       key.Binding
	SwitchPane key.Binding
	Enter      key.Binding
	Up         key.Binding
	Down       key.Binding
	Left       key.Binding
	Right      key.Binding
	ShiftUp    key.Binding
	ShiftDown  key.Binding
	ShiftLeft  key.Binding
	ShiftRight key.Binding
	Comment    key.Binding
	ClearAll   key.Binding
}

func newKeyMap() keyMap {
	return keyMap{
		Quit: key.NewBinding(
			key.WithKeys("esc", "ctrl+c"),
			key.WithHelp("Esc", "quit"),
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
		ShiftUp: key.NewBinding(
			key.WithKeys("shift+up"),
			key.WithHelp("Shift+↑", "select up"),
		),
		ShiftDown: key.NewBinding(
			key.WithKeys("shift+down"),
			key.WithHelp("Shift+↓", "select down"),
		),
		ShiftLeft: key.NewBinding(
			key.WithKeys("shift+left"),
			key.WithHelp("Shift+←", "select left"),
		),
		ShiftRight: key.NewBinding(
			key.WithKeys("shift+right"),
			key.WithHelp("Shift+→", "select right"),
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
		k.Comment, k.ClearAll, k.Quit,
	}
}

// FullHelp returns key bindings for the full help view.
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Left, k.Right},
		{k.ShiftUp, k.ShiftDown, k.ShiftLeft, k.ShiftRight},
		{k.Enter, k.SwitchPane, k.Comment, k.ClearAll, k.Quit},
	}
}

// contextKeyMap returns a help.KeyMap with bindings enabled/disabled
// based on the current TUI state.
func (m Model) contextKeyMap() help.KeyMap {
	km := m.keys
	km.Comment.SetEnabled(
		m.focusPane == 1 && m.previewLines == nil,
	)
	km.ClearAll.SetEnabled(
		m.focusPane == 1 && m.previewLines == nil,
	)
	return km
}
