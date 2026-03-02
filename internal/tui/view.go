package tui

import (
	"fmt"
	"path/filepath"
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
		return fmt.Sprintf("Error: %v\n\nPress Ctrl+C to quit.", m.err)
	}

	if m.width == 0 || m.height == 0 {
		return ""
	}

	t := m.activeTabState()

	// header
	header := fmt.Sprintf("gracilius - Port %d", m.server.Port())
	if t != nil && t.filePath != "" {
		header += fmt.Sprintf(" | %s", t.filePath)
	}
	// content
	lo := m.computeLayout()

	treeLines := m.renderTree(lo.treeWidth, lo.contentHeight)

	var editorContent string
	if t == nil {
		editorContent = renderWelcome(lo.editorWidth, lo.contentHeight)
	} else {
		editorLines := m.renderEditor(lo.editorWidth, lo.contentHeight)
		editorContent = strings.Join(editorLines, "\n")
	}

	sepLines := make([]string, lo.contentHeight)
	for i := range sepLines {
		sepLines[i] = " \u2502 "
	}

	content := lipgloss.JoinHorizontal(
		lipgloss.Top,
		strings.Join(treeLines, "\n"),
		strings.Join(sepLines, "\n"),
		editorContent,
	)

	// footer
	footer := m.renderFooter()

	footerRendered := styleFooter.
		Width(m.width).
		Render(footer)

	tabBar := m.renderTabBar(lo.editorStartX)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		tabBar,
		content,
		footerRendered,
	)
}

// renderTabBar generates the tab bar (2 lines: labels + underline).
// offset is the left padding to align with the editor pane.
func (m *Model) renderTabBar(offset int) string {
	if len(m.tabs) == 0 {
		return "\n"
	}

	styleActive := lipgloss.NewStyle().
		Foreground(lipgloss.Color(activeTheme.tabActiveFg))
	styleInactive := lipgloss.NewStyle().
		Foreground(lipgloss.Color(activeTheme.tabInactiveFg))
	styleBorder := lipgloss.NewStyle().
		Foreground(lipgloss.Color(activeTheme.tabActiveBorder))

	padding := strings.Repeat(" ", offset)

	var labels []string
	var borders []string

	for i, t := range m.tabs {
		name := "[empty]"
		if t.filePath != "" {
			name = filepath.Base(t.filePath)
		}
		if t.kind == diffTab {
			name = "[diff] " + name
		}

		label := " " + name + " "
		w := len([]rune(label))
		if i == m.activeTab {
			labels = append(labels, styleActive.Render(label))
			borders = append(borders, styleBorder.Render(
				strings.Repeat("\u2500", w)))
		} else {
			labels = append(labels, styleInactive.Render(label))
			borders = append(borders, strings.Repeat(" ", w))
		}
	}

	sep := " "
	borderSep := " "
	labelLine := strings.Join(labels, sep)
	borderLine := strings.Join(borders, borderSep)

	return ansi.Truncate(padding+labelLine, m.width, "...") +
		"\n" +
		ansi.Truncate(padding+borderLine, m.width, "")
}

// renderFooter generates the footer area (help hints + status).
func (m *Model) renderFooter() string {
	t := m.activeTabState()

	var sb strings.Builder

	if m.quitPending {
		sb.WriteString("Press Ctrl+C again to quit")
		return sb.String()
	}

	if t != nil && t.inputMode {
		sb.WriteString("[Editor] Comment (Enter: confirm, Esc: cancel)\n")
		fmt.Fprintf(&sb, "Line %d: %s",
			t.inputLine+1, t.commentInput.View())
	} else {
		m.help.Width = m.width
		sb.WriteString(m.help.View(m.contextKeyMap()))
		sb.WriteString("\n")

		if t == nil {
			sb.WriteString("Open a file from the tree to begin")
		} else if m.focusPane == paneEditor {
			switch {
			case t.selecting:
				sLine, sChar, eLine, eChar := t.normalizedSelection()
				fmt.Fprintf(&sb, "Selection: %d:%d - %d:%d",
					sLine+1, sChar+1, eLine+1, eChar+1)
				if m.statusMsg != "" {
					fmt.Fprintf(&sb, "  %s", m.statusMsg)
				}
			case len(t.lines) > 0:
				fmt.Fprintf(&sb, "Cursor: %d:%d",
					t.cursorLine+1, t.cursorChar+1)
			default:
				sb.WriteString("Select a file to view")
			}
		} else if m.treeCursor < len(m.fileTree) {
			entry := m.fileTree[m.treeCursor]
			sb.WriteString(entry.path)
		}
	}

	return sb.String()
}

// renderTree generates the tree pane lines.
func (m *Model) renderTree(width, height int) []string {
	lines := make([]string, 0, height)

	var activeFilePath string
	if t := m.activeTabState(); t != nil {
		activeFilePath = t.filePath
	}

	for i := m.treeScrollOffset; i < len(m.fileTree) && len(lines) < height; i++ {
		entry := m.fileTree[i]
		indent := strings.Repeat("  ", entry.depth)

		isCursor := i == m.treeCursor && m.focusPane == paneTree
		isActiveFile := !entry.isDir && entry.path == activeFilePath

		var arrow string
		if entry.isDir {
			if entry.expanded {
				arrow = "\u25be "
			} else {
				arrow = "\u25b8 "
			}
		} else {
			arrow = "  "
		}

		icon := iconFor(m.iconMode, entry)

		line := indent + arrow + icon.prefix() + entry.name
		displayLine := ansi.Truncate(line, width, "...")
		displayLine = padRight(displayLine, width)

		switch {
		case isCursor:
			displayLine = styleTreeCursor().Render(displayLine)
		case isActiveFile:
			displayLine = lipgloss.NewStyle().
				Background(lipgloss.Color(activeTheme.activeFileBg)).
				Render(displayLine)
		}

		displayLine = icon.colorize(displayLine)

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
		fmt.Fprintf(&sb, "%3d ", i+1)

		isCursorLine := m.focusPane == paneEditor && i == t.cursorLine
		isSelected := m.focusPane == paneEditor && t.selecting && i >= startLine && i <= endLine

		switch {
		case isCursorLine && isSelected:
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
		case isCursorLine:
			if hl := t.getHighlightedLine(i); hl != nil {
				renderStyledLineWithCursor(&sb, hl.runs, t.cursorChar)
			} else {
				renderLineWithCursor(&sb, lineContent, t.cursorChar)
			}
		case isSelected:
			sc, ec := selRange(i, startLine, endLine, startChar, endChar, lineContent)
			if hl := t.getHighlightedLine(i); hl != nil {
				renderStyledLineWithSelection(&sb, hl.runs, sc, ec)
			} else {
				renderLineWithCursorAndSelection(&sb, lineContent, sc, ec)
			}
		default:
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
