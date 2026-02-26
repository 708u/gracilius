package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

var separatorBorder = lipgloss.Border{
	Top: "\u2500",
}

var (
	styleComment = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	styleFooter  = lipgloss.NewStyle().
			BorderTop(true).
			BorderStyle(separatorBorder)
)

func styleTreeCursor() lipgloss.Style {
	return lipgloss.NewStyle().Background(lipgloss.Color(activeTheme.listSelectionBg))
}

// View implements tea.Model.
func (m *Model) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n\nPress Esc to quit.", m.err)
	}

	if m.width == 0 || m.height == 0 {
		return ""
	}

	t := m.activeTabState()

	// header
	header := fmt.Sprintf("gracilius - Port %d", m.server.Port())
	if t.filePath != "" {
		header += fmt.Sprintf(" | %s", t.filePath)
	}
	// content
	treeWidth := m.getTreeWidth()
	editorWidth := m.width - treeWidth - separatorWidth
	contentHeight := m.getContentHeight()

	treeLines := m.renderTree(treeWidth, contentHeight)
	editorLines := m.renderEditor(editorWidth, contentHeight)

	sepLines := make([]string, contentHeight)
	for i := range sepLines {
		sepLines[i] = " \u2502 "
	}

	content := lipgloss.JoinHorizontal(
		lipgloss.Top,
		strings.Join(treeLines, "\n"),
		strings.Join(sepLines, "\n"),
		strings.Join(editorLines, "\n"),
	)

	// footer
	footer := m.renderFooter()

	footerRendered := styleFooter.
		Width(m.width).
		Render(footer)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		content,
		footerRendered,
	)
}

// renderFooter generates the footer area (help hints + status).
func (m *Model) renderFooter() string {
	t := m.activeTabState()

	var sb strings.Builder

	if t.inputMode {
		sb.WriteString("[Editor] Comment (Enter: confirm, Esc: cancel)\n")
		fmt.Fprintf(&sb, "Line %d: %s",
			t.inputLine+1, t.commentInput.View())
	} else {
		m.help.Width = m.width
		sb.WriteString(m.help.View(m.contextKeyMap()))
		sb.WriteString("\n")

		if m.focusPane == paneEditor {
			if t.selecting {
				sLine, sChar, eLine, eChar := t.normalizedSelection()
				fmt.Fprintf(&sb, "Selection: %d:%d - %d:%d",
					sLine+1, sChar+1, eLine+1, eChar+1)
			} else if len(t.lines) > 0 {
				fmt.Fprintf(&sb, "Cursor: %d:%d",
					t.cursorLine+1, t.cursorChar+1)
			} else {
				sb.WriteString("Select a file to view")
			}
		} else {
			if m.treeCursor < len(m.fileTree) {
				entry := m.fileTree[m.treeCursor]
				sb.WriteString(entry.path)
			}
		}
	}

	return sb.String()
}

// renderTree generates the tree pane lines.
func (m *Model) renderTree(width, height int) []string {
	lines := make([]string, 0, height)

	for i := m.treeScrollOffset; i < len(m.fileTree) && len(lines) < height; i++ {
		entry := m.fileTree[i]
		indent := strings.Repeat("  ", entry.depth)

		icon := "  "
		if entry.isDir {
			if entry.expanded {
				icon = "\u25bc "
			} else {
				icon = "\u25b6 "
			}
		}

		line := indent + icon + entry.name

		displayLine := ansi.Truncate(line, width, "...")
		displayLine = padRight(displayLine, width)

		if i == m.treeCursor && m.focusPane == paneTree {
			displayLine = styleTreeCursor().Render(displayLine)
		}

		lines = append(lines, displayLine)
	}

	for len(lines) < height {
		lines = append(lines, padRight("", width))
	}

	return lines
}

// renderEditor generates the editor pane lines.
func (m *Model) renderEditor(width, height int) []string {
	t := m.activeTabState()

	lines := make([]string, 0, height)

	if len(t.lines) == 0 {
		emptyMsg := "No file selected"
		lines = append(lines, padRight(emptyMsg, width))
		for len(lines) < height {
			lines = append(lines, padRight("", width))
		}
		return lines
	}

	startLine, startChar, endLine, endChar := t.normalizedSelection()

	offset := t.scrollOffset

	for i := offset; i < len(t.lines) && len(lines) < height; i++ {
		lineContent := t.lines[i]

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("%3d ", i+1))

		isCursorLine := m.focusPane == paneEditor && i == t.cursorLine
		isSelected := m.focusPane == paneEditor && t.selecting && i >= startLine && i <= endLine

		if isCursorLine && isSelected {
			sc, ec := selRange(i, startLine, endLine, startChar, endChar, lineContent)
			if sc == ec {
				if hl := t.getHighlightedLine(i); hl != nil {
					renderStyledLineWithCursor(&sb, hl.runs, t.cursorChar)
				} else {
					renderLineWithCursor(&sb, lineContent, t.cursorChar)
				}
			} else if hl := t.getHighlightedLine(i); hl != nil {
				renderStyledLineWithSelection(&sb, hl.runs, sc, ec)
			} else {
				renderLineWithCursorAndSelection(&sb, lineContent, sc, ec)
			}
		} else if isCursorLine {
			if hl := t.getHighlightedLine(i); hl != nil {
				renderStyledLineWithCursor(&sb, hl.runs, t.cursorChar)
			} else {
				renderLineWithCursor(&sb, lineContent, t.cursorChar)
			}
		} else if isSelected {
			sc, ec := selRange(i, startLine, endLine, startChar, endChar, lineContent)
			if hl := t.getHighlightedLine(i); hl != nil {
				renderStyledLineWithSelection(&sb, hl.runs, sc, ec)
			} else {
				renderLineWithCursorAndSelection(&sb, lineContent, sc, ec)
			}
		} else {
			if hl := t.getHighlightedLine(i); hl != nil {
				sb.WriteString(hl.rendered)
			} else {
				sb.WriteString(expandTabs(lineContent))
			}
		}

		if _, hasComment := t.comments[i]; hasComment {
			sb.WriteString(" " + styleComment.Render("[C]"))
		}

		lines = append(lines, sb.String())
	}

	for len(lines) < height {
		lines = append(lines, padRight("", width))
	}

	return lines
}

// selRange computes selection start/end character positions for a line.
func selRange(line, startLine, endLine, startChar, endChar int, content string) (int, int) {
	sc := 0
	if line == startLine {
		sc = startChar
	}
	ec := len([]rune(content))
	if line == endLine {
		ec = endChar
	}
	return sc, ec
}

// renderLineWithCursor renders a line with an inverted cursor.
func renderLineWithCursor(sb *strings.Builder, line string, cursorChar int) {
	runs := []styledRun{{Text: line}}
	renderStyledLineWithCursor(sb, runs, cursorChar)
}

// renderLineWithCursorAndSelection renders a line with selection highlight.
func renderLineWithCursorAndSelection(sb *strings.Builder, line string, selStart, selEnd int) {
	runs := []styledRun{{Text: line}}
	renderStyledLineWithSelection(sb, runs, selStart, selEnd)
}
