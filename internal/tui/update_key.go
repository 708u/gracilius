package tui

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/708u/gracilius/internal/comment"
	"github.com/708u/gracilius/internal/diff"
	"github.com/google/uuid"
)

// handleKeyPress dispatches key press events to the appropriate handler.
func (m *Model) handleKeyPress(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	t, hasTab := m.activeTabState()

	if key.Matches(msg, m.keys.Quit) {
		if m.quitPending {
			return m, tea.Quit
		}
		m.quitPending = true
		return m, tea.Tick(quitTimeout, func(time.Time) tea.Msg {
			return quitTimeoutMsg{}
		})
	}

	if m.clearAllPending {
		m.clearAllPending = false
		if key.Matches(msg, m.keys.Confirm) {
			if hasTab && t.kind == diffTab {
				t.comments = nil
				t.diffCommentSides = nil
				t.diffCacheWidth = 0
			} else if hasTab && t.filePath != "" {
				if err := m.commentRepo.DeleteByFile(t.filePath); err != nil {
					log.Printf("Failed to clear comments from store: %v", err)
				}
			}
		}
		return m, nil
	}

	if hasTab && t.inputMode {
		return m.handleKeyInputMode(t, msg)
	}

	if m.search.active {
		return m.handleKeySearch(msg)
	}

	if m.gPending {
		m.gPending = false
		if key.Matches(msg, m.keys.GoTop) {
			switch {
			case m.focusPane == paneTree && m.activePanel == panelGitDiff:
				gs := m.gitState()
				gs.cursor = firstGitEntryIdx(gs.visualRows)
			case m.focusPane == paneTree:
				m.treeCursor = 0
			case hasTab && t.diffViewData != nil:
				t.diffCursor = 0
				t.syncDiffAnchor()
				m.notifySelectionChanged()
			case hasTab && len(t.lines) > 0:
				t.cursorLine = 0
				t.cursorChar = 0
				t.syncAnchorToCursor()
				m.notifySelectionChanged()
			}
			m.adjustScroll()
			return m, nil
		}
	}

	if m.openFile.active {
		return m.handleKeyOpenFile(msg)
	}

	return m.handleKeyNormal(msg)
}

// handleKeyInputMode handles key events during comment input mode.
func (m *Model) handleKeyInputMode(t *tab, msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	isSubmit := key.Matches(msg, m.keys.CommentSubmit)
	if m.enhancedKeyboard && msg.Code == tea.KeyEnter && !msg.Mod.Contains(tea.ModShift) {
		isSubmit = true
	}

	switch {
	case key.Matches(msg, m.keys.Cancel):
		t.inputMode = false
		t.commentInput.Reset()
		t.commentInput.Blur()
	case isSubmit:
		m.submitComment(t)
		t.inputMode = false
		t.commentInput.Reset()
		t.commentInput.Blur()
	default:
		linesBefore := strings.Count(t.commentInput.Value(), "\n") + 1
		if msg.Code == tea.KeyEnter && linesBefore >= t.commentInput.Height() {
			t.commentInput.SetHeight(t.commentInput.Height() + 1)
		}
		t.commentInput, cmd = t.commentInput.Update(msg)
		linesAfter := strings.Count(t.commentInput.Value(), "\n") + 1
		if linesAfter < linesBefore && t.commentInput.Height() > 3 {
			t.commentInput.SetHeight(max(linesAfter, 3))
		}
		return m, cmd
	}
	return m, nil
}

// captureSnippet returns the text of lines[startLine:endLine+1].
func (t *tab) captureSnippet(startLine, endLine int) string {
	if startLine < 0 || startLine >= len(t.lines) {
		return ""
	}
	end := min(endLine+1, len(t.lines))
	return strings.Join(t.lines[startLine:end], "\n")
}

// submitComment persists the current comment input to the store.
func (m *Model) submitComment(t *tab) {
	if t.kind == diffTab {
		m.submitDiffComment(t)
		return
	}

	val := t.commentInput.Value()
	idx := t.findComment(t.inputStart)
	var oldID string
	if idx >= 0 {
		oldID = t.comments[idx].ID
	}

	if val == "" && oldID != "" {
		if err := m.commentRepo.Delete(oldID); err != nil {
			log.Printf("Failed to delete comment: %v", err)
		}
		return
	}
	if val == "" {
		return
	}

	id, err := uuid.NewV7()
	if err != nil {
		log.Printf("Failed to generate UUID: %v", err)
	}
	m.notifyComment(t.inputStart, t.inputEnd, val)
	sc := comment.Entry{
		ID:        id.String(),
		FilePath:  t.filePath,
		StartLine: t.inputStart,
		EndLine:   t.inputEnd,
		Text:      val,
		Snippet:   t.captureSnippet(t.inputStart, t.inputEnd),
		CreatedAt: time.Now(),
	}
	if oldID != "" {
		if err := m.commentRepo.Replace(oldID, sc); err != nil {
			log.Printf("Failed to update comment: %v", err)
		}
		return
	}
	if err := m.commentRepo.Add(sc); err != nil {
		log.Printf("Failed to persist comment: %v", err)
	}
}

// submitDiffComment handles comment submission for diff tabs (in-memory only).
func (m *Model) submitDiffComment(t *tab) {
	val := t.commentInput.Value()
	side := t.diffInputSide
	idx := t.findDiffComment(t.inputStart, side)

	if val == "" && idx >= 0 {
		t.comments = append(t.comments[:idx], t.comments[idx+1:]...)
		t.diffCommentSides = append(t.diffCommentSides[:idx], t.diffCommentSides[idx+1:]...)
		t.diffCacheWidth = 0
		return
	}
	if val == "" {
		return
	}

	id, err := uuid.NewV7()
	if err != nil {
		log.Printf("Failed to generate UUID: %v", err)
	}
	m.notifyDiffComment(side, t.inputStart, t.inputEnd, val)
	sc := comment.Entry{
		ID:        id.String(),
		FilePath:  t.filePath,
		StartLine: t.inputStart,
		EndLine:   t.inputEnd,
		Text:      val,
		Snippet:   t.diffCaptureSnippet(t.inputStart, t.inputEnd, side),
		CreatedAt: time.Now(),
	}
	if idx >= 0 {
		t.comments[idx] = sc
	} else {
		t.comments = append(t.comments, sc)
		t.diffCommentSides = append(t.diffCommentSides, side)
	}
	t.diffCacheWidth = 0
}

// handleKeyOpenFile handles key events when the open-file overlay is active.
func (m *Model) handleKeyOpenFile(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Cancel):
		m.openFile.close()
		return m, nil
	case msg.Code == tea.KeyEnter:
		if p := m.openFile.selectedPath(); p != "" {
			absPath := filepath.Join(m.rootDir, p)
			m.openFile.close()
			m.openFileByPath(absPath)
		}
		return m, nil
	default:
		cmd := m.openFile.update(msg)
		return m, cmd
	}
}

// handleDiffKeyNormal handles key events when a diff tab has editor focus.
// It returns (model, cmd, handled). When handled is true the caller should
// return immediately; when false the event falls through to handleKeyNormal.
func (m *Model) handleDiffKeyNormal(t *tab, msg tea.KeyPressMsg) (tea.Model, tea.Cmd, bool) {
	switch {
	// Accept/reject diff (only when a blocking diff responder exists).
	case key.Matches(msg, m.keys.AcceptDiff):
		if t.diff != nil {
			contents := strings.Join(t.lines, "\n")
			t.diff.onAccept(contents)
			t.diff = nil
			m.closeTab(m.activeTab)
			return m, nil, true
		}
	case key.Matches(msg, m.keys.RejectDiff):
		if t.diff != nil {
			t.rejectAndClear()
			m.closeTab(m.activeTab)
			return m, nil, true
		}

	// Cursor movement.
	case key.Matches(msg, m.keys.Up):
		if t.diffViewData != nil && t.diffCursor > 0 {
			t.diffCursor--
			t.syncDiffAnchor()
			cmd := m.scheduleSelectionNotify()
			return m, cmd, true
		}
		return m, nil, true
	case key.Matches(msg, m.keys.Down):
		if t.diffViewData != nil && t.diffCursor < len(t.diffViewData.Rows)-1 {
			t.diffCursor++
			t.syncDiffAnchor()
			cmd := m.scheduleSelectionNotify()
			return m, cmd, true
		}
		return m, nil, true
	case key.Matches(msg, m.keys.GoBottom):
		if t.diffViewData != nil && len(t.diffViewData.Rows) > 0 {
			t.diffCursor = len(t.diffViewData.Rows) - 1
			t.syncDiffAnchor()
			cmd := m.scheduleSelectionNotify()
			return m, cmd, true
		}
		return m, nil, true

	// Selection toggle.
	case key.Matches(msg, m.keys.Select):
		if t.diffViewData != nil {
			if t.diffSelecting {
				t.diffSelecting = false
				m.notifyClearSelection()
			} else {
				t.diffSelecting = true
				t.diffAnchor = t.diffCursor
				m.notifySelectionChanged()
			}
		}
		return m, nil, true

	// Blank-line boundary jump (same as file tab {/}).
	case key.Matches(msg, m.keys.BlockUp):
		m.diffJumpBlankLine(t, -1)
		cmd := m.scheduleSelectionNotify()
		return m, cmd, true
	case key.Matches(msg, m.keys.BlockDown):
		m.diffJumpBlankLine(t, 1)
		cmd := m.scheduleSelectionNotify()
		return m, cmd, true

	// Change block jump ([/]).
	case key.Matches(msg, m.keys.ChangeUp):
		m.diffJumpChange(t, -1)
		cmd := m.scheduleSelectionNotify()
		return m, cmd, true
	case key.Matches(msg, m.keys.ChangeDown):
		m.diffJumpChange(t, 1)
		cmd := m.scheduleSelectionNotify()
		return m, cmd, true

	// Copy selection.
	case key.Matches(msg, m.keys.Copy):
		if t.diffViewData != nil && t.diffSelecting {
			text := t.diffSelectedText()
			n := strings.Count(text, "\n") + 1
			m.statusMsg = fmt.Sprintf("Copied %d lines", n)
			return m, tea.Batch(
				tea.SetClipboard(text),
				statusTickCmd(),
			), true
		}
		return m, nil, true

	// Cancel selection.
	case key.Matches(msg, m.keys.Cancel):
		if t.diffSelecting {
			t.diffSelecting = false
			m.notifyClearSelection()
			return m, nil, true
		}

	// Left/Right: switch diff side on unchanged/modified rows.
	case key.Matches(msg, m.keys.Left):
		m.setDiffSide(t, diffSideOld)
		return m, nil, true
	case key.Matches(msg, m.keys.Right):
		m.setDiffSide(t, diffSideNew)
		return m, nil, true

	// Comment on diff view.
	case key.Matches(msg, m.keys.Comment):
		if t.diffViewData != nil {
			t.inputMode = true
			t.diffInputSide = diffRowAvailableSide(
				t.diffViewData.Rows[t.diffCursor], t.diffSide)

			if t.diffSelecting {
				startRow, endRow := t.diffNormalizedSelection()
				t.inputStart = diffRowLineNumForSide(
					t.diffViewData.Rows[startRow], t.diffInputSide)
				t.inputEnd = diffRowLineNumForSide(
					t.diffViewData.Rows[endRow], t.diffInputSide)
				t.diffSelecting = false
			} else {
				ln := diffRowLineNumForSide(
					t.diffViewData.Rows[t.diffCursor], t.diffInputSide)
				t.inputStart = ln
				t.inputEnd = ln
			}

			lo := m.computeLayout()
			sideWidth := (lo.editorWidth - diffSeparatorWidth) / 2
			t.commentInput.SetWidth(
				sideWidth - commentBlockMargin - commentBorderChars)
			t.commentInput.SetHeight(3)
			if m.enhancedKeyboard {
				t.commentInput.KeyMap.InsertNewline = key.NewBinding(
					key.WithKeys("shift+enter"),
				)
			}
			t.commentInput.Reset()
			if idx := t.findDiffComment(t.inputStart, t.diffInputSide); idx >= 0 {
				t.inputStart = t.comments[idx].StartLine
				t.inputEnd = t.comments[idx].EndLine
				t.commentInput.SetValue(t.comments[idx].Text)
			}
			t.commentInput.Focus()
		}
		return m, nil, true
	}

	return m, nil, false
}

// setDiffSide switches the diff side. On rows that have both sides
// (unchanged/modified), the side is switched in place. On single-sided
// rows (added/deleted), the cursor jumps to the nearest row that has
// content on the requested side.
func (m *Model) setDiffSide(t *tab, side diffSide) {
	if t.diffViewData == nil || t.diffCursor >= len(t.diffViewData.Rows) {
		return
	}
	row := t.diffViewData.Rows[t.diffCursor]
	// Row has the requested side → switch in place.
	if diffRowAvailableSide(row, side) == side {
		if t.diffSide == side {
			return // already on requested side
		}
		t.diffSide = side
		m.notifySelectionChanged()
		return
	}
	// Current row is single-sided without the requested side.
	// Jump to nearest row that has it.
	target := findNearestRowForSide(t.diffViewData.Rows, t.diffCursor, side)
	if target < 0 {
		return
	}
	t.diffCursor = target
	t.diffSide = side
	t.syncDiffAnchor()
	m.notifySelectionChanged()
}

// diffJumpBlankLine moves the diff cursor to the next/previous blank-line
// boundary, mirroring moveToParagraphBoundary for file tabs.
func (m *Model) diffJumpBlankLine(t *tab, dir int) {
	if t.diffViewData == nil {
		return
	}
	rows := t.diffViewData.Rows
	cur := t.diffCursor
	last := len(rows) - 1

	inBounds := func(i int) bool {
		if dir > 0 {
			return i < last
		}
		return i > 0
	}

	isBlank := func(i int) bool {
		side := diffRowAvailableSide(rows[i], t.diffSide)
		return isBlankLine(diffRowTextForSide(rows[i], side))
	}

	line := cur
	if inBounds(line) {
		line += dir
		for inBounds(line) && isBlank(line) {
			line += dir
		}
		for inBounds(line) && !isBlank(line) {
			line += dir
		}
	}

	if line != cur {
		t.diffCursor = line
		t.snapDiffSide()
		t.syncDiffAnchor()
	}
}

// diffJumpChange moves the diff cursor stepwise through change blocks,
// landing only on rows that have content on the current diffSide.
//
// For dir > 0 (]):
//   - Inside a change block: jump to the last matching row in this block.
//   - At the last matching row or on an unchanged row: jump to the next
//     block's first matching row.
//
// For dir < 0 ([):
//   - Inside a change block: jump to the first matching row in this block.
//   - At the first matching row or on an unchanged row: jump to the
//     previous block's last matching row.
func (m *Model) diffJumpChange(t *tab, dir int) {
	if t.diffViewData == nil {
		return
	}
	rows := t.diffViewData.Rows
	if len(rows) == 0 {
		return
	}
	cur := t.diffCursor
	last := len(rows) - 1

	isChanged := func(i int) bool {
		return rows[i].Type != diff.RowUnchanged
	}
	matchesSide := func(i int) bool {
		return diffRowAvailableSide(rows[i], t.diffSide) == t.diffSide
	}

	if !isChanged(cur) {
		m.diffJumpToNextBlock(t, cur, dir)
		return
	}

	// Find the boundaries of the current change block.
	blockStart := cur
	for blockStart > 0 && isChanged(blockStart-1) {
		blockStart--
	}
	blockEnd := cur
	for blockEnd < last && isChanged(blockEnd+1) {
		blockEnd++
	}

	if dir > 0 {
		// Find last matching row between cur+1 and blockEnd.
		target := -1
		for i := blockEnd; i > cur; i-- {
			if matchesSide(i) {
				target = i
				break
			}
		}
		if target >= 0 {
			t.diffCursor = target
		} else {
			m.diffJumpToNextBlock(t, blockEnd, dir)
			return
		}
	} else {
		// Find first matching row between blockStart and cur-1.
		target := -1
		for i := blockStart; i < cur; i++ {
			if matchesSide(i) {
				target = i
				break
			}
		}
		if target >= 0 {
			t.diffCursor = target
		} else {
			m.diffJumpToNextBlock(t, blockStart, dir)
			return
		}
	}

	t.syncDiffAnchor()
}

// diffJumpToNextBlock moves the cursor to the next change block in dir,
// landing on the first (dir>0) or last (dir<0) row that matches the
// current diffSide. Blocks with no matching rows are skipped.
func (m *Model) diffJumpToNextBlock(t *tab, from, dir int) {
	rows := t.diffViewData.Rows
	last := len(rows) - 1

	isChanged := func(i int) bool {
		return rows[i].Type != diff.RowUnchanged
	}
	matchesSide := func(i int) bool {
		return diffRowAvailableSide(rows[i], t.diffSide) == t.diffSide
	}
	expandBlock := func(pos int) (int, int) {
		s, e := pos, pos
		for s > 0 && isChanged(s-1) {
			s--
		}
		for e < last && isChanged(e+1) {
			e++
		}
		return s, e
	}
	// searchMatch scans [from, to] in steps of step for a matching row.
	// Returns the index or -1.
	searchMatch := func(from, to, step int) int {
		for i := from; i != to+step; i += step {
			if matchesSide(i) {
				return i
			}
		}
		return -1
	}

	line := from + dir
	// Skip any remaining changed rows of the current block.
	for line >= 0 && line <= last && isChanged(line) {
		line += dir
	}

	// Search successive blocks until finding one with a matching row.
	for {
		for line >= 0 && line <= last && !isChanged(line) {
			line += dir
		}
		if line < 0 || line > last || !isChanged(line) {
			return
		}

		bStart, bEnd := expandBlock(line)

		var target int
		if dir > 0 {
			target = searchMatch(bStart, bEnd, 1)
		} else {
			target = searchMatch(bEnd, bStart, -1)
		}
		if target >= 0 {
			t.diffCursor = target
			t.syncDiffAnchor()
			return
		}

		// No matching row in this block; skip past it.
		if dir > 0 {
			line = bEnd + 1
		} else {
			line = bStart - 1
		}
	}
}

// handleKeyNormal handles key events in normal (non-input, non-overlay) mode.
func (m *Model) handleKeyNormal(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	t, hasTab := m.activeTabState()

	if hasTab && (t.diffViewData != nil || t.diff != nil) && m.focusPane == paneEditor {
		if model, cmd, ok := m.handleDiffKeyNormal(t, msg); ok {
			m.adjustScroll()
			return model, cmd
		}
	}

	switch {
	case key.Matches(msg, m.keys.Cancel):
		if m.search.query != "" {
			m.search.query = ""
			m.clearSearchMatches()
			return m, nil
		}
		if hasTab && t.selecting {
			t.selecting = false
			m.notifyClearSelection()
			return m, nil
		}
	case key.Matches(msg, m.keys.SwitchPanel):
		m.activePanel = (m.activePanel + 1) % panelCount
		m.treeScrollOffset = 0
		m.treeCursor = 0
		if m.activePanel == panelGitDiff && !m.gitState().loaded {
			m.adjustScroll()
			cmd := m.loadGitChangesForMode(m.gitDiffMode)
			return m, cmd
		}
	case key.Matches(msg, m.keys.ToggleSidebar):
		m.sidebarVisible = !m.sidebarVisible
		if !m.sidebarVisible {
			m.resizingPane = false
			if m.focusPane == paneTree {
				m.focusPane = paneEditor
			}
		}
	case key.Matches(msg, m.keys.SwitchPane):
		if hasTab && len(t.lines) > 0 && m.sidebarVisible {
			if m.focusPane == paneEditor {
				m.notifyClearSelection()
			}
			if m.focusPane == paneTree {
				m.focusPane = paneEditor
			} else {
				m.focusPane = paneTree
			}
		}
	case msg.Code == tea.KeyEnter && msg.Mod.Contains(tea.ModShift):
		if m.focusPane == paneEditor && m.search.query != "" {
			m.prevMatch()
		}
	case key.Matches(msg, m.keys.Enter):
		if m.focusPane == paneEditor && m.search.query != "" {
			m.nextMatch()
		} else if m.focusPane == paneTree {
			switch m.activePanel {
			case panelFiles:
				if len(m.fileTree) > 0 {
					m.toggleTreeEntry(m.treeCursor)
				}
			case panelGitDiff:
				m.openGitDiffEntry()
			}
		}
	case key.Matches(msg, m.keys.Left):
		switch {
		case m.focusPane == paneTree && m.activePanel == panelGitDiff:
			cmd := m.switchGitMode(-1)
			return m, cmd
		case m.focusPane == paneTree:
			if len(m.fileTree) > 0 {
				entry := m.fileTree[m.treeCursor]
				if entry.isDir && entry.expanded {
					m.fileTree = collapseDir(m.fileTree, m.treeCursor)
				}
			}
		case hasTab:
			if t.cursorChar > 0 {
				t.cursorChar--
			} else if t.cursorLine > 0 {
				t.cursorLine--
				t.cursorChar = t.lineLen(t.cursorLine)
			}
			t.syncAnchorToCursor()
			m.notifySelectionChanged()
		}
	case key.Matches(msg, m.keys.Right):
		switch {
		case m.focusPane == paneTree && m.activePanel == panelGitDiff:
			cmd := m.switchGitMode(1)
			return m, cmd
		case m.focusPane == paneTree:
			if len(m.fileTree) > 0 {
				entry := m.fileTree[m.treeCursor]
				if entry.isDir && !entry.expanded {
					m.fileTree = expandDir(m.fileTree, m.treeCursor)
				}
			}
		case hasTab:
			if t.cursorChar < t.lineLen(t.cursorLine) {
				t.cursorChar++
			} else if t.cursorLine < len(t.lines)-1 {
				t.cursorLine++
				t.cursorChar = 0
			}
			t.syncAnchorToCursor()
			m.notifySelectionChanged()
		}
	case key.Matches(msg, m.keys.Up):
		switch {
		case m.focusPane == paneTree && m.activePanel == panelGitDiff:
			m.gitCursorUp()
		case m.focusPane == paneTree:
			if m.treeCursor > 0 {
				m.treeCursor--
			}
		case hasTab:
			if t.cursorLine > 0 {
				t.cursorLine--
				t.cursorChar = min(t.cursorChar, t.lineLen(t.cursorLine))
				t.syncAnchorToCursor()
				m.notifySelectionChanged()
			}
		}
	case key.Matches(msg, m.keys.Down):
		switch {
		case m.focusPane == paneTree && m.activePanel == panelGitDiff:
			m.gitCursorDown()
		case m.focusPane == paneTree:
			if m.treeCursor < len(m.fileTree)-1 {
				m.treeCursor++
			}
		case hasTab:
			if t.cursorLine < len(t.lines)-1 {
				t.cursorLine++
				t.cursorChar = min(t.cursorChar, t.lineLen(t.cursorLine))
				t.syncAnchorToCursor()
				m.notifySelectionChanged()
			}
		}
	case key.Matches(msg, m.keys.Select):
		if hasTab && m.focusPane == paneEditor && len(t.lines) > 0 {
			if t.selecting {
				t.selecting = false
				m.notifyClearSelection()
			} else {
				t.startSelecting()
				m.notifySelectionChanged()
			}
		}
	case key.Matches(msg, m.keys.Comment):
		if hasTab && m.focusPane == paneEditor && len(t.lines) > 0 {
			t.inputMode = true
			if t.selecting {
				s, _, e, _ := t.normalizedSelection()
				t.inputStart = s
				t.inputEnd = e
				t.selecting = false
			} else {
				t.inputStart = t.cursorLine
				t.inputEnd = t.cursorLine
			}
			lo := m.computeLayout()
			t.commentInput.SetWidth(
				lo.editorWidth - lo.lineNumWidth - commentBlockMargin - commentBorderChars)
			t.commentInput.SetHeight(3)
			if m.enhancedKeyboard {
				t.commentInput.KeyMap.InsertNewline = key.NewBinding(
					key.WithKeys("shift+enter"),
				)
			}
			t.commentInput.Reset()
			if idx := t.findComment(t.inputStart); idx >= 0 {
				t.inputStart = t.comments[idx].StartLine
				t.inputEnd = t.comments[idx].EndLine
				t.commentInput.SetValue(t.comments[idx].Text)
			}
			t.commentInput.Focus()
		}
	case key.Matches(msg, m.keys.Copy):
		if hasTab && m.focusPane == paneEditor && t.selecting {
			text := t.selectedText()
			n := strings.Count(text, "\n") + 1
			m.statusMsg = fmt.Sprintf("Copied %d lines", n)
			return m, tea.Batch(
				tea.SetClipboard(text),
				statusTickCmd(),
			)
		}
	case key.Matches(msg, m.keys.ClearAll):
		if hasTab && m.focusPane == paneEditor && t.filePath != "" && len(t.comments) > 0 {
			m.clearAllPending = true
		}
	case key.Matches(msg, m.keys.NextTab):
		if len(m.tabs) > 0 {
			m.activeTab = (m.activeTab + 1) % len(m.tabs)
		}
	case key.Matches(msg, m.keys.PrevTab):
		if len(m.tabs) > 0 {
			m.activeTab = (m.activeTab - 1 + len(m.tabs)) % len(m.tabs)
		}
	case key.Matches(msg, m.keys.GoBottom):
		switch {
		case m.focusPane == paneTree && m.activePanel == panelGitDiff:
			gs := m.gitState()
			if len(gs.entries) > 0 {
				gs.cursor = lastGitEntryIdx(gs.visualRows)
			}
		case m.focusPane == paneTree:
			if len(m.fileTree) > 0 {
				m.treeCursor = len(m.fileTree) - 1
			}
		case hasTab && len(t.lines) > 0:
			t.cursorLine = len(t.lines) - 1
			t.cursorChar = 0
			t.syncAnchorToCursor()
			m.notifySelectionChanged()
		}
	case key.Matches(msg, m.keys.BlockUp):
		m.moveToParagraphBoundary(dirUp)
	case key.Matches(msg, m.keys.BlockDown):
		m.moveToParagraphBoundary(dirDown)
	case key.Matches(msg, m.keys.CloseTab):
		if len(m.tabs) > 0 {
			m.closeTab(m.activeTab)
		}
	case key.Matches(msg, m.keys.GoTop):
		m.gPending = true
	case key.Matches(msg, m.keys.OpenFile):
		return m, m.openFile.open(m.rootDir, m.excludeFunc)
	case key.Matches(msg, m.keys.Search):
		if hasTab {
			m.startSearch()
		}
	case key.Matches(msg, m.keys.SearchNext):
		m.nextMatch()
	case key.Matches(msg, m.keys.SearchPrev):
		m.prevMatch()
	}

	m.adjustScroll()
	return m, nil
}
