package tui

import (
	"fmt"
	"strings"
)

// notifySelectionChanged sends the current selection to the MCP server.
func (m Model) notifySelectionChanged() {
	startLine, startChar := m.anchorLine, m.anchorChar
	endLine, endChar := m.cursorLine, m.cursorChar

	if startLine > endLine || (startLine == endLine && startChar > endChar) {
		startLine, endLine = endLine, startLine
		startChar, endChar = endChar, startChar
	}

	var text string
	if startLine == endLine {
		if startLine < len(m.lines) {
			runes := []rune(m.lines[startLine])
			if startChar <= len(runes) && endChar <= len(runes) {
				text = string(runes[startChar:endChar])
			}
		}
	} else {
		var parts []string
		for i := startLine; i <= endLine; i++ {
			if i >= len(m.lines) {
				continue
			}
			runes := []rune(m.lines[i])
			if i == startLine {
				if startChar <= len(runes) {
					parts = append(parts, string(runes[startChar:]))
				}
			} else if i == endLine {
				if endChar <= len(runes) {
					parts = append(parts, string(runes[:endChar]))
				}
			} else {
				parts = append(parts, m.lines[i])
			}
		}
		text = strings.Join(parts, "\n")
	}

	m.server.NotifySelectionChanged(
		m.filePath,
		text,
		startLine,
		startChar,
		endLine,
		endChar,
	)
}

// notifyClearSelection sends a clear-selection notification.
func (m Model) notifyClearSelection() {
	m.server.NotifySelectionChanged(
		m.filePath,
		"",
		m.cursorLine,
		m.cursorChar,
		m.cursorLine,
		m.cursorChar,
	)
}

// notifyComment sends a comment as a selection_changed notification.
func (m Model) notifyComment(line int, comment string) {
	text := fmt.Sprintf("[Comment] %s:%d\n%s", m.filePath, line+1, comment)

	m.server.NotifySelectionChanged(
		m.filePath,
		text,
		line,
		0,
		line,
		0,
	)
}
