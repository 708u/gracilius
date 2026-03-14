package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	"github.com/708u/gracilius/internal/comment"
)

// tabKind distinguishes between file and diff tabs.
type tabKind int

const (
	fileTab tabKind = iota
	diffTab

	// diffScrollRatio is the golden-ratio position (≈38% from top)
	// used to place the first hunk when opening a diff view.
	diffScrollRatio = 38
)

// tab holds all per-tab state.
type tab struct {
	kind tabKind

	filePath         string
	lines            []string
	highlightedLines []highlightedLine
	cursorLine       int
	cursorChar       int
	anchorLine       int
	anchorChar       int
	selecting        bool
	lineSelect       bool
	vp               viewport.Model

	comments     []comment.Entry
	commentInput textarea.Model
	inputMode    bool
	inputStart   int
	inputEnd     int

	diff         *diffState // non-nil for diff review tabs
	diffViewData *diffData  // side-by-side diff data (nil for file tabs)

	// diff syntax highlights (old/new sides)
	diffOldHighlights []highlightedLine
	diffNewHighlights []highlightedLine
	diffOldSource     string // old-side source text for re-highlighting on theme change

	// diff render cache (invalidated on width/theme change)
	diffCachedLines     []string // pre-rendered visual lines (same as viewport content)
	diffCacheWidth      int
	diffCacheTheme      string
	diffRowVisualStarts []int // logical row → visual line offset
}

// diffState holds accept/reject callbacks for a diff review tab.
type diffState struct {
	onAccept func(string)
	onReject func()
}

func newTextarea() textarea.Model {
	ta := textarea.New()
	ta.Placeholder = "Enter comment..."
	ta.CharLimit = 2000
	ta.SetHeight(3)
	ta.ShowLineNumbers = false
	ta.Prompt = ""
	ta.SetVirtualCursor(false)
	return ta
}

// newViewport creates a viewport with keybindings disabled.
func newViewport() viewport.Model {
	vp := viewport.New()
	vp.KeyMap = viewport.KeyMap{} // disable all keybindings
	vp.MouseWheelEnabled = true
	return vp
}

// newFileTab creates a new tab for file viewing.
func newFileTab() *tab {
	return &tab{
		kind:         fileTab,
		commentInput: newTextarea(),
		vp:           newViewport(),
	}
}

// newDiffTab creates a new tab for diff viewing.
func newDiffTab(filePath string, lines []string, onAccept func(string), onReject func()) *tab {
	return &tab{
		kind:         diffTab,
		filePath:     filePath,
		lines:        lines,
		commentInput: newTextarea(),
		vp:           newViewport(),
		diff: &diffState{
			onAccept: onAccept,
			onReject: onReject,
		},
	}
}

// selectionInfo holds the selection state for MCP notification.
type selectionInfo struct {
	text      string
	startLine int
	startChar int
	endLine   int
	endChar   int
}

// getSelectionInfo returns current selection state for MCP notification.
func (t *tab) getSelectionInfo() selectionInfo {
	if t.kind == diffTab {
		return selectionInfo{}
	}
	startLine, startChar, endLine, endChar := t.normalizedSelection()
	return selectionInfo{
		text:      t.selectedTextRange(startLine, startChar, endLine, endChar),
		startLine: startLine,
		startChar: startChar,
		endLine:   endLine,
		endChar:   endChar,
	}
}

// getCursorPos returns cursor position as (line, char).
func (t *tab) getCursorPos() (int, int) {
	if t.kind == diffTab {
		return 0, 0
	}
	return t.cursorLine, t.cursorChar
}

// configureGutter sets up the LeftGutterFunc for line numbers
// with comment markers.
func (t *tab) configureGutter(digitWidth int) {
	softPad := strings.Repeat(" ", digitWidth+2)
	t.vp.LeftGutterFunc = func(ctx viewport.GutterContext) string {
		if ctx.Soft || ctx.Index >= ctx.TotalLines {
			return softPad
		}
		var sb strings.Builder
		if t.findComment(ctx.Index) >= 0 {
			sb.WriteString(styleComment.Render("\u258e"))
			fmt.Fprintf(&sb, "%*d ", digitWidth, ctx.Index+1)
		} else {
			fmt.Fprintf(&sb, " %*d ", digitWidth, ctx.Index+1)
		}
		return sb.String()
	}
}

// syncContent updates the viewport content and reconfigures the gutter.
func (t *tab) syncContent(lines []string) {
	t.vp.SetContentLines(lines)
	t.configureGutter(lineNumWidthFor(len(lines)) - 2)
}

// renderDiffContent pre-renders diff lines and updates viewport content.
// Returns the hunk visual offsets for initial scroll positioning.
func (t *tab) renderDiffContent(theme themeConfig, width int) []int {
	result := renderAllDiffLines(t.diffViewData, theme, width, t.diffOldHighlights, t.diffNewHighlights)
	t.diffCachedLines = result.lines
	t.diffRowVisualStarts = result.rowVisualStarts
	t.vp.SetContentLines(result.lines)
	t.diffCacheWidth = width
	t.diffCacheTheme = theme.name
	return result.hunkVisualOffs
}

// initDiffContent pre-renders diff lines and jumps to the first hunk.
func (t *tab) initDiffContent(theme themeConfig, width, height int) {
	if width <= diffSeparatorWidth {
		return
	}
	hunkOffs := t.renderDiffContent(theme, width)
	if len(hunkOffs) > 0 {
		offset := max(hunkOffs[0]-height*diffScrollRatio/100, 0)
		maxOff := max(len(t.diffCachedLines)-height, 0)
		offset = min(offset, maxOff)
		t.vp.SetYOffset(offset)
	}
}

// diffVisualToLogical converts a visual line offset to a logical row index
// and a sub-offset within that row.
func (t *tab) diffVisualToLogical(visualOff int) (logicalRow, subOff int) {
	starts := t.diffRowVisualStarts
	if len(starts) == 0 {
		return 0, 0
	}
	row := 0
	for i, s := range starts {
		if s > visualOff {
			break
		}
		row = i
	}
	return row, visualOff - starts[row]
}

// ensureDiffContent refreshes the diff render cache if width/theme changed.
// Anchors the viewport to the same logical diff row across re-renders.
func (t *tab) ensureDiffContent(theme themeConfig, width int) {
	if width <= diffSeparatorWidth || (t.diffCacheWidth == width && t.diffCacheTheme == theme.name) {
		return
	}
	logicalRow, subOff := t.diffVisualToLogical(t.vp.YOffset())
	t.renderDiffContent(theme, width)
	newOff := 0
	if logicalRow < len(t.diffRowVisualStarts) {
		newOff = t.diffRowVisualStarts[logicalRow]
		rowLines := len(t.diffCachedLines) - newOff
		if logicalRow+1 < len(t.diffRowVisualStarts) {
			rowLines = t.diffRowVisualStarts[logicalRow+1] - newOff
		}
		if subOff >= rowLines {
			subOff = max(rowLines-1, 0)
		}
		newOff += subOff
	}
	t.vp.SetYOffset(newOff)
}

// rejectAndClear calls onReject if set and nils the diff state.
func (t *tab) rejectAndClear() {
	if t.diff != nil && t.diff.onReject != nil {
		t.diff.onReject()
	}
	t.diff = nil
}

// findComment returns the index of the comment covering line, or -1.
func (t *tab) findComment(line int) int {
	for i := range t.comments {
		if line >= t.comments[i].StartLine && line <= t.comments[i].EndLine {
			return i
		}
	}
	return -1
}

// commentEndingAt returns the comment whose EndLine is line, or nil.
func (t *tab) commentEndingAt(line int) *comment.Entry {
	for i := range t.comments {
		if t.comments[i].EndLine == line {
			return &t.comments[i]
		}
	}
	return nil
}

// commentDisplayRows returns the visual row count for a comment block.
// (top border + content lines + bottom border)
func commentDisplayRows(text string) int {
	return strings.Count(text, "\n") + 1 + 2
}

// selectedText returns the text within the current selection range.
func (t *tab) selectedText() string {
	startLine, startChar, endLine, endChar := t.normalizedSelection()
	return t.selectedTextRange(startLine, startChar, endLine, endChar)
}

// selectedTextRange returns the text within the given range.
func (t *tab) selectedTextRange(startLine, startChar, endLine, endChar int) string {
	if startLine == endLine {
		if startLine < len(t.lines) {
			runes := []rune(t.lines[startLine])
			if startChar <= len(runes) && endChar <= len(runes) {
				return string(runes[startChar:endChar])
			}
		}
		return ""
	}

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
	return strings.Join(parts, "\n")
}

// normalizedSelection returns the selection range with start <= end.
func (t *tab) normalizedSelection() (startLine, startChar, endLine, endChar int) {
	startLine, startChar = t.anchorLine, t.anchorChar
	endLine, endChar = t.cursorLine, t.cursorChar
	if startLine > endLine || (startLine == endLine && startChar > endChar) {
		startLine, endLine = endLine, startLine
		startChar, endChar = endChar, startChar
	}
	if t.lineSelect {
		startChar = 0
		endChar = t.lineLen(endLine)
	}
	return
}

// startSelecting begins a selection if not already selecting.
func (t *tab) startSelecting() {
	if !t.selecting {
		t.selecting = true
		t.anchorLine = t.cursorLine
		t.anchorChar = t.cursorChar
	}
}

// syncAnchorToCursor synchronizes anchor to cursor when not selecting.
func (t *tab) syncAnchorToCursor() {
	if !t.selecting {
		t.anchorLine = t.cursorLine
		t.anchorChar = t.cursorChar
	}
}

// lineLen returns the rune-length of the given line.
func (t *tab) lineLen(line int) int {
	if line < 0 || line >= len(t.lines) {
		return 0
	}
	return len([]rune(t.lines[line]))
}

// getHighlightedLine returns a pointer to the highlighted line at lineIdx.
func (t *tab) getHighlightedLine(lineIdx int) *highlightedLine {
	if t.highlightedLines != nil && lineIdx >= 0 && lineIdx < len(t.highlightedLines) {
		return &t.highlightedLines[lineIdx]
	}
	return nil
}

// resetEditorState resets cursor, selection, highlight, comments, and input state.
func (t *tab) resetEditorState() {
	t.highlightedLines = nil
	t.cursorLine = 0
	t.cursorChar = 0
	t.anchorLine = 0
	t.anchorChar = 0
	t.vp.SetYOffset(0)
	t.selecting = false
	t.lineSelect = false
	t.comments = nil
	t.inputMode = false
	t.commentInput.Reset()
	t.commentInput.Blur()
}

// adjustScrollForCursor adjusts the scroll so the cursor stays visible.
func (t *tab) adjustScrollForCursor(contentHeight, textWidth int) {
	margin := contentHeight / 5

	// Cursor above visible area (logical check is sufficient)
	if t.cursorLine < t.vp.YOffset()+margin {
		t.vp.SetYOffset(t.cursorLine - margin)
	}

	// Cursor below visible area (visual-row aware)
	if t.visualRowsBetween(t.vp.YOffset(), t.cursorLine, textWidth) > contentHeight-margin {
		t.vp.SetYOffset(t.scrollOffsetFor(t.cursorLine, contentHeight-margin, textWidth))
	}

	maxOffset := t.maxScrollOffset(contentHeight, textWidth)
	if t.vp.YOffset() > maxOffset {
		t.vp.SetYOffset(maxOffset)
	}
}

// lineVisualRows returns the number of visual rows a single line occupies,
// including word-wrap rows and any comment block or active input attached to it.
func (t *tab) lineVisualRows(line, textWidth int) int {
	rows := 1
	if textWidth > 0 && line >= 0 && line < len(t.lines) {
		rows = countWraps(t.lines[line], textWidth)
	}
	if c := t.commentEndingAt(line); c != nil {
		rows += commentDisplayRows(c.Text)
	}
	if t.inputMode && line == t.inputEnd {
		rows += t.commentInput.Height() + 2
	}
	return rows
}

// visualRowsBetween returns the total visual rows from line 'from'
// to line 'to' inclusive.
func (t *tab) visualRowsBetween(from, to, textWidth int) int {
	rows := 0
	for i := from; i <= to && i < len(t.lines); i++ {
		rows += t.lineVisualRows(i, textWidth)
	}
	return rows
}

// scrollOffsetFor finds the scroll offset (logical line) where targetLine
// appears at approximately targetVisualRow from the top of the viewport.
func (t *tab) scrollOffsetFor(targetLine, targetVisualRow, textWidth int) int {
	rows := 0
	for i := targetLine; i >= 0; i-- {
		rows += t.lineVisualRows(i, textWidth)
		if rows >= targetVisualRow {
			return i
		}
	}
	return 0
}

// maxScrollOffset returns the largest valid scrollOffset (logical line)
// such that rendering from that line fills at least contentHeight visual rows.
func (t *tab) maxScrollOffset(contentHeight, textWidth int) int {
	return t.scrollOffsetFor(len(t.lines)-1, contentHeight, textWidth)
}
