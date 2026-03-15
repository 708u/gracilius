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
			if hasTab && t.filePath != "" {
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
			m.notifySelectionChanged()
		}
		return m, nil, true
	case key.Matches(msg, m.keys.Down):
		if t.diffViewData != nil && t.diffCursor < len(t.diffViewData.rows)-1 {
			t.diffCursor++
			t.syncDiffAnchor()
			m.notifySelectionChanged()
		}
		return m, nil, true
	case key.Matches(msg, m.keys.GoBottom):
		if t.diffViewData != nil && len(t.diffViewData.rows) > 0 {
			t.diffCursor = len(t.diffViewData.rows) - 1
			t.syncDiffAnchor()
			m.notifySelectionChanged()
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
		return m, nil, true
	case key.Matches(msg, m.keys.BlockDown):
		m.diffJumpBlankLine(t, 1)
		return m, nil, true

	// Change block jump ([/]).
	case key.Matches(msg, m.keys.ChangeUp):
		m.diffJumpChange(t, -1)
		return m, nil, true
	case key.Matches(msg, m.keys.ChangeDown):
		m.diffJumpChange(t, 1)
		return m, nil, true

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

	// No-op: suppress file-tab actions that should not fire on diff tabs.
	// Search keys (/, n, N, Enter, Shift+Enter) are intentionally NOT
	// listed here so they fall through to handleKeyNormal.
	case key.Matches(msg, m.keys.Left),
		key.Matches(msg, m.keys.Right),
		key.Matches(msg, m.keys.Comment):
		return m, nil, true
	}

	return m, nil, false
}

// diffJumpBlankLine moves the diff cursor to the next/previous blank-line
// boundary, mirroring moveToParagraphBoundary for file tabs.
func (m *Model) diffJumpBlankLine(t *tab, dir int) {
	if t.diffViewData == nil {
		return
	}
	rows := t.diffViewData.rows
	cur := t.diffCursor
	last := len(rows) - 1

	inBounds := func(i int) bool {
		if dir > 0 {
			return i < last
		}
		return i > 0
	}

	isBlank := func(i int) bool {
		return isBlankLine(diffRowText(rows[i]))
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
		t.syncDiffAnchor()
		m.notifySelectionChanged()
	}
}

// diffJumpChange moves the diff cursor stepwise through change blocks.
//
// For dir > 0 (]):
//   - Inside a change block (not at its last row): jump to the last row.
//   - At the last row of a change block, or on an unchanged row: jump to
//     the first row of the next change block.
//
// For dir < 0 ([):
//   - Inside a change block (not at its first row): jump to the first row.
//   - At the first row of a change block, or on an unchanged row: jump to
//     the last row of the previous change block.
func (m *Model) diffJumpChange(t *tab, dir int) {
	if t.diffViewData == nil {
		return
	}
	rows := t.diffViewData.rows
	if len(rows) == 0 {
		return
	}
	cur := t.diffCursor
	last := len(rows) - 1

	isChanged := func(i int) bool {
		return rows[i].rowType != diffRowUnchanged
	}

	if !isChanged(cur) {
		// On an unchanged row: find the nearest change block in dir.
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
		if cur < blockEnd {
			// Not at the last row: jump to end of this block.
			t.diffCursor = blockEnd
		} else {
			// At the last row: jump to the next block's first row.
			m.diffJumpToNextBlock(t, cur, dir)
			return
		}
	} else {
		if cur > blockStart {
			// Not at the first row: jump to start of this block.
			t.diffCursor = blockStart
		} else {
			// At the first row: jump to the previous block's last row.
			m.diffJumpToNextBlock(t, cur, dir)
			return
		}
	}

	t.syncDiffAnchor()
	m.notifySelectionChanged()
}

// diffJumpToNextBlock moves the cursor to the next change block in dir.
// For dir > 0 it lands on the block's first row; for dir < 0 on its last row.
func (m *Model) diffJumpToNextBlock(t *tab, from, dir int) {
	rows := t.diffViewData.rows
	last := len(rows) - 1

	isChanged := func(i int) bool {
		return rows[i].rowType != diffRowUnchanged
	}

	line := from + dir
	// Skip any remaining changed rows of the current block.
	for line >= 0 && line <= last && isChanged(line) {
		line += dir
	}
	// Skip unchanged rows.
	for line >= 0 && line <= last && !isChanged(line) {
		line += dir
	}

	if line < 0 || line > last || !isChanged(line) {
		return
	}

	// Going backward: the scan lands on the first changed row of the block;
	// walk forward to find the last row so we land on the block's end.
	if dir < 0 {
		for line < last && isChanged(line+1) {
			line++
		}
	}

	t.diffCursor = line
	t.syncDiffAnchor()
	m.notifySelectionChanged()
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
