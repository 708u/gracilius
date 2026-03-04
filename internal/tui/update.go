package tui

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

const (
	scrollAmount       = 3
	quitTimeout        = 750 * time.Millisecond
	statusClearTimeout = 2 * time.Second
)

// quitTimeoutMsg is sent when the quit confirmation window expires.
type quitTimeoutMsg struct{}

// statusClearMsg is sent to clear the temporary status message.
type statusClearMsg struct{}

// Init implements tea.Model.
func (m *Model) Init() tea.Cmd {
	return tea.Batch(m.watchFile(), m.watchDir(), tea.RequestBackgroundColor)
}

type direction int

const (
	dirUp   direction = -1
	dirDown direction = 1
)

// isBlankLine returns true if the line contains only whitespace.
func isBlankLine(s string) bool {
	return !strings.ContainsFunc(s, func(r rune) bool {
		return !unicode.IsSpace(r)
	})
}

// moveToParagraphBoundary moves the cursor to the next paragraph
// boundary in the given direction (1 for down, -1 for up).
func (m *Model) moveToParagraphBoundary(dir direction) {
	t, hasTab := m.activeTabState()
	if !hasTab || m.focusPane != paneEditor || len(t.lines) == 0 {
		return
	}
	line := t.cursorLine
	last := len(t.lines) - 1
	inBounds := func(l int) bool {
		if dir > 0 {
			return l < last
		}
		return l > 0
	}
	if inBounds(line) {
		line += int(dir)
		for inBounds(line) && isBlankLine(t.lines[line]) {
			line += int(dir)
		}
		for inBounds(line) && !isBlankLine(t.lines[line]) {
			line += int(dir)
		}
	}
	t.cursorLine = line
	t.cursorChar = 0
	t.syncAnchorToCursor()
	m.notifySelectionChanged()
}

// adjustTreeScroll adjusts the tree scroll so the tree cursor
// stays visible.
func (m *Model) adjustTreeScroll(contentHeight int) {
	if m.treeScrollOffset > m.treeCursor {
		m.treeScrollOffset = m.treeCursor
	}
	if m.treeCursor >= m.treeScrollOffset+contentHeight {
		m.treeScrollOffset = m.treeCursor - contentHeight + 1
	}
	maxOffset := max(len(m.fileTree)-contentHeight, 0)
	if m.treeScrollOffset > maxOffset {
		m.treeScrollOffset = maxOffset
	}
}

// Update implements tea.Model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Route non-key messages (e.g. cursor blink) to the open-file overlay
	// when it is active.
	if m.openFile.active {
		switch msg.(type) {
		case tea.KeyPressMsg, tea.MouseClickMsg,
			tea.WindowSizeMsg,
			fileChangedMsg, treeChangedMsg,
			OpenDiffMsg, CloseDiffMsg,
			quitTimeoutMsg, statusClearMsg, IdeConnectedMsg:
			// Fall through to normal handling below.
		default:
			cmd := m.openFile.update(msg)
			return m, cmd
		}
	}

	t, hasTab := m.activeTabState()

	switch msg := msg.(type) {
	case tea.BackgroundColorMsg:
		m.isDark = msg.IsDark()
		if m.isDark {
			m.theme = darkTheme
		} else {
			m.theme = lightTheme
		}
		m.help.Styles = help.DefaultStyles(m.isDark)
		m.openFile.updateTheme(m.theme)
		for _, tab := range m.tabs {
			if tab.filePath != "" && len(tab.lines) > 0 {
				tab.highlightedLines = highlightFile(
					tab.filePath, strings.Join(tab.lines, "\n"), m.theme,
				)
			}
		}
		return m, nil
	case fileChangedMsg:
		if hasTab {
			t.lines = msg.lines
			t.highlightedLines = highlightFile(
				t.filePath, strings.Join(msg.lines, "\n"), m.theme,
			)
			if t.cursorLine >= len(t.lines) {
				t.cursorLine = max(0, len(t.lines)-1)
			}
			if t.cursorLine < len(t.lines) {
				t.cursorChar = min(t.cursorChar, len(t.lines[t.cursorLine]))
			}
			if t.filePath != "" {
				m.notifySelectionChanged()
			}
		}
		cmd := m.watchFile()
		return m, cmd
	case treeChangedMsg:
		m.fileTree = buildFileTree(m.rootDir)
		if m.treeCursor >= len(m.fileTree) {
			m.treeCursor = max(0, len(m.fileTree)-1)
		}
		cmd := m.watchDir()
		return m, cmd
	case OpenDiffMsg:
		lines := splitLines([]byte(msg.Contents))
		dt := newDiffTab(msg.FilePath, lines, msg.Accept, msg.Reject)
		dt.highlightedLines = highlightFile(msg.FilePath, msg.Contents, m.theme)
		m.tabs = append(m.tabs, dt)
		m.activeTab = len(m.tabs) - 1
		m.focusPane = paneEditor
		return m, nil
	case CloseDiffMsg:
		m.closeDiffTabs()
		return m, nil
	case quitTimeoutMsg:
		m.quitPending = false
		return m, nil
	case statusClearMsg:
		m.statusMsg = ""
		return m, nil
	case IdeConnectedMsg:
		if hasTab && t.filePath != "" && len(t.lines) > 0 {
			m.notifySelectionChanged()
		}
		return m, nil
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		maxWidth := m.width * maxTreeWidthPercent / 100
		if m.treeWidth > maxWidth {
			m.treeWidth = maxWidth
		}
		if hasTab && t.filePath != "" && len(t.lines) > 0 && m.focusPane == paneEditor {
			m.notifySelectionChanged()
		}
	case tea.MouseClickMsg:
		if m.openFile.active {
			if msg.Button == tea.MouseLeft {
				path, closeOverlay := m.openFile.handleClick(msg.X, msg.Y, m.width, m.height)
				if path != "" {
					absPath := filepath.Join(m.rootDir, path)
					m.openFile.close()
					m.openFileByPath(absPath)
				} else if closeOverlay {
					m.openFile.close()
				}
			}
			return m, nil
		}
		lo := m.computeLayout()

		borderX := lo.treeWidth
		isBorderArea := msg.X >= borderX && msg.X <= borderX+2 && msg.Y >= contentStartY

		if isBorderArea && msg.Button == tea.MouseLeft {
			m.resizingPane = true
			return m, nil
		}

		if msg.X < lo.treeWidth && msg.Y >= contentStartY && msg.Button == tea.MouseLeft {
			treeIdx := msg.Y - contentStartY + m.treeScrollOffset
			if treeIdx >= 0 && treeIdx < len(m.fileTree) {
				m.treeCursor = treeIdx
				m.toggleTreeEntry(treeIdx)
			}
			return m, nil
		}

		if !hasTab || len(t.lines) == 0 {
			return m, nil
		}

		if msg.Button == tea.MouseLeft && msg.X >= lo.editorStartX && msg.Y >= contentStartY {
			targetLine, targetChar := m.editorTarget(t, lo, msg.X, msg.Y)
			m.focusPane = paneEditor
			t.cursorLine = targetLine
			t.cursorChar = targetChar
			t.anchorLine = targetLine
			t.anchorChar = targetChar
			t.selecting = false
			t.lineSelect = false
			m.mouseDown = true
			m.lastMouseLine = targetLine
			m.lastMouseChar = targetChar
		}
		return m, nil
	case tea.MouseMotionMsg:
		if m.resizingPane {
			m.treeWidth = msg.X
			return m, nil
		}

		if !hasTab || len(t.lines) == 0 || !m.mouseDown {
			return m, nil
		}

		lo := m.computeLayout()
		if msg.X >= lo.editorStartX && msg.Y >= contentStartY {
			targetLine, targetChar := m.editorTarget(t, lo, msg.X, msg.Y)
			if targetLine != m.lastMouseLine || targetChar != m.lastMouseChar {
				t.selecting = true
				t.cursorLine = targetLine
				t.cursorChar = targetChar
				m.lastMouseLine = targetLine
				m.lastMouseChar = targetChar
			}
		}
		return m, nil
	case tea.MouseReleaseMsg:
		if m.openFile.active {
			return m, nil
		}
		if m.resizingPane {
			m.resizingPane = false
			return m, nil
		}

		wasDown := m.mouseDown
		m.mouseDown = false

		if !hasTab || len(t.lines) == 0 {
			return m, nil
		}

		if wasDown && t.selecting {
			lo := m.computeLayout()
			if msg.X >= lo.editorStartX && msg.Y >= contentStartY {
				targetLine, targetChar := m.editorTarget(t, lo, msg.X, msg.Y)
				t.cursorLine = targetLine
				t.cursorChar = targetChar
			}
			m.notifySelectionChanged()
		}
		return m, nil
	case tea.MouseWheelMsg:
		if m.openFile.active {
			return m, nil
		}
		if !hasTab || len(t.lines) == 0 {
			return m, nil
		}
		lo := m.computeLayout()
		if msg.X >= lo.editorStartX && msg.Y >= contentStartY {
			switch msg.Button {
			case tea.MouseWheelUp:
				t.scrollOffset -= scrollAmount
				if t.scrollOffset < 0 {
					t.scrollOffset = 0
				}
			case tea.MouseWheelDown:
				t.scrollOffset += scrollAmount
				maxOffset := t.maxScrollOffset(lo.contentHeight, lo.textWidth)
				if t.scrollOffset > maxOffset {
					t.scrollOffset = maxOffset
				}
			}
		}
		return m, nil
	case tea.KeyPressMsg:
		var cmd tea.Cmd
		if key.Matches(msg, m.keys.Quit) {
			if m.quitPending {
				return m, tea.Quit
			}
			m.quitPending = true
			return m, tea.Tick(quitTimeout, func(time.Time) tea.Msg {
				return quitTimeoutMsg{}
			})
		}

		if hasTab && t.inputMode {
			switch {
			case key.Matches(msg, m.keys.Cancel):
				t.inputMode = false
				t.commentInput.Reset()
				t.commentInput.Blur()
			case key.Matches(msg, m.keys.CommentSubmit):
				val := t.commentInput.Value()
				idx := t.findComment(t.inputStart)
				if idx >= 0 {
					t.comments = slices.Delete(t.comments, idx, idx+1)
				}
				if val != "" {
					t.comments = append(t.comments, comment{
						startLine: t.inputStart,
						endLine:   t.inputEnd,
						text:      val,
					})
					m.notifyComment(t.inputStart, t.inputEnd, val)
				}
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

		if m.gPending {
			m.gPending = false
			if key.Matches(msg, m.keys.GoTop) {
				if m.focusPane == paneTree {
					m.treeCursor = 0
				} else if len(t.lines) > 0 {
					t.cursorLine = 0
					t.cursorChar = 0
					t.syncAnchorToCursor()
					m.notifySelectionChanged()
				}
				break
			}
		}

		if m.openFile.active {
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
				cmd = m.openFile.update(msg)
				return m, cmd
			}
		}

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
		case key.Matches(msg, m.keys.SwitchPane):
			if hasTab && len(t.lines) > 0 {
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
			if m.focusPane == paneTree && len(m.fileTree) > 0 {
				m.toggleTreeEntry(m.treeCursor)
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
			if m.focusPane == paneTree {
				if m.treeCursor > 0 {
					m.treeCursor--
				}
			} else if hasTab {
				if t.cursorLine > 0 {
					t.cursorLine--
					t.cursorChar = min(t.cursorChar, t.lineLen(t.cursorLine))
					t.syncAnchorToCursor()
					m.notifySelectionChanged()
				}
			}
		case key.Matches(msg, m.keys.Down):
			if m.focusPane == paneTree {
				if m.treeCursor < len(m.fileTree)-1 {
					m.treeCursor++
				}
			} else if hasTab {
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
				t.commentInput.Reset()
				if idx := t.findComment(t.inputStart); idx >= 0 {
					t.inputStart = t.comments[idx].startLine
					t.inputEnd = t.comments[idx].endLine
					t.commentInput.SetValue(t.comments[idx].text)
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
					tea.Tick(statusClearTimeout, func(time.Time) tea.Msg {
						return statusClearMsg{}
					}),
				)
			}
		case key.Matches(msg, m.keys.ClearAll):
			if hasTab && m.focusPane == paneEditor {
				t.comments = nil
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
			if m.focusPane == paneTree {
				if len(m.fileTree) > 0 {
					m.treeCursor = len(m.fileTree) - 1
				}
			} else if hasTab && len(t.lines) > 0 {
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
			return m, m.openFile.open(m.rootDir)
		}
	}

	lo := m.computeLayout()
	if m.focusPane == paneTree {
		m.adjustTreeScroll(lo.contentHeight)
	} else if hasTab && len(t.lines) > 0 {
		t.adjustScrollForCursor(lo.contentHeight, lo.textWidth)
	}

	return m, nil
}

// editorTarget converts mouse coordinates to editor line and character.
func (m *Model) editorTarget(t *tab, lo layout, mouseX, mouseY int) (int, int) {
	editorX := mouseX - lo.editorStartX - lo.lineNumWidth
	editorY := mouseY - contentStartY

	targetLine := t.scrollOffset + editorY
	if editorY >= 0 && editorY < len(m.lastMapping) {
		targetLine = m.lastMapping[editorY].logicalLine
	}
	if targetLine >= len(t.lines) {
		targetLine = len(t.lines) - 1
	}
	if targetLine < 0 {
		targetLine = 0
	}

	targetChar := max(editorX, 0)
	if editorY >= 0 && editorY < len(m.lastMapping) {
		targetChar += m.lastMapping[editorY].wrapOffset
	}
	if targetLine < len(t.lines) {
		runeLen := len([]rune(t.lines[targetLine]))
		if targetChar > runeLen {
			targetChar = runeLen
		}
	}
	return targetLine, targetChar
}

// closeTab removes the tab at idx and adjusts activeTab.
func (m *Model) closeTab(idx int) {
	t := m.tabs[idx]
	if t.kind == diffTab {
		t.rejectAndClear()
	}
	if t.filePath != "" && t.kind == fileTab && m.watcher != nil {
		_ = m.watcher.Remove(t.filePath)
	}
	m.tabs = slices.Delete(m.tabs, idx, idx+1)
	if len(m.tabs) == 0 {
		m.activeTab = 0
		m.focusPane = paneTree
	} else if m.activeTab >= len(m.tabs) {
		m.activeTab = len(m.tabs) - 1
	}
}

// closeDiffTabs removes all diff tabs.
func (m *Model) closeDiffTabs() {
	tabs := make([]*tab, 0, len(m.tabs))
	for _, t := range m.tabs {
		if t.kind == diffTab {
			t.rejectAndClear()
		} else {
			tabs = append(tabs, t)
		}
	}
	m.tabs = tabs
	if len(m.tabs) == 0 {
		m.activeTab = 0
		m.focusPane = paneTree
	} else if m.activeTab >= len(m.tabs) {
		m.activeTab = len(m.tabs) - 1
	}
}
