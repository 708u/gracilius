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
	stylePreview     = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	styleDiffAdded   = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	styleDiffRemoved = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	styleComment     = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	styleTreeCursor  = lipgloss.NewStyle().Reverse(true)
	styleFooter      = lipgloss.NewStyle().
				BorderTop(true).
				BorderStyle(separatorBorder)
)

// View implements tea.Model.
func (m Model) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n\nPress Esc to quit.", m.err)
	}

	if m.width == 0 || m.height == 0 {
		return ""
	}

	// header
	header := fmt.Sprintf("gracilius - Port %d", m.server.Port())
	if m.filePath != "" {
		header += fmt.Sprintf(" | %s", m.filePath)
	}
	if m.previewLines != nil {
		header += " " + stylePreview.Render("[PREVIEW]")
	}

	// content
	treeWidth := m.getTreeWidth()
	editorWidth := m.width - treeWidth - 3
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

	headerRendered := header
	footerRendered := styleFooter.
		Width(m.width).
		Render(footer)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		headerRendered,
		content,
		footerRendered,
	)
}

// renderFooter generates the footer area (help hints + status).
func (m Model) renderFooter() string {
	var sb strings.Builder

	if m.inputMode {
		sb.WriteString("[Editor] Comment (Enter: confirm, Esc: cancel)\n")
		fmt.Fprintf(&sb, "Line %d: %s",
			m.inputLine+1, m.commentInput.View())
	} else {
		m.help.Width = m.width
		sb.WriteString(m.help.View(m.contextKeyMap()))
		sb.WriteString("\n")

		if m.focusPane == 1 {
			if m.previewLines != nil {
				sb.WriteString("Preview mode - waiting for accept/reject")
			} else if m.selecting {
				sLine, sChar := m.anchorLine, m.anchorChar
				eLine, eChar := m.cursorLine, m.cursorChar
				if sLine > eLine || (sLine == eLine && sChar > eChar) {
					sLine, eLine = eLine, sLine
					sChar, eChar = eChar, sChar
				}
				fmt.Fprintf(&sb, "Selection: %d:%d - %d:%d",
					sLine+1, sChar+1, eLine+1, eChar+1)
			} else if len(m.lines) > 0 {
				fmt.Fprintf(&sb, "Cursor: %d:%d",
					m.cursorLine+1, m.cursorChar+1)
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
func (m Model) renderTree(width, height int) []string {
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

		if i == m.treeCursor && m.focusPane == 0 {
			displayLine = styleTreeCursor.Render(displayLine)
		}

		lines = append(lines, displayLine)
	}

	for len(lines) < height {
		lines = append(lines, padRight("", width))
	}

	return lines
}

// renderEditor generates the editor pane lines.
func (m Model) renderEditor(width, height int) []string {
	lines := make([]string, 0, height)

	if len(m.lines) == 0 {
		emptyMsg := "No file selected"
		lines = append(lines, padRight(emptyMsg, width))
		for len(lines) < height {
			lines = append(lines, padRight("", width))
		}
		return lines
	}

	if m.previewLines != nil {
		diffLines := computeLineDiff(m.lines, m.previewLines)
		lineNum := 0
		for _, dl := range diffLines {
			if len(lines) >= height {
				break
			}
			var line string
			switch dl.op {
			case '+':
				lineNum++
				line = styleDiffAdded.Render(fmt.Sprintf("%3d + %s", lineNum, expandTabs(dl.text)))
			case '-':
				line = styleDiffRemoved.Render(fmt.Sprintf("    - %s", expandTabs(dl.text)))
			default:
				lineNum++
				line = fmt.Sprintf("%3d   %s", lineNum, expandTabs(dl.text))
			}
			lines = append(lines, line)
		}
	} else {
		startLine, startChar := m.anchorLine, m.anchorChar
		endLine, endChar := m.cursorLine, m.cursorChar
		if startLine > endLine || (startLine == endLine && startChar > endChar) {
			startLine, endLine = endLine, startLine
			startChar, endChar = endChar, startChar
		}

		offset := m.getScrollOffset()

		for i := offset; i < len(m.lines) && len(lines) < height; i++ {
			lineContent := m.lines[i]

			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("%3d ", i+1))

			isCursorLine := m.focusPane == 1 && i == m.cursorLine
			isSelected := m.focusPane == 1 && m.selecting && i >= startLine && i <= endLine

			if isCursorLine && isSelected {
				sc, ec := selRange(i, startLine, endLine, startChar, endChar, lineContent)
				if hl := m.getHighlightedLine(i); hl != nil {
					renderStyledLineWithSelection(&sb, hl.runs, sc, ec)
				} else {
					m.renderLineWithCursorAndSelection(&sb, lineContent, sc, ec)
				}
			} else if isCursorLine {
				if hl := m.getHighlightedLine(i); hl != nil {
					renderStyledLineWithCursor(&sb, hl.runs, m.cursorChar)
				} else {
					m.renderLineWithCursor(&sb, lineContent)
				}
			} else if isSelected {
				sc, ec := selRange(i, startLine, endLine, startChar, endChar, lineContent)
				if hl := m.getHighlightedLine(i); hl != nil {
					renderStyledLineWithSelection(&sb, hl.runs, sc, ec)
				} else {
					m.renderLineWithCursorAndSelection(&sb, lineContent, sc, ec)
				}
			} else {
				if hl := m.getHighlightedLine(i); hl != nil {
					sb.WriteString(hl.rendered)
				} else {
					sb.WriteString(expandTabs(lineContent))
				}
			}

			if _, hasComment := m.comments[i]; hasComment {
				sb.WriteString(" " + styleComment.Render("[C]"))
			}

			lines = append(lines, sb.String())
		}
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
func (m Model) renderLineWithCursor(sb *strings.Builder, line string) {
	runes := []rune(line)
	if m.cursorChar >= len(runes) {
		sb.WriteString(expandTabs(line))
		sb.WriteString("\033[7m \033[0m")
	} else {
		sb.WriteString(expandTabs(string(runes[:m.cursorChar])))
		sb.WriteString("\033[7m")
		ch := runes[m.cursorChar]
		if ch == '\t' {
			sb.WriteString("    ")
		} else {
			sb.WriteString(string(ch))
		}
		sb.WriteString("\033[0m")
		sb.WriteString(expandTabs(string(runes[m.cursorChar+1:])))
	}
}

// renderLineWithCursorAndSelection renders a line with selection highlight.
func (m Model) renderLineWithCursorAndSelection(sb *strings.Builder, line string, selStart, selEnd int) {
	runes := []rune(line)
	if selStart > len(runes) {
		selStart = len(runes)
	}
	if selEnd > len(runes) {
		selEnd = len(runes)
	}
	if selStart > selEnd {
		selStart, selEnd = selEnd, selStart
	}

	sb.WriteString(expandTabs(string(runes[:selStart])))
	sb.WriteString("\033[7m")
	sb.WriteString(expandTabs(string(runes[selStart:selEnd])))
	sb.WriteString("\033[0m")
	sb.WriteString(expandTabs(string(runes[selEnd:])))
}
