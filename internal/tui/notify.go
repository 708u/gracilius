package tui

import "fmt"

// notifySelectionChanged sends the current selection to the MCP server.
func (m *Model) notifySelectionChanged() {
	t, ok := m.activeTabState()
	if !ok {
		return
	}
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
	t, ok := m.activeTabState()
	if !ok {
		return
	}
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
func (m *Model) notifyComment(startLine, endLine int, comment string) {
	t, ok := m.activeTabState()
	if !ok {
		return
	}
	var text string
	if startLine == endLine {
		text = fmt.Sprintf("[Comment] %s:%d\n%s",
			t.filePath, startLine+1, comment)
	} else {
		text = fmt.Sprintf("[Comment] %s:%d-%d\n%s",
			t.filePath, startLine+1, endLine+1, comment)
	}

	m.server.NotifySelectionChanged(
		t.filePath,
		text,
		startLine,
		0,
		endLine,
		0,
	)
}
