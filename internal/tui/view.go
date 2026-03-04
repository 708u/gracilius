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

const emptyStateMsg = "Select a file to view"

var (
	styleComment   = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	styleInput     = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	styleBodyWhite = lipgloss.NewStyle().Foreground(lipgloss.Color("15"))
	styleFooter    = lipgloss.NewStyle().
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

	t, hasTab := m.activeTabState()

	// header
	header := fmt.Sprintf("gracilius - Port %d", m.server.Port())
	if hasTab && t.filePath != "" {
		header += fmt.Sprintf(" | %s", t.filePath)
	}
	// content
	lo := m.computeLayout()

	treeLines := m.renderTree(lo.treeWidth, lo.contentHeight)

	var editorLines []string
	if !hasTab {
		editorLines = renderWelcome(lo.editorWidth, lo.contentHeight)
	} else {
		editorLines = m.renderEditor(lo)
	}

	sepLines := make([]string, lo.contentHeight)
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

	tabBar := m.renderTabBar(lo.editorStartX)

	base := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		tabBar,
		content,
		footerRendered,
	)

	if m.openFile.active {
		return m.openFile.overlay(base, m.width, m.height)
	}

	return base
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
	t, hasTab := m.activeTabState()

	var sb strings.Builder

	if m.gPending {
		sb.WriteString("g → g: top")
		return sb.String()
	}

	if m.quitPending {
		sb.WriteString("Press Ctrl+C again to quit")
		return sb.String()
	}

	if hasTab && t.inputMode {
		fmt.Fprintf(&sb, "[Comment] %s: save, Esc: cancel",
			m.keys.CommentSubmit.Help().Key)
	} else {
		m.help.Width = m.width
		sb.WriteString(m.help.View(m.contextKeyMap()))
		sb.WriteString("\n")

		switch {
		case !hasTab:
			sb.WriteString("Open a file from the tree to begin")
		case m.focusPane == paneEditor:
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
				sb.WriteString(emptyStateMsg)
			}
		case m.treeCursor < len(m.fileTree):
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
	if t, ok := m.activeTabState(); ok {
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
func (m *Model) renderEditor(lo layout) []string {
	t, hasTab := m.activeTabState()
	width := lo.editorWidth
	height := lo.contentHeight
	lnw := lo.lineNumWidth
	textWidth := lo.textWidth

	lines := make([]string, 0, height)
	var mapping []visualEntry

	if !hasTab || len(t.lines) == 0 {
		emptyMsg := emptyStateMsg
		lines = append(lines, padRight(emptyMsg, width))
		for len(lines) < height {
			lines = append(lines, padRight("", width))
		}
		m.lastMapping = nil
		return lines
	}

	startLine, startChar, endLine, endChar := t.normalizedSelection()
	offset := t.scrollOffset
	commentBodyWidth := width - lnw - commentBlockMargin
	lnPad := strings.Repeat(" ", lnw)
	digitWidth := lnw - 2 // subtract marker and trailing space
	normalFmt := fmt.Sprintf(" %%%dd ", digitWidth)
	barFmt := fmt.Sprintf("%%%dd ", digitWidth)

	for i := offset; i < len(t.lines) && len(lines) < height; i++ {
		lineContent := t.lines[i]

		// Build line number prefix
		var lnSB strings.Builder
		if t.findComment(i) >= 0 {
			lnSB.WriteString(styleComment.Render("\u258e"))
			fmt.Fprintf(&lnSB, barFmt, i+1)
		} else {
			fmt.Fprintf(&lnSB, normalFmt, i+1)
		}
		lineNumStr := lnSB.String()

		// Build content (without line number)
		var contentSB strings.Builder
		isCursorLine := m.focusPane == paneEditor && i == t.cursorLine
		isSelected := m.focusPane == paneEditor && t.selecting && i >= startLine && i <= endLine

		switch {
		case isCursorLine && isSelected:
			sc, ec := selRange(i, startLine, endLine, startChar, endChar, lineContent)
			if sc == ec {
				if hl := t.getHighlightedLine(i); hl != nil {
					renderStyledLineWithCursor(&contentSB, hl.runs, t.cursorChar)
				} else {
					renderLineWithCursor(&contentSB, lineContent, t.cursorChar)
				}
			} else if hl := t.getHighlightedLine(i); hl != nil {
				renderStyledLineWithSelection(&contentSB, hl.runs, sc, ec)
			} else {
				renderLineWithCursorAndSelection(&contentSB, lineContent, sc, ec)
			}
		case isCursorLine:
			if hl := t.getHighlightedLine(i); hl != nil {
				renderStyledLineWithCursor(&contentSB, hl.runs, t.cursorChar)
			} else {
				renderLineWithCursor(&contentSB, lineContent, t.cursorChar)
			}
		case isSelected:
			sc, ec := selRange(i, startLine, endLine, startChar, endChar, lineContent)
			if hl := t.getHighlightedLine(i); hl != nil {
				renderStyledLineWithSelection(&contentSB, hl.runs, sc, ec)
			} else {
				renderLineWithCursorAndSelection(&contentSB, lineContent, sc, ec)
			}
		default:
			if hl := t.getHighlightedLine(i); hl != nil {
				contentSB.WriteString(hl.rendered)
			} else {
				contentSB.WriteString(expandTabs(lineContent))
			}
		}

		content := contentSB.String()

		// Word wrap and emit visual rows
		bp := wrapBreakpoints(lineContent, textWidth)
		if bp != nil {
			wrapped := ansi.Hardwrap(content, textWidth, true)
			segments := strings.Split(wrapped, "\n")
			for si, seg := range segments {
				if len(lines) >= height {
					break
				}
				wrapOff := 0
				if si > 0 {
					if si-1 < len(bp) {
						wrapOff = bp[si-1]
					}
					lines = append(lines, lnPad+ansiReset+seg)
				} else {
					lines = append(lines, lineNumStr+seg)
				}
				mapping = append(mapping, visualEntry{
					logicalLine: i,
					wrapOffset:  wrapOff,
				})
			}
		} else {
			lines = append(lines, lineNumStr+content)
			mapping = append(mapping, visualEntry{logicalLine: i})
		}

		if t.inputMode && i == t.inputEnd {
			label := fmt.Sprintf("comment (%s: save, Esc: cancel)",
				m.keys.CommentSubmit.Help().Key)
			blockRows := renderBlock(
				t.commentInput.View(), label, commentBodyWidth, styleInput, styleBodyWhite)
			for _, r := range blockRows {
				if len(lines) >= height {
					break
				}
				lines = append(lines, lnPad+r)
				mapping = append(mapping,
					visualEntry{logicalLine: i, kind: lineKindInput})
			}
		} else if c := t.commentEndingAt(i); c != nil {
			label := formatCommentLabel(c)
			blockRows := renderBlock(
				c.text, label, commentBodyWidth, styleComment, styleBodyWhite)
			for _, r := range blockRows {
				if len(lines) >= height {
					break
				}
				lines = append(lines, lnPad+r)
				mapping = append(mapping,
					visualEntry{logicalLine: i, kind: lineKindComment})
			}
		}
	}

	for len(lines) < height {
		lines = append(lines, padRight("", width))
	}

	m.lastMapping = mapping
	return lines
}

// formatCommentLabel returns the label for a comment block header.
func formatCommentLabel(c *comment) string {
	if c.startLine == c.endLine {
		return "comment"
	}
	return fmt.Sprintf("comment (L%d-%d)", c.startLine+1, c.endLine+1)
}

// renderBlock renders text inside a bordered block with a label header.
// Each body line is right-padded to width. bodyStyle is applied to body text.
func renderBlock(text, label string, width int, borderStyle, bodyStyle lipgloss.Style) []string {
	if width < 10 {
		width = 10
	}
	var rows []string
	topLabel := "\u256d\u2500 " + label + " "
	remaining := width - len([]rune(topLabel)) - 1
	if remaining > 0 {
		topLabel += strings.Repeat("\u2500", remaining)
	}
	topLabel += "\u256e"
	rows = append(rows, borderStyle.Render(topLabel))

	for line := range strings.SplitSeq(text, "\n") {
		content := "\u2502 " + bodyStyle.Render(line)
		content = padRight(content, width-1)
		rows = append(rows, borderStyle.Render(content+"\u2502"))
	}

	bottom := "\u2570" + strings.Repeat("\u2500", width-2) + "\u256f"
	rows = append(rows, borderStyle.Render(bottom))
	return rows
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
