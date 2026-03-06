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

	if m.gPending {
		m.gPending = false
		if key.Matches(msg, m.keys.GoTop) {
			switch {
			case m.focusPane == paneTree && m.activePanel == panelGitDiff:
				m.gitCursor = 0
			case m.focusPane == paneTree:
				m.treeCursor = 0
			case hasTab && t.diffViewData != nil:
				t.vp.SetYOffset(0)
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

// handleKeyNormal handles key events in normal (non-input, non-overlay) mode.
func (m *Model) handleKeyNormal(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	t, hasTab := m.activeTabState()

	// Accept/reject use enter/esc, so they must be checked
	// before the general Cancel/Enter handlers below.
	if hasTab && t.diff != nil && m.focusPane == paneEditor {
		switch {
		case key.Matches(msg, m.keys.AcceptDiff):
			contents := strings.Join(t.lines, "\n")
			t.diff.onAccept(contents)
			t.diff = nil
			m.closeTab(m.activeTab)
			return m, nil
		case key.Matches(msg, m.keys.RejectDiff):
			t.rejectAndClear()
			m.closeTab(m.activeTab)
			return m, nil
		}
	}

	switch {
	case key.Matches(msg, m.keys.Cancel):
		if hasTab && t.selecting {
			t.selecting = false
			t.lineSelect = false
			m.notifyClearSelection()
			return m, nil
		}
	case key.Matches(msg, m.keys.SwitchPanel):
		m.activePanel = (m.activePanel + 1) % panelCount
		m.treeScrollOffset = 0
		m.treeCursor = 0
		if m.activePanel == panelGitDiff && !m.gitLoaded {
			m.adjustScroll()
			cmd := m.loadGitChanges()
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
	case key.Matches(msg, m.keys.Enter):
		if m.focusPane == paneTree {
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
		if m.focusPane == paneTree {
			if len(m.fileTree) > 0 {
				entry := m.fileTree[m.treeCursor]
				if entry.isDir && entry.expanded {
					m.fileTree = collapseDir(m.fileTree, m.treeCursor)
				}
			}
		} else if hasTab {
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
		if m.focusPane == paneTree {
			if len(m.fileTree) > 0 {
				entry := m.fileTree[m.treeCursor]
				if entry.isDir && !entry.expanded {
					m.fileTree = expandDir(m.fileTree, m.treeCursor)
				}
			}
		} else if hasTab {
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
			if m.gitCursor > 0 {
				m.gitCursor--
			}
		case m.focusPane == paneTree:
			if m.treeCursor > 0 {
				m.treeCursor--
			}
		case hasTab && t.diffViewData != nil:
			if t.vp.YOffset() > 0 {
				t.vp.SetYOffset(t.vp.YOffset() - 1)
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
			if m.gitCursor < len(m.gitChangedFiles)-1 {
				m.gitCursor++
			}
		case m.focusPane == paneTree:
			if m.treeCursor < len(m.fileTree)-1 {
				m.treeCursor++
			}
		case hasTab && t.diffViewData != nil:
			t.vp.SetYOffset(t.vp.YOffset() + 1)
		case hasTab:
			if t.cursorLine < len(t.lines)-1 {
				t.cursorLine++
				t.cursorChar = min(t.cursorChar, t.lineLen(t.cursorLine))
				t.syncAnchorToCursor()
				m.notifySelectionChanged()
			}
		}
	case key.Matches(msg, m.keys.CharSelect):
		if hasTab && m.focusPane == paneEditor && len(t.lines) > 0 {
			switch {
			case t.selecting && !t.lineSelect:
				t.selecting = false
				m.notifyClearSelection()
			case t.selecting && t.lineSelect:
				t.lineSelect = false
				m.notifySelectionChanged()
			default:
				t.startSelecting()
			}
		}
	case key.Matches(msg, m.keys.LineSelect):
		if hasTab && m.focusPane == paneEditor && len(t.lines) > 0 {
			switch {
			case t.selecting && t.lineSelect:
				t.selecting = false
				t.lineSelect = false
				m.notifyClearSelection()
			case t.selecting && !t.lineSelect:
				t.lineSelect = true
				m.notifySelectionChanged()
			default:
				t.startSelecting()
				t.lineSelect = true
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
				t.lineSelect = false
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
			if len(m.gitChangedFiles) > 0 {
				m.gitCursor = len(m.gitChangedFiles) - 1
			}
		case m.focusPane == paneTree:
			if len(m.fileTree) > 0 {
				m.treeCursor = len(m.fileTree) - 1
			}
		case hasTab && t.diffViewData != nil:
			t.vp.GotoBottom()
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
	case key.Matches(msg, m.keys.Refresh):
		if m.activePanel == panelGitDiff {
			m.gitLoaded = false
			cmd := m.loadGitChanges()
			return m, cmd
		}
	case key.Matches(msg, m.keys.CloseTab):
		if len(m.tabs) > 0 {
			m.closeTab(m.activeTab)
		}
	case key.Matches(msg, m.keys.GoTop):
		m.gPending = true
	case key.Matches(msg, m.keys.OpenFile):
		return m, m.openFile.open(m.rootDir)
	}

	m.adjustScroll()
	return m, nil
}
