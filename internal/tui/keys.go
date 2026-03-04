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
	Copy       key.Binding
	Comment    key.Binding
	ClearAll   key.Binding
	GoTop      key.Binding
	GoBottom   key.Binding
	BlockUp    key.Binding
	BlockDown  key.Binding
	NextTab    key.Binding
	PrevTab    key.Binding
	CloseTab   key.Binding
	Search     key.Binding
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
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Left: key.NewBinding(
			key.WithKeys("left", "h"),
			key.WithHelp("←/h", "left"),
		),
		Right: key.NewBinding(
			key.WithKeys("right", "l"),
			key.WithHelp("→/l", "right"),
		),
		CharSelect: key.NewBinding(
			key.WithKeys("v"),
			key.WithHelp("v", "select"),
		),
		LineSelect: key.NewBinding(
			key.WithKeys("V"),
			key.WithHelp("V", "select line"),
		),
		Copy: key.NewBinding(
			key.WithKeys("y"),
			key.WithHelp("y", "copy"),
		),
		Comment: key.NewBinding(
			key.WithKeys("i"),
			key.WithHelp("i", "comment"),
		),
		ClearAll: key.NewBinding(
			key.WithKeys("D"),
			key.WithHelp("D", "clear comments"),
		),
		GoTop: key.NewBinding(
			key.WithKeys("g"),
			key.WithHelp("gg", "go top"),
		),
		GoBottom: key.NewBinding(
			key.WithKeys("G"),
			key.WithHelp("G", "go bottom"),
		),
		BlockUp: key.NewBinding(
			key.WithKeys("{"),
			key.WithHelp("{", "block up"),
		),
		BlockDown: key.NewBinding(
			key.WithKeys("}"),
			key.WithHelp("}", "block down"),
		),
		NextTab: key.NewBinding(
			key.WithKeys("L"),
			key.WithHelp("L", "next tab"),
		),
		PrevTab: key.NewBinding(
			key.WithKeys("H"),
			key.WithHelp("H", "prev tab"),
		),
		CloseTab: key.NewBinding(
			key.WithKeys("q"),
			key.WithHelp("q", "close tab"),
		),
		Search: key.NewBinding(
			key.WithKeys("o"),
			key.WithHelp("o", "open file"),
		),
	}
}

// ShortHelp returns key bindings for the short help view.
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		k.SwitchPane, k.PrevTab, k.NextTab, k.CloseTab,
		k.CharSelect, k.LineSelect, k.Copy, k.Search, k.Cancel, k.Quit,
	}
}

// FullHelp returns key bindings for the full help view.
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Left, k.Right, k.GoTop, k.GoBottom, k.BlockUp, k.BlockDown},
		{k.Enter, k.SwitchPane, k.PrevTab, k.NextTab, k.CloseTab},
		{k.CharSelect, k.LineSelect, k.Copy, k.Comment, k.ClearAll, k.Search, k.Cancel, k.Quit},
	}
}

// contextKeyMap returns a help.KeyMap with bindings enabled/disabled
// based on the current TUI state.
func (m *Model) contextKeyMap() help.KeyMap {
	km := m.keys
	t, hasTab := m.activeTabState()
	km.CharSelect.SetEnabled(hasTab && m.focusPane == paneEditor)
	km.LineSelect.SetEnabled(hasTab && m.focusPane == paneEditor)
	km.Copy.SetEnabled(hasTab && m.focusPane == paneEditor && t.selecting)
	km.Comment.SetEnabled(hasTab && m.focusPane == paneEditor)
	km.ClearAll.SetEnabled(hasTab && m.focusPane == paneEditor)
	km.NextTab.SetEnabled(hasTab)
	km.PrevTab.SetEnabled(hasTab)
	km.CloseTab.SetEnabled(hasTab)
	km.SwitchPane.SetEnabled(hasTab)
	return km
}
