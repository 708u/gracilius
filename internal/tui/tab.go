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
	vp               viewport.Model

	comments     []comment.Entry
	commentInput textarea.Model
	inputMode    bool
	inputStart   int
	inputEnd     int

	diff              *diffState  // non-nil for diff review tabs
	diffViewData      *diffData   // side-by-side diff data (nil for file tabs)
	gitDiffModeTag    gitDiffMode // diff mode for git diff tabs
	hasGitDiffModeTag bool        // true if opened from git panel
	gitDiffLabel      string      // tab label prefix (e.g. "[working]", "[vs main]")

	// diff syntax highlights (old/new sides)
	diffOldHighlights []highlightedLine
	diffNewHighlights []highlightedLine
	diffOldSource     string // old-side source text for re-highlighting on theme change

	// diff cursor/selection
	diffCursor    int      // index into diffData.rows
	diffAnchor    int      // selection anchor row index
	diffSelecting bool     // whether row-level selection is active
	diffSide      diffSide // old/new side the cursor is on

	// diff render cache (invalidated on width/theme change)
	diffCachedLines     []string // pre-rendered visual lines (same as viewport content)
	diffCacheWidth      int
	diffCacheTheme      string
	diffRowVisualStarts []int // logical row → visual line offset

	// search match cache (per-tab)
	searchMatches     []searchMatch
	diffSearchMatches []diffSearchMatch
	searchGen         int // generation for diff cache invalidation
}

// diffSide identifies which side of the diff the cursor is on.
type diffSide int

const (
	diffSideNew diffSide = iota // right (default, zero-value)
	diffSideOld                 // left
)

// String returns "old" or "new" for display purposes.
func (s diffSide) String() string {
	if s == diffSideOld {
		return "old"
	}
	return "new"
}

// diffRowAvailableSide returns the available side for a row type.
// deleted → old only, added → new only, otherwise preferred as-is.
func diffRowAvailableSide(row diffRow, preferred diffSide) diffSide {
	switch row.rowType {
	case diffRowDeleted:
		return diffSideOld
	case diffRowAdded:
		return diffSideNew
	}
	return preferred
}

// snapDiffSide adjusts diffSide to match the current row's constraints.
func (t *tab) snapDiffSide() {
	if t.diffViewData == nil || t.diffCursor >= len(t.diffViewData.rows) {
		return
	}
	t.diffSide = diffRowAvailableSide(t.diffViewData.rows[t.diffCursor], t.diffSide)
}

// diffRowTextForSide returns the text for the given side of a diff row.
func diffRowTextForSide(row diffRow, side diffSide) string {
	if side == diffSideOld {
		if row.oldLineNum > 0 {
			return row.oldText
		}
		return row.newText
	}
	if row.newLineNum > 0 {
		return row.newText
	}
	return row.oldText
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
		return t.getDiffSelectionInfo()
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
		return t.diffCursorLineNum(), 0
	}
	return t.cursorLine, t.cursorChar
}

// diffCursorLineNum returns the 0-based line number for the current diff cursor row,
// respecting the current diffSide.
func (t *tab) diffCursorLineNum() int {
	if t.diffViewData == nil || t.diffCursor >= len(t.diffViewData.rows) {
		return 0
	}
	return diffRowLineNumForSide(t.diffViewData.rows[t.diffCursor], t.diffSide)
}

// syncDiffAnchor synchronizes the diff anchor to the cursor when not selecting.
func (t *tab) syncDiffAnchor() {
	if !t.diffSelecting {
		t.diffAnchor = t.diffCursor
	}
}

// diffNormalizedSelection returns the selection range with startRow <= endRow.
func (t *tab) diffNormalizedSelection() (startRow, endRow int) {
	startRow, endRow = t.diffAnchor, t.diffCursor
	if startRow > endRow {
		startRow, endRow = endRow, startRow
	}
	return
}

// diffRowLineNumForSide returns the 0-based line number for a diff row
// respecting the given side. Falls back to the other side if the
// requested side has no line number.
func diffRowLineNumForSide(row diffRow, side diffSide) int {
	if side == diffSideOld {
		if row.oldLineNum > 0 {
			return row.oldLineNum - 1
		}
		if row.newLineNum > 0 {
			return row.newLineNum - 1
		}
		return 0
	}
	if row.newLineNum > 0 {
		return row.newLineNum - 1
	}
	if row.oldLineNum > 0 {
		return row.oldLineNum - 1
	}
	return 0
}

// diffEffectiveRange returns the effective row range for selection/cursor.
// When not selecting, both values equal diffCursor.
func (t *tab) diffEffectiveRange() (startRow, endRow int) {
	if t.diffSelecting {
		return t.diffNormalizedSelection()
	}
	return t.diffCursor, t.diffCursor
}

// diffSelectedText returns the text of the selected diff rows for the current side.
func (t *tab) diffSelectedText() string {
	if t.diffViewData == nil {
		return ""
	}
	startRow, endRow := t.diffEffectiveRange()
	var parts []string
	for i := startRow; i <= endRow && i < len(t.diffViewData.rows); i++ {
		row := t.diffViewData.rows[i]
		side := diffRowAvailableSide(row, t.diffSide)
		parts = append(parts, diffRowTextForSide(row, side))
	}
	return strings.Join(parts, "\n")
}

// getDiffSelectionInfo returns selection info for MCP notification in diff mode.
func (t *tab) getDiffSelectionInfo() selectionInfo {
	if t.diffViewData == nil || len(t.diffViewData.rows) == 0 {
		return selectionInfo{}
	}
	startRow, endRow := t.diffEffectiveRange()
	startRowData := t.diffViewData.rows[startRow]
	startSide := diffRowAvailableSide(startRowData, t.diffSide)
	startLineNum := diffRowLineNumForSide(startRowData, startSide)
	endRowData := t.diffViewData.rows[min(endRow, len(t.diffViewData.rows)-1)]
	endSide := diffRowAvailableSide(endRowData, t.diffSide)
	endLineNum := diffRowLineNumForSide(endRowData, endSide)
	endText := diffRowTextForSide(endRowData, endSide)
	var parts []string
	for i := startRow; i <= endRow && i < len(t.diffViewData.rows); i++ {
		row := t.diffViewData.rows[i]
		side := diffRowAvailableSide(row, t.diffSide)
		parts = append(parts, diffRowTextForSide(row, side))
	}
	return selectionInfo{
		text:      strings.Join(parts, "\n"),
		startLine: startLineNum,
		startChar: 0,
		endLine:   endLineNum,
		endChar:   len([]rune(endText)),
	}
}

// adjustDiffScrollForCursor adjusts the viewport so the diff cursor row is visible.
func (t *tab) adjustDiffScrollForCursor(contentHeight int) {
	if t.diffViewData == nil || len(t.diffRowVisualStarts) == 0 {
		return
	}
	cursor := t.diffCursor
	if cursor >= len(t.diffRowVisualStarts) {
		return
	}
	margin := contentHeight / 5
	visLine := t.diffRowVisualStarts[cursor]
	off := t.vp.YOffset()
	if visLine < off+margin {
		t.vp.SetYOffset(max(visLine-margin, 0))
	}
	// Calculate visual rows for cursor row.
	nextVis := len(t.diffCachedLines)
	if cursor+1 < len(t.diffRowVisualStarts) {
		nextVis = t.diffRowVisualStarts[cursor+1]
	}
	cursorBottom := nextVis - 1
	if cursorBottom >= off+contentHeight-margin {
		t.vp.SetYOffset(max(cursorBottom-contentHeight+margin+1, 0))
	}
}

// diffVisualLineToRow converts a visual line index to a diff row index
// using binary search on diffRowVisStart.
func (t *tab) diffVisualLineToRow(visualLine int) int {
	if len(t.diffRowVisualStarts) == 0 {
		return 0
	}
	lo, hi := 0, len(t.diffRowVisualStarts)-1
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if t.diffRowVisualStarts[mid] <= visualLine {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return lo
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
	result := renderAllDiffLines(t.diffViewData, theme, width, t.diffOldHighlights, t.diffNewHighlights, t.diffSearchMatches)
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
	// Set cursor to first changed row.
	if t.diffViewData != nil {
		for i, row := range t.diffViewData.rows {
			if row.rowType != diffRowUnchanged {
				t.diffCursor = i
				t.diffAnchor = i
				break
			}
		}
		t.snapDiffSide()
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

// ensureDiffContent refreshes the diff render cache if width/theme/search changed.
// Anchors the viewport to the same logical diff row across re-renders.
func (t *tab) ensureDiffContent(theme themeConfig, width int, searchGen int) {
	if width <= diffSeparatorWidth ||
		(t.diffCacheWidth == width && t.diffCacheTheme == theme.name && t.searchGen == searchGen) {
		return
	}
	t.searchGen = searchGen
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
// Selection is always line-granular: startChar is 0 and endChar is the
// full length of the end line.
func (t *tab) normalizedSelection() (startLine, startChar, endLine, endChar int) {
	startLine = t.anchorLine
	endLine = t.cursorLine
	if startLine > endLine {
		startLine, endLine = endLine, startLine
	}
	startChar = 0
	endChar = t.lineLen(endLine)
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
	t.comments = nil
	t.inputMode = false
	t.commentInput.Reset()
	t.commentInput.Blur()
	t.searchMatches = nil
	t.diffSearchMatches = nil
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
