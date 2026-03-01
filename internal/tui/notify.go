package tui

import (
	"fmt"
	"strings"
)

// notifySelectionChanged sends the current selection to the MCP server.
func (m *Model) notifySelectionChanged() {
	t := m.activeTabState()
	startLine, startChar, endLine, endChar := t.normalizedSelection()

	var text string
	if startLine == endLine {
		if startLine < len(t.lines) {
			runes := []rune(t.lines[startLine])
			if startChar <= len(runes) && endChar <= len(runes) {
				text = string(runes[startChar:endChar])
			}
		}
	} else {
		var parts []string
		for i := startLine; i <= endLine; i++ {
			if i >= len(t.lines) {
				continue
			}
			runes := []rune(t.lines[i])
			switch i {
			case startLine:
				if startChar <= len(runes) {
					parts = append(parts, string(runes[startChar:]))
				}
			case endLine:
				if endChar <= len(runes) {
					parts = append(parts, string(runes[:endChar]))
				}
			default:
				parts = append(parts, t.lines[i])
			}
		}
		text = strings.Join(parts, "\n")
	}

	m.server.NotifySelectionChanged(
		t.filePath,
		text,
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
