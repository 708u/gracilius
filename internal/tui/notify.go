package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
)

// scheduleSelectionNotify bumps the generation counter and returns a
// tea.Cmd that fires after selectionDebounce. Only the latest generation
// triggers the actual notification, coalescing rapid cursor movements.
func (m *Model) scheduleSelectionNotify() tea.Cmd {
	m.selectionGen++
	gen := m.selectionGen
	return tea.Tick(selectionDebounce, func(time.Time) tea.Msg {
		return selectionDebounceMsg{gen: gen}
	})
}

// notifySelectionChanged sends the current selection to the MCP server.
func (m *Model) notifySelectionChanged() {
	t, ok := m.activeTabState()
	if !ok {
		return
	}
	sel := t.getSelectionInfo()
	m.server.NotifySelectionChanged(
		t.filePath,
		sel.text,
		sel.startLine,
		sel.startChar,
		sel.endLine,
		sel.endChar,
	)
}

// notifyClearSelection sends a clear-selection notification.
func (m *Model) notifyClearSelection() {
	t, ok := m.activeTabState()
	if !ok {
		return
	}
	line, char := t.getCursorPos()
	m.server.NotifySelectionChanged(
		t.filePath,
		"",
		line,
		char,
		line,
		char,
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

// notifyDiffComment sends a diff comment with side info as a selection_changed notification.
func (m *Model) notifyDiffComment(side diffSide, startLine, endLine int, commentText string) {
	t, ok := m.activeTabState()
	if !ok {
		return
	}
	var text string
	if startLine == endLine {
		text = fmt.Sprintf("[DiffComment:%s] %s:%d\n%s",
			side, t.filePath, startLine+1, commentText)
	} else {
		text = fmt.Sprintf("[DiffComment:%s] %s:%d-%d\n%s",
			side, t.filePath, startLine+1, endLine+1, commentText)
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
