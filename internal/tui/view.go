package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// cursorPosition represents screen-space cursor coordinates.
// A zero value means no cursor is visible.
type cursorPosition struct {
	x, y int
}

func (c cursorPosition) isZero() bool {
	return c.x == 0 && c.y == 0
}

func (c cursorPosition) XY() (int, int) {
	return c.x, c.y
}

var separatorBorder = lipgloss.Border{
	Top: "\u2500",
}

const emptyStateMsg = "Select a file to view"

const (
	commentHintEnhanced = "Enter: save, Shift+Enter: newline, Esc: cancel"
	commentHintBasic    = "Ctrl+D: save, Esc: cancel"
)

var (
	styleComment   = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	styleInput     = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	styleBodyWhite = lipgloss.NewStyle().Foreground(lipgloss.Color("15"))
	styleFooter    = lipgloss.NewStyle().
			BorderTop(true).
			BorderStyle(separatorBorder)
)

func styleTreeCursor(theme themeConfig) lipgloss.Style {
	return lipgloss.NewStyle().Background(lipgloss.Color(theme.listSelectionBg))
}

// newView returns a tea.View with the base terminal settings.
func newView(content string) tea.View {
	var v tea.View
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	v.SetContent(content)
	return v
}

// View implements tea.Model.
func (m *Model) View() tea.View {
	if m.err != nil {
		return newView(fmt.Sprintf("Error: %v\n\nPress Ctrl+C to quit.", m.err))
	}

	if m.width == 0 || m.height == 0 {
		return newView("")
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
		editorLines = renderWelcome(lo.editorWidth, lo.contentHeight, m.theme)
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
		v := newView(m.openFile.overlay(base, m.width, m.height))
		if cp := m.openFile.cursorPos(m.width, m.height); !cp.isZero() {
			v.Cursor = tea.NewCursor(cp.XY())
		}
		return v
	}

	v := newView(base)
	var cp cursorPosition
	switch {
	case hasTab && t.inputMode:
		cp = m.commentCursorScreenPos(lo)
	case hasTab:
		cp = m.cursorScreenPos(lo)
	}
	if !cp.isZero() {
		v.Cursor = tea.NewCursor(cp.XY())
	}
	return v
}

// cursorScreenPos computes the screen coordinates for the editor cursor
// using lastMapping (which must be populated by renderEditor before this call).
func (m *Model) cursorScreenPos(lo layout) cursorPosition {
	t, ok := m.activeTabState()
	if !ok || m.focusPane != paneEditor ||
		len(t.lines) == 0 || len(m.lastMapping) == 0 {
		return cursorPosition{}
	}

	visualRow := -1
	for i, ve := range m.lastMapping {
		if ve.kind != lineKindCode {
			continue
		}
		if ve.logicalLine != t.cursorLine {
			continue
		}

		segEnd := t.lineLen(t.cursorLine)
		for j := i + 1; j < len(m.lastMapping); j++ {
			next := m.lastMapping[j]
			if next.logicalLine == t.cursorLine &&
				next.kind == lineKindCode {
				segEnd = next.wrapOffset
				break
			}
			if next.logicalLine != t.cursorLine {
				break
			}
		}

		if t.cursorChar >= ve.wrapOffset &&
			t.cursorChar < segEnd {
			visualRow = i
			break
		}
		if t.cursorChar >= ve.wrapOffset {
			visualRow = i
		}
	}

	if visualRow < 0 {
		return cursorPosition{}
	}

	return cursorPosition{
		x: lo.editorStartX + lo.lineNumWidth +
			displayWidthRange(
				t.lines[t.cursorLine],
				m.lastMapping[visualRow].wrapOffset,
				t.cursorChar,
			),
		y: contentStartY + visualRow,
	}
}

// commentCursorScreenPos computes the screen-space cursor position for the
// comment textarea by finding its input block in lastMapping.
func (m *Model) commentCursorScreenPos(lo layout) cursorPosition {
	t, hasTab := m.activeTabState()
	if !hasTab || !t.inputMode {
		return cursorPosition{}
	}
	c := t.commentInput.Cursor()
	if c == nil {
		return cursorPosition{}
	}

	// Find the first lineKindInput entry in lastMapping.
	blockStart := -1
	for i, ve := range m.lastMapping {
		if ve.kind == lineKindInput {
			blockStart = i
			break
		}
	}
	if blockStart < 0 {
		return cursorPosition{}
	}

	return cursorPosition{
		x: lo.editorStartX + lo.lineNumWidth + blockBorderLeft + c.X,
		y: contentStartY + blockStart + blockBorderTop + c.Y,
	}
}

// renderTabBar generates the tab bar (2 lines: labels + underline).
// offset is the left padding to align with the editor pane.
func (m *Model) renderTabBar(offset int) string {
	if len(m.tabs) == 0 {
		return "\n"
	}

	styleActive := lipgloss.NewStyle().
		Foreground(lipgloss.Color(m.theme.tabActiveFg))
	styleInactive := lipgloss.NewStyle().
		Foreground(lipgloss.Color(m.theme.tabInactiveFg))
	styleBorder := lipgloss.NewStyle().
		Foreground(lipgloss.Color(m.theme.tabActiveBorder))

	padding := strings.Repeat(" ", offset)

	var labels []string
	var borders []string

	for i, t := range m.tabs {
		name := "[empty]"
		if t.filePath != "" {
			name = filepath.Base(t.filePath)
		}
		if t.kind == diffTab {
			if t.diff != nil {
				name = "[review] " + name
			} else {
				name = "[diff] " + name
			}
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
		hint := commentHintBasic
		if m.enhancedKeyboard {
			hint = commentHintEnhanced
		}
		sb.WriteString("[Comment] " + hint)
	} else {
		m.help.SetWidth(m.width)
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
			displayLine = styleTreeCursor(m.theme).Render(displayLine)
		case isActiveFile:
			displayLine = lipgloss.NewStyle().
				Background(lipgloss.Color(m.theme.activeFileBg)).
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
	selBgSeq := m.theme.selectionBgSeq()
	offset := t.vp.YOffset()
	commentBodyWidth := width - lnw - commentBlockMargin
	total := len(t.lines)
	gutterCtx := viewport.GutterContext{TotalLines: total}

	for i := offset; i < len(t.lines) && len(lines) < height; i++ {
		lineContent := t.lines[i]

		// Build line number prefix via LeftGutterFunc
		gutterCtx.Index = i
		gutterCtx.Soft = false
		lineNumStr := t.vp.LeftGutterFunc(gutterCtx)

		// Build content and emit visual rows
		isSelected := m.focusPane == paneEditor && t.selecting && i >= startLine && i <= endLine

		bp := wrapBreakpoints(lineContent, textWidth)
		if bp != nil {
			// Per-segment rendering: split runs at wrap breakpoints,
			// then apply cursor/selection per segment independently.
			var runs []styledRun
			if hl := t.getHighlightedLine(i); hl != nil {
				runs = hl.runs
			} else {
				runs = []styledRun{{Text: lineContent}}
			}

			var sc, ec int
			if isSelected {
				sc, ec = selRange(i, startLine, endLine, startChar, endChar, lineContent)
			}

			segRunsList := splitRunsAtBreakpoints(runs, bp)
			for si, segRuns := range segRunsList {
				if len(lines) >= height {
					break
				}

				wrapOff := 0
				if si > 0 && si-1 < len(bp) {
					wrapOff = bp[si-1]
				}

				segLen := 0
				for _, r := range segRuns {
					segLen += len([]rune(r.Text))
				}

				// Clamp selection range to this segment.
				segSC, segEC := 0, 0
				if isSelected && sc < ec {
					segSC = max(0, sc-wrapOff)
					segEC = min(segLen, ec-wrapOff)
					if segSC >= segEC {
						segSC, segEC = 0, 0
					}
				}

				// Render segment content.
				var segSB strings.Builder
				switch {
				case segSC < segEC:
					renderStyledLineWithSelection(&segSB, segRuns, segSC, segEC, selBgSeq)
				default:
					for _, r := range segRuns {
						writeStyledText(&segSB, r.ANSI, expandTabs(r.Text))
					}
				}

				seg := segSB.String()
				if si > 0 {
					gutterCtx.Soft = true
					lnPad := t.vp.LeftGutterFunc(gutterCtx)
					lines = append(lines, padRight(lnPad+seg+ansiReset, width))
				} else {
					lines = append(lines, padRight(lineNumStr+seg+ansiReset, width))
				}
				mapping = append(mapping, visualEntry{
					logicalLine: i,
					wrapOffset:  wrapOff,
				})
			}
		} else {
			// Non-wrapped: build full content then emit.
			var contentSB strings.Builder

			switch {
			case isSelected:
				sc, ec := selRange(i, startLine, endLine, startChar, endChar, lineContent)
				if hl := t.getHighlightedLine(i); hl != nil {
					renderStyledLineWithSelection(&contentSB, hl.runs, sc, ec, selBgSeq)
				} else {
					renderLineWithCursorAndSelection(&contentSB, lineContent, sc, ec, selBgSeq)
				}
			default:
				if hl := t.getHighlightedLine(i); hl != nil {
					contentSB.WriteString(hl.rendered)
				} else {
					contentSB.WriteString(expandTabs(lineContent))
				}
			}

			content := contentSB.String()
			lines = append(lines, padRight(lineNumStr+content+ansiReset, width))
			mapping = append(mapping, visualEntry{logicalLine: i})
		}

		if t.inputMode && i == t.inputEnd {
			gutterCtx.Soft = true
			lnPad := t.vp.LeftGutterFunc(gutterCtx)
			hint := commentHintBasic
			if m.enhancedKeyboard {
				hint = commentHintEnhanced
			}
			label := "comment (" + hint + ")"
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
			gutterCtx.Soft = true
			lnPad := t.vp.LeftGutterFunc(gutterCtx)
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

// renderLineWithCursorAndSelection renders a line with selection highlight.
func renderLineWithCursorAndSelection(sb *strings.Builder, line string, selStart, selEnd int, selBgSeq string) {
	runs := []styledRun{{Text: line}}
	renderStyledLineWithSelection(sb, runs, selStart, selEnd, selBgSeq)
}
