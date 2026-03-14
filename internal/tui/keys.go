package tui

import (
	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
)

type keyMap struct {
	Quit          key.Binding
	Cancel        key.Binding
	SwitchPane    key.Binding
	Enter         key.Binding
	Up            key.Binding
	Down          key.Binding
	Left          key.Binding
	Right         key.Binding
	CharSelect    key.Binding
	LineSelect    key.Binding
	Copy          key.Binding
	Comment       key.Binding
	CommentSubmit key.Binding
	ClearAll      key.Binding
	GoTop         key.Binding
	GoBottom      key.Binding
	BlockUp       key.Binding
	BlockDown     key.Binding
	NextTab       key.Binding
	PrevTab       key.Binding
	CloseTab      key.Binding
	OpenFile      key.Binding
	AcceptDiff    key.Binding
	RejectDiff    key.Binding
	SwitchPanel   key.Binding
	ToggleSidebar key.Binding
	Confirm       key.Binding
}

func newKeyMap() keyMap {
	return keyMap{
		Quit: key.NewBinding(
			key.WithKeys("ctrl+c"),
			key.WithHelp("Ctrl+C (2x)", "quit"),
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
		CommentSubmit: key.NewBinding(
			key.WithKeys("ctrl+d"),
			key.WithHelp("Enter/Ctrl+D", "save comment"),
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
		OpenFile: key.NewBinding(
			key.WithKeys("o"),
			key.WithHelp("o", "open file"),
		),
		AcceptDiff: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("Enter", "accept diff"),
		),
		RejectDiff: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("Esc", "reject diff"),
		),
		SwitchPanel: key.NewBinding(
			key.WithKeys("shift+tab"),
			key.WithHelp("Shift+Tab", "switch panel"),
		),
		ToggleSidebar: key.NewBinding(
			key.WithKeys("ctrl+b"),
			key.WithHelp("Ctrl+b", "toggle sidebar"),
		),
		Confirm: key.NewBinding(
			key.WithKeys("y"),
		),
	}
}

// ShortHelp returns key bindings for the short help view.
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		k.SwitchPane, k.SwitchPanel, k.ToggleSidebar,
		k.PrevTab, k.NextTab, k.CloseTab,
		k.CharSelect, k.LineSelect, k.Copy, k.OpenFile,
		k.AcceptDiff, k.RejectDiff, k.Cancel, k.Quit,
	}
}

// FullHelp returns key bindings for the full help view.
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Left, k.Right, k.GoTop, k.GoBottom, k.BlockUp, k.BlockDown},
		{k.Enter, k.SwitchPane, k.SwitchPanel, k.ToggleSidebar, k.PrevTab, k.NextTab, k.CloseTab},
		{k.CharSelect, k.LineSelect, k.Copy, k.Comment, k.ClearAll, k.OpenFile, k.AcceptDiff, k.RejectDiff, k.Cancel, k.Quit},
	}
}

// contextKeyMap returns a help.KeyMap with bindings enabled/disabled
// based on the current TUI state.
func (m *Model) contextKeyMap() help.KeyMap {
	km := m.keys
	t, hasTab := m.activeTabState()
	isDiffReview := hasTab && t.diff != nil
	isDiffView := hasTab && t.diffViewData != nil
	editorFocus := hasTab && m.focusPane == paneEditor
	km.CharSelect.SetEnabled(editorFocus)
	km.LineSelect.SetEnabled(editorFocus)
	km.Copy.SetEnabled(editorFocus && ((isDiffView && t.diffSelecting) || (!isDiffView && t.selecting)))
	km.Comment.SetEnabled(editorFocus && !isDiffView)
	km.ClearAll.SetEnabled(editorFocus && !isDiffView)
	km.BlockUp.SetEnabled(editorFocus && isDiffView)
	km.BlockDown.SetEnabled(editorFocus && isDiffView)
	km.NextTab.SetEnabled(hasTab)
	km.PrevTab.SetEnabled(hasTab)
	km.CloseTab.SetEnabled(hasTab)
	km.AcceptDiff.SetEnabled(isDiffReview && m.focusPane == paneEditor)
	km.RejectDiff.SetEnabled(isDiffReview && m.focusPane == paneEditor)
	km.SwitchPane.SetEnabled(hasTab && m.sidebarVisible)
	km.SwitchPanel.SetEnabled(true)
	km.ToggleSidebar.SetEnabled(true)
	return km
}
