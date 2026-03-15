package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/708u/gracilius/internal/comment"
	"github.com/charmbracelet/x/ansi"
)

// tabLabel returns the display label for a tab (with leading/trailing space).
func tabLabel(t *tab) string {
	name := "[empty]"
	if t.filePath != "" {
		name = filepath.Base(t.filePath)
	}
	if t.kind == diffTab {
		switch {
		case t.diff != nil:
			name = "[review] " + name
		case t.hasGitDiffModeTag:
			name = t.gitDiffLabel + " " + name
		default:
			name = "[diff] " + name
		}
	}
	return " " + name + " "
}

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
	styleWarning   = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
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
	v.ReportFocus = true
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

	var editorLines []string
	if !hasTab {
		editorLines = renderWelcome(lo.editorWidth, lo.contentHeight, m.theme)
	} else {
		editorLines = m.renderEditor(lo)
	}

	// Overlay search box on the top-right of the editor.
	var searchBoxW int
	if len(editorLines) > 0 && (m.search.active || m.search.query != "") {
		boxLines := m.renderSearchOverlay(lo.editorWidth)
		for _, l := range boxLines {
			if w := ansi.StringWidth(l); w > searchBoxW {
				searchBoxW = w
			}
		}
		startX := max(lo.editorWidth-searchBoxW, 0)
		for i, boxLine := range boxLines {
			if i < len(editorLines) {
				editorLines[i] = composeLine(editorLines[i], boxLine, startX)
			}
		}
	}

	var content string
	if m.sidebarVisible {
		panelLines := m.renderLeftPane(lo.treeWidth, lo.contentHeight)

		sepLines := make([]string, lo.contentHeight)
		for i := range sepLines {
			sepLines[i] = " \u2502 "
		}

		content = lipgloss.JoinHorizontal(
			lipgloss.Top,
			strings.Join(panelLines, "\n"),
			strings.Join(sepLines, "\n"),
			strings.Join(editorLines, "\n"),
		)
	} else {
		content = strings.Join(editorLines, "\n")
	}

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
	case m.search.active:
		cp = m.searchCursorScreenPos(lo, searchBoxW)
	case hasTab && t.inputMode:
		cp = m.commentCursorScreenPos(lo)
	case hasTab && t.diffViewData != nil:
		// No text cursor for diff view (scroll-only).
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

	xOffset := lo.lineNumWidth + blockBorderLeft
	if t.diffViewData != nil {
		// Diff view: textarea is inside a side panel.
		sideWidth := (lo.editorWidth - diffSeparatorWidth) / 2
		if t.diffInputSide == diffSideOld {
			xOffset = blockBorderLeft
		} else {
			xOffset = sideWidth + diffSeparatorWidth + blockBorderLeft
		}
	}

	return cursorPosition{
		x: lo.editorStartX + xOffset + c.X,
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
		label := tabLabel(t)
		w := ansi.StringWidth(label)
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

	if m.clearAllPending {
		n := 0
		if hasTab {
			n = len(t.comments)
		}
		sb.WriteString(styleWarning.Render(
			fmt.Sprintf("Clear %d comments? (y/n)", n)))
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
			case t.diffViewData != nil && t.diffSelecting:
				startRow, endRow := t.diffNormalizedSelection()
				n := endRow - startRow + 1
				fmt.Fprintf(&sb, "Selection: %d rows (%s)", n, t.diffSide)
				if m.statusMsg != "" {
					fmt.Fprintf(&sb, "  %s", m.statusMsg)
				}
			case t.diffViewData != nil:
				lineNum := t.diffCursorLineNum() + 1
				fmt.Fprintf(&sb, "Line %d (%s)", lineNum, t.diffSide)
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
		case m.focusPane == paneTree && m.activePanel == panelGitDiff:
			gs := m.gitState()
			if gs.cursor < len(gs.entries) {
				sb.WriteString(gs.entries[gs.cursor].name)
			}
		case m.treeCursor < len(m.fileTree):
			entry := m.fileTree[m.treeCursor]
			sb.WriteString(entry.path)
		}
	}

	return sb.String()
}

// renderLeftPane generates the left pane lines with a header and panel body.
func (m *Model) renderLeftPane(width, height int) []string {
	var header string
	if m.activePanel == panelGitDiff {
		label := m.activePanel.label() + " \u276e" + m.gitDiffMode.label(m.gitDefaultBranch) + "\u276f"
		header = renderPanelHeader(label, width, m.theme)
	} else {
		header = renderPanelHeader(m.activePanel.label(), width, m.theme)
	}
	bodyHeight := height - 1

	var body []string
	switch m.activePanel {
	case panelFiles:
		body = m.renderTree(width, bodyHeight)
	case panelGitDiff:
		body = m.renderGitPanel(width, bodyHeight)
	default:
		body = renderChangedFiles(nil, width, bodyHeight)
	}

	return append([]string{header}, body...)
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

// renderDiffEditor generates the editor pane lines for a diff tab.
// The viewport owns scrolling; we slice cached rendered lines directly.
// Cursor and selected rows are re-rendered with gutter highlights.
// Active textarea (inputMode) is overlaid after viewport slicing.
func (m *Model) renderDiffEditor(t *tab, lo layout) []string {
	width := lo.editorWidth
	height := lo.contentHeight

	t.ensureDiffContent(m.theme, width, m.search.gen)
	m.lastMapping = nil

	off := t.vp.YOffset()
	end := min(off+height, len(t.diffCachedLines))
	diffLines := make([]string, 0, height)
	if off < len(t.diffCachedLines) {
		diffLines = append(diffLines, t.diffCachedLines[off:end]...)
	}
	for len(diffLines) < height {
		diffLines = append(diffLines, padRight("", width))
	}

	// Apply cursor/selection gutter highlights to visible rows.
	if t.diffViewData != nil && len(t.diffRowVisualStarts) > 0 && m.focusPane == paneEditor {
		m.applyDiffGutterHighlights(t, diffLines, off, width)
	}

	// Overlay active textarea for diff comment input.
	if t.inputMode && t.diffViewData != nil {
		m.overlayDiffTextarea(t, diffLines, off, width, height)
	}

	return diffLines
}

// overlayDiffTextarea inserts the comment textarea block into diffLines
// at the visual position corresponding to the input's ending diff row.
func (m *Model) overlayDiffTextarea(t *tab, diffLines []string, viewOff, width, height int) {
	// Find the diff row where the input ends.
	insertRowIdx := -1
	for ri, row := range t.diffViewData.Rows {
		ln := diffRowLineNumForSide(row, t.diffInputSide)
		if ln == t.inputEnd && diffRowAvailableSide(row, t.diffInputSide) == t.diffInputSide {
			insertRowIdx = ri
			break
		}
	}
	if insertRowIdx < 0 || insertRowIdx >= len(t.diffRowVisualStarts) {
		return
	}

	// Calculate visual line position after this row's lines.
	rowVisEnd := len(t.diffCachedLines)
	if insertRowIdx+1 < len(t.diffRowVisualStarts) {
		rowVisEnd = t.diffRowVisualStarts[insertRowIdx+1]
	}

	// Also account for any comment blocks already interleaved after this row.
	// (They are between the code lines and the next row's visual start.)
	insertVisLine := rowVisEnd - viewOff
	if insertVisLine < 0 || insertVisLine > height {
		return
	}

	label := fmt.Sprintf("comment (%s)", t.diffInputSide)
	sideWidth := (width - diffSeparatorWidth) / 2
	blockBodyWidth := sideWidth - commentBlockMargin
	blockRows := renderBlock(
		t.commentInput.View(), label, blockBodyWidth, styleInput, styleBodyWhite)
	composedLines := renderDiffCommentLines(blockRows, t.diffInputSide, sideWidth, width)

	// In-place splice: shift tail down and insert block rows.
	// diffLines is exactly height long; excess lines fall off the bottom.
	blockCount := min(len(composedLines), height-insertVisLine)
	if blockCount <= 0 {
		return
	}

	// Shift tail lines right by blockCount (copy handles overlap correctly
	// when dst > src, copying from end).
	copy(diffLines[insertVisLine+blockCount:], diffLines[insertVisLine:height-blockCount])

	mapping := make([]visualEntry, blockCount)
	for i := range blockCount {
		diffLines[insertVisLine+i] = composedLines[i]
		mapping[i] = visualEntry{kind: lineKindInput}
	}

	// Record mapping for cursor positioning.
	m.lastMapping = mapping
}

// applyDiffGutterHighlights re-renders diff rows that need cursor or selection
// gutter highlighting within the visible window.
func (m *Model) applyDiffGutterHighlights(t *tab, diffLines []string, viewOff, width int) {
	if t.diffViewData == nil || len(t.diffRowVisualStarts) == 0 {
		return
	}

	ctx := newDiffSideCtx(t.diffViewData, m.theme, width)
	highlightBg := m.theme.selectionBgSeq()

	startRow, endRow := t.diffCursor, t.diffCursor
	if t.diffSelecting {
		startRow, endRow = t.diffNormalizedSelection()
	}

	viewEnd := viewOff + len(diffLines)

	// Only iterate cursor/selection rows instead of all rows.
	for rowIdx := startRow; rowIdx <= endRow && rowIdx < len(t.diffViewData.Rows); rowIdx++ {
		rowVisStart := t.diffRowVisualStarts[rowIdx]
		rowVisEnd := len(t.diffCachedLines)
		if rowIdx+1 < len(t.diffRowVisualStarts) {
			rowVisEnd = t.diffRowVisualStarts[rowIdx+1]
		}

		// Skip if entirely outside visible window.
		if rowVisEnd <= viewOff || rowVisStart >= viewEnd {
			continue
		}

		row := t.diffViewData.Rows[rowIdx]
		activeSide := diffRowAvailableSide(row, t.diffSide)
		oldCtx, newCtx := ctx, ctx
		if activeSide == diffSideOld {
			oldCtx.gutterHighlight = highlightBg
		} else {
			newCtx.gutterHighlight = highlightBg
		}
		reRendered := renderSingleDiffRow(row, t.diffOldHighlights, t.diffNewHighlights, oldCtx, newCtx, width, nil, nil)

		for j, line := range reRendered {
			visIdx := rowVisStart + j - viewOff
			if visIdx >= 0 && visIdx < len(diffLines) {
				diffLines[visIdx] = line
			}
		}
	}
}

// renderEditor generates the editor pane lines.
func (m *Model) renderEditor(lo layout) []string {
	t, hasTab := m.activeTabState()

	// Diff view dispatch: separate renderer owns the diff path.
	if hasTab && t.diffViewData != nil {
		return m.renderDiffEditor(t, lo)
	}

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
	hasSearchMatches := len(t.searchMatches) > 0
	var searchMatchBg, searchCurrentBg string
	if hasSearchMatches {
		searchMatchBg = m.theme.searchMatchBgSeq()
		searchCurrentBg = m.theme.searchCurrentBgSeq()
	}
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

		// Collect highlight ranges: search matches first, then selection (later wins).
		lineHighlights := m.searchHighlightsForLine(t, i, searchMatchBg, searchCurrentBg)
		if isSelected {
			sc, ec := selRange(i, startLine, endLine, startChar, endChar, lineContent)
			if sc < ec {
				lineHighlights = append(lineHighlights, highlightRange{start: sc, end: ec, bgSeq: selBgSeq})
			}
		}
		hasHighlights := len(lineHighlights) > 0

		bp := wrapBreakpoints(lineContent, textWidth)
		if bp != nil {
			// Per-segment rendering: split runs at wrap breakpoints,
			// then apply highlights per segment independently.
			var runs []styledRun
			if hl := t.getHighlightedLine(i); hl != nil {
				runs = hl.runs
			} else {
				runs = []styledRun{{Text: lineContent}}
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

				// Render segment content.
				var segSB strings.Builder
				if hasHighlights {
					// Clamp highlights to this segment.
					segHL := clampHighlightsToSegment(lineHighlights, wrapOff, segLen)
					if len(segHL) > 0 {
						renderStyledLineWithHighlights(&segSB, segRuns, segHL)
					} else {
						for _, r := range segRuns {
							writeStyledText(&segSB, r.ANSI, expandTabs(r.Text))
						}
					}
				} else {
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

			if hasHighlights {
				var runs []styledRun
				if hl := t.getHighlightedLine(i); hl != nil {
					runs = hl.runs
				} else {
					runs = []styledRun{{Text: lineContent}}
				}
				renderStyledLineWithHighlights(&contentSB, runs, lineHighlights)
			} else {
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
				c.Text, label, commentBodyWidth, styleComment, styleBodyWhite)
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
func formatCommentLabel(c *comment.Entry) string {
	if c.StartLine == c.EndLine {
		return "comment"
	}
	return fmt.Sprintf("comment (L%d-%d)", c.StartLine+1, c.EndLine+1)
}

// formatDiffCommentLabel returns the label for a diff comment block header.
func formatDiffCommentLabel(c *comment.Entry, side diffSide) string {
	if c.StartLine == c.EndLine {
		return fmt.Sprintf("comment (%s)", side)
	}
	return fmt.Sprintf("comment (%s, L%d-%d)", side, c.StartLine+1, c.EndLine+1)
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
	} else {
		// Label too long: truncate to fit within width.
		topLabel = string([]rune(topLabel)[:max(width-1, 1)])
	}
	topLabel += "\u256e"
	rows = append(rows, borderStyle.Render(topLabel))

	for line := range strings.SplitSeq(text, "\n") {
		content := "\u2502 " + bodyStyle.Render(line)
		content = ansi.Truncate(content, width-1, "")
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
