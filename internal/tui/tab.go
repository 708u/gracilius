package tui

import "github.com/charmbracelet/bubbles/textinput"

// tabKind distinguishes between file and diff tabs.
type tabKind int

const (
	fileTab tabKind = iota
	diffTab
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
	scrollOffset     int

	comments     map[int]string
	commentInput textinput.Model
	inputMode    bool
	inputLine    int
}

// newFileTab creates a new tab for file viewing.
func newFileTab() *tab {
	ti := textinput.New()
	ti.Placeholder = "Enter comment..."
	ti.CharLimit = 500

	return &tab{
		kind:         fileTab,
		comments:     make(map[int]string),
		commentInput: ti,
	}
}

// newDiffTab creates a new tab for diff viewing.
func newDiffTab(filePath string, lines []string) *tab {
	ti := textinput.New()
	ti.Placeholder = "Enter comment..."
	ti.CharLimit = 500

	return &tab{
		kind:         diffTab,
		filePath:     filePath,
		lines:        lines,
		comments:     make(map[int]string),
		commentInput: ti,
	}
}

// normalizedSelection returns the selection range with start <= end.
func (t *tab) normalizedSelection() (startLine, startChar, endLine, endChar int) {
	startLine, startChar = t.anchorLine, t.anchorChar
	endLine, endChar = t.cursorLine, t.cursorChar
	if startLine > endLine || (startLine == endLine && startChar > endChar) {
		startLine, endLine = endLine, startLine
		startChar, endChar = endChar, startChar
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
	t.scrollOffset = 0
	t.selecting = false
	t.comments = make(map[int]string)
	t.inputMode = false
	t.commentInput.Reset()
	t.commentInput.Blur()
}

// adjustScrollForCursor adjusts the scroll so the cursor stays visible.
func (t *tab) adjustScrollForCursor(contentHeight int) {
	margin := contentHeight / 5

	if t.cursorLine < t.scrollOffset+margin {
		t.scrollOffset = t.cursorLine - margin
	}

	if t.cursorLine >= t.scrollOffset+contentHeight-margin {
		t.scrollOffset = t.cursorLine - contentHeight + margin + 1
	}

	maxOffset := max(len(t.lines)-contentHeight, 0)
	if t.scrollOffset > maxOffset {
		t.scrollOffset = maxOffset
	}
	if t.scrollOffset < 0 {
		t.scrollOffset = 0
	}
}
