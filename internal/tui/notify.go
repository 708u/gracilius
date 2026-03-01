package tui

import "fmt"

// notifySelectionChanged sends the current selection to the MCP server.
func (m *Model) notifySelectionChanged() {
	t := m.activeTabState()
	startLine, startChar, endLine, endChar := t.normalizedSelection()

	m.server.NotifySelectionChanged(
		t.filePath,
		t.selectedText(),
		startLine,
		startChar,
		endLine,
		endChar,
	)
}

// notifyClearSelection sends a clear-selection notification.
func (m *Model) notifyClearSelection() {
	t := m.activeTabState()
	m.server.NotifySelectionChanged(
		t.filePath,
		"",
		t.cursorLine,
		t.cursorChar,
		t.cursorLine,
		t.cursorChar,
	)
}

// notifyComment sends a comment as a selection_changed notification.
func (m *Model) notifyComment(line int, comment string) {
	t := m.activeTabState()
	text := fmt.Sprintf("[Comment] %s:%d\n%s", t.filePath, line+1, comment)

	m.server.NotifySelectionChanged(
		t.filePath,
		text,
		line,
		0,
		line,
		0,
	)
}
