package tui

import (
	"fmt"
	"slices"
	"strings"
	"unicode"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/708u/gracilius/internal/diff"
	"github.com/708u/gracilius/internal/tui/render"
	"github.com/charmbracelet/x/ansi"
)

// searchMatch represents a single match in a file tab.
type searchMatch struct {
	line      int // 0-based logical line
	startChar int // rune offset (inclusive)
	endChar   int // rune offset (exclusive)
}

// diffSearchMatch represents a single match in a diff tab.
type diffSearchMatch struct {
	rowIdx    int // index into diff.Data.Rows
	isOld     bool
	startChar int // rune offset (inclusive)
	endChar   int // rune offset (exclusive)
}

// searchState holds the search UI state (one per Model).
type searchState struct {
	active       bool
	input        textinput.Model
	query        string // confirmed query (persists after Enter)
	currentMatch int
	savedLine    int // cursor position before search (for Esc restore)
	savedChar    int
	savedScroll  int
	gen          int // generation counter for cache invalidation
}

func newSearchState() searchState {
	ti := textinput.New()
	ti.Placeholder = "Search"
	ti.Prompt = ""
	ti.CharLimit = 500
	ti.SetVirtualCursor(false)
	return searchState{input: ti}
}

// currentSearchQuery returns the active search query: the textinput value
// during input mode, or the confirmed query otherwise.
func (m *Model) currentSearchQuery() string {
	if m.search.active {
		return m.search.input.Value()
	}
	return m.search.query
}

// isSmartCaseSensitive returns true if query contains any uppercase letter.
func isSmartCaseSensitive(query string) bool {
	for _, r := range query {
		if unicode.IsUpper(r) {
			return true
		}
	}
	return false
}

// findSubstringPositions returns the rune-offset start positions of all
// occurrences of queryRunes in lineRunes.
func findSubstringPositions(lineRunes, queryRunes []rune) []int {
	queryLen := len(queryRunes)
	var positions []int
	for j := 0; j <= len(lineRunes)-queryLen; j++ {
		if slices.Equal(lineRunes[j:j+queryLen], queryRunes) {
			positions = append(positions, j)
		}
	}
	return positions
}

// normalizeForSearch returns the rune slice of text, lowercased if not case-sensitive.
func normalizeForSearch(text string, caseSensitive bool) []rune {
	if !caseSensitive {
		return []rune(strings.ToLower(text))
	}
	return []rune(text)
}

// computeSearchMatches finds all substring matches of query in lines.
// Uses smartcase: case-insensitive unless query contains uppercase.
func computeSearchMatches(lines []string, query string) []searchMatch {
	if query == "" {
		return nil
	}
	caseSensitive := isSmartCaseSensitive(query)
	queryRunes := normalizeForSearch(query, caseSensitive)
	queryLen := len(queryRunes)

	var matches []searchMatch
	for i, line := range lines {
		lineRunes := normalizeForSearch(line, caseSensitive)
		for _, j := range findSubstringPositions(lineRunes, queryRunes) {
			matches = append(matches, searchMatch{
				line:      i,
				startChar: j,
				endChar:   j + queryLen,
			})
		}
	}
	return matches
}

// computeDiffSearchMatches finds matches in both old and new sides of diff rows.
func computeDiffSearchMatches(data *diff.Data, query string) []diffSearchMatch {
	if query == "" || data == nil {
		return nil
	}
	caseSensitive := isSmartCaseSensitive(query)
	queryRunes := normalizeForSearch(query, caseSensitive)
	queryLen := len(queryRunes)

	var matches []diffSearchMatch
	for i, row := range data.Rows {
		if row.OldLineNum > 0 {
			for _, j := range findSubstringPositions(normalizeForSearch(row.OldText, caseSensitive), queryRunes) {
				matches = append(matches, diffSearchMatch{
					rowIdx: i, isOld: true, startChar: j, endChar: j + queryLen,
				})
			}
		}
		if row.NewLineNum > 0 {
			for _, j := range findSubstringPositions(normalizeForSearch(row.NewText, caseSensitive), queryRunes) {
				matches = append(matches, diffSearchMatch{
					rowIdx: i, isOld: false, startChar: j, endChar: j + queryLen,
				})
			}
		}
	}
	return matches
}

// startSearch enters search mode, saving cursor/scroll state.
// If a previous query exists, it is preset and fully selected
// so typing immediately replaces it.
func (m *Model) startSearch() {
	t, ok := m.activeTabState()
	if !ok {
		return
	}
	m.search.active = true
	m.search.savedLine = t.cursorLine
	m.search.savedChar = t.cursorChar
	m.search.savedScroll = t.vp.YOffset()
	if m.search.query != "" {
		m.search.input.SetValue(m.search.query)
		m.search.input.CursorEnd()
	} else {
		m.search.input.Reset()
	}
	m.search.input.Focus()
	m.focusPane = paneEditor
}

// confirmSearch locks in the current query and exits search input mode.
// If the query is empty or has no matches, it cancels the search instead.
func (m *Model) confirmSearch() {
	m.search.query = m.search.input.Value()
	m.search.active = false
	m.search.input.Blur()
	if m.search.query == "" || m.searchMatchCount() == 0 {
		m.cancelSearch()
	}
}

// cancelSearch restores cursor/scroll and clears matches.
func (m *Model) cancelSearch() {
	m.search.active = false
	m.search.input.Blur()
	m.search.query = ""
	m.clearSearchMatches()

	if t, ok := m.activeTabState(); ok {
		t.cursorLine = m.search.savedLine
		t.cursorChar = m.search.savedChar
		t.vp.SetYOffset(m.search.savedScroll)
	}
}

// clearSearchMatches clears match data from the active tab.
func (m *Model) clearSearchMatches() {
	if t, ok := m.activeTabState(); ok {
		t.searchMatches = nil
		t.diffSearchMatches = nil
	}
	m.search.currentMatch = 0
	m.search.gen++
}

// refreshSearchMatches recomputes matches for the active tab.
func (m *Model) refreshSearchMatches() {
	t, ok := m.activeTabState()
	if !ok {
		return
	}
	query := m.currentSearchQuery()
	if query == "" {
		t.searchMatches = nil
		t.diffSearchMatches = nil
		m.search.gen++
		return
	}

	if t.diffViewData != nil {
		t.diffSearchMatches = computeDiffSearchMatches(t.diffViewData, query)
		t.searchMatches = nil
	} else {
		t.searchMatches = computeSearchMatches(t.lines, query)
		t.diffSearchMatches = nil
	}
	m.search.gen++

	// Clamp currentMatch
	total := m.searchMatchCount()
	if total == 0 {
		m.search.currentMatch = 0
	} else if m.search.currentMatch >= total {
		m.search.currentMatch = 0
	}
}

// searchMatchCount returns the total match count for the active tab.
func (m *Model) searchMatchCount() int {
	t, ok := m.activeTabState()
	if !ok {
		return 0
	}
	if t.diffViewData != nil {
		return len(t.diffSearchMatches)
	}
	return len(t.searchMatches)
}

// jumpToMatch moves cursor/scroll to the match at idx.
func (m *Model) jumpToMatch(idx int) {
	t, ok := m.activeTabState()
	if !ok {
		return
	}

	if t.diffViewData != nil {
		// For diff tabs, scroll to the row containing the match.
		if idx < 0 || idx >= len(t.diffSearchMatches) {
			return
		}
		m.search.currentMatch = idx
		// Scroll to make the matched row visible.
		// We approximate by using the row index in diffCachedLines.
		// A simple approach: find visual offset of the row.
		dm := t.diffSearchMatches[idx]
		visualOff := dm.rowIdx
		lo := m.computeLayout()
		if visualOff < t.vp.YOffset() || visualOff >= t.vp.YOffset()+lo.contentHeight {
			t.vp.SetYOffset(max(visualOff-lo.contentHeight/3, 0))
		}
		return
	}

	if idx < 0 || idx >= len(t.searchMatches) {
		return
	}
	m.search.currentMatch = idx
	match := t.searchMatches[idx]
	t.cursorLine = match.line
	t.cursorChar = match.startChar
	t.syncAnchorToCursor()
	m.adjustScroll()
}

// nextMatch moves to the next search match.
func (m *Model) nextMatch() {
	total := m.searchMatchCount()
	if total == 0 {
		return
	}
	m.jumpToMatch((m.search.currentMatch + 1) % total)
}

// prevMatch moves to the previous search match.
func (m *Model) prevMatch() {
	total := m.searchMatchCount()
	if total == 0 {
		return
	}
	m.jumpToMatch((m.search.currentMatch - 1 + total) % total)
}

// handleKeySearch handles key events during search input mode.
func (m *Model) handleKeySearch(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Cancel):
		m.cancelSearch()
		return m, nil
	case msg.Code == tea.KeyEnter:
		m.confirmSearch()
		return m, nil
	default:
		prevVal := m.search.input.Value()
		var cmd tea.Cmd
		m.search.input, cmd = m.search.input.Update(msg)
		if m.search.input.Value() != prevVal {
			m.refreshSearchMatches()
			// Jump to first match from cursor position during incremental search.
			if total := m.searchMatchCount(); total > 0 {
				m.jumpToFirstMatchFromCursor()
			}
		}
		return m, cmd
	}
}

// Search overlay geometry.
const (
	searchOverlayWidthPercent = 40 // percentage of editor width
	searchOverlayMinWidth     = 24
	searchOverlayMaxWidth     = 50
	searchOverlayCounterW     = 8 // fixed width reserved for counter
)

// searchOverlayInnerWidth returns the inner content width of the search
// overlay box (outer width minus 2 border chars and 2 padding spaces).
func searchOverlayInnerWidth(boxW int) int {
	return max(boxW-4, 1)
}

// searchOverlayInputWidth returns the text input width inside the search
// overlay box of the given outer width.
func searchOverlayInputWidth(boxW int) int {
	return max(searchOverlayInnerWidth(boxW)-searchOverlayCounterW, 1)
}

// renderSearchOverlay renders the search bar as a bordered box (3 lines)
// and returns its lines for overlaying on the editor.
func (m *Model) renderSearchOverlay(editorWidth int) []string {
	total := m.searchMatchCount()
	borderFg := lipgloss.Color(m.theme.TabActiveBorder)
	borderStyle := lipgloss.NewStyle().Foreground(borderFg)

	// Compute fixed box width (outer, including borders).
	boxW := editorWidth * searchOverlayWidthPercent / 100
	boxW = min(max(min(boxW, searchOverlayMaxWidth), searchOverlayMinWidth), editorWidth)
	inputW := searchOverlayInputWidth(boxW)

	// Render input.
	m.search.input.SetWidth(inputW)
	var inputView string
	if m.search.active {
		inputView = m.search.input.View()
	} else {
		inputView = m.search.query
	}
	// Truncate to inputW to guarantee single-line.
	inputView = ansi.Truncate(inputView, inputW, "")

	// Build counter string.
	query := m.currentSearchQuery()
	var counter string
	if query != "" {
		if total > 0 {
			counter = fmt.Sprintf("%d/%d", m.search.currentMatch+1, total)
		} else {
			counter = "-/0"
		}
	}

	// Right-align counter within its fixed-width zone.
	counterRendered := counter
	if pad := searchOverlayCounterW - ansi.StringWidth(counter); pad > 0 {
		counterRendered = strings.Repeat(" ", pad) + counter
	}

	// Compose the single content line: input padded to inputW, then counter.
	inputVisualW := ansi.StringWidth(inputView)
	if inputVisualW < inputW {
		inputView += strings.Repeat(" ", inputW-inputVisualW)
	}
	contentLine := inputView + counterRendered

	// Ensure contentLine is exactly the inner width (truncate if over, pad if under).
	innerW := searchOverlayInnerWidth(boxW)
	contentVisualW := ansi.StringWidth(contentLine)
	if contentVisualW > innerW {
		contentLine = ansi.Truncate(contentLine, innerW, "")
	} else if contentVisualW < innerW {
		contentLine += strings.Repeat(" ", innerW-contentVisualW)
	}

	// Build 3 lines manually: top border, content, bottom border.
	top := borderStyle.Render("\u256d" + strings.Repeat("\u2500", boxW-2) + "\u256e")
	mid := borderStyle.Render("\u2502") + " " + contentLine + " " + borderStyle.Render("\u2502")
	bot := borderStyle.Render("\u2570" + strings.Repeat("\u2500", boxW-2) + "\u256f")

	return []string{top, mid, bot}
}

// searchCursorScreenPos returns the cursor position within the search overlay
// at the top-right of the editor area. lo and boxW must be pre-computed by
// the caller to avoid redundant work.
func (m *Model) searchCursorScreenPos(lo layout, boxW int) cursorPosition {
	if !m.search.input.Focused() {
		return cursorPosition{}
	}
	// Overlay is right-aligned within the editor area.
	startX := lo.editorStartX + lo.editorWidth - boxW
	// Content is on the second line (after top border).
	y := contentStartY + 1
	// Compute display width instead of using Cursor().X, which returns
	// a rune index and is incorrect for wide characters (CJK).
	// See: https://github.com/charmbracelet/bubbles/issues/906
	val := m.search.input.Value()
	pos := m.search.input.Position()
	cursorCol := min(render.DisplayWidthRange(val, 0, pos), searchOverlayInputWidth(boxW))
	x := startX + 1 + 1 + cursorCol
	return cursorPosition{x: x, y: y}
}

// searchHighlightsForLine returns highlight ranges for search matches on a given line.
func (m *Model) searchHighlightsForLine(t *tab, line int, matchBg, currentBg string) []render.HighlightRange {
	if len(t.searchMatches) == 0 {
		return nil
	}
	var ranges []render.HighlightRange
	for i, match := range t.searchMatches {
		if match.line != line {
			continue
		}
		bg := matchBg
		if i == m.search.currentMatch {
			bg = currentBg
		}
		ranges = append(ranges, render.HighlightRange{
			Start: match.startChar,
			End:   match.endChar,
			BgSeq: bg,
		})
	}
	return ranges
}

// jumpToFirstMatchFromCursor finds the nearest match at or after the saved cursor position.
func (m *Model) jumpToFirstMatchFromCursor() {
	t, ok := m.activeTabState()
	if !ok {
		return
	}

	if t.diffViewData != nil {
		if len(t.diffSearchMatches) > 0 {
			m.jumpToMatch(0)
		}
		return
	}

	savedLine := m.search.savedLine
	savedChar := m.search.savedChar
	for i, match := range t.searchMatches {
		if match.line > savedLine || (match.line == savedLine && match.startChar >= savedChar) {
			m.jumpToMatch(i)
			return
		}
	}
	// Wrap around to first match.
	m.jumpToMatch(0)
}
