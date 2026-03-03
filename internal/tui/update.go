package tui

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
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
	return tea.Batch(m.watchFile(), m.watchDir())
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
	// Route non-key messages (e.g. cursor blink) to the search overlay
	// when it is active.
	if m.search.active {
		switch msg.(type) {
		case tea.KeyMsg, tea.MouseMsg, tea.WindowSizeMsg,
			fileChangedMsg, treeChangedMsg,
			OpenDiffMsg, CloseDiffMsg,
			quitTimeoutMsg, statusClearMsg, IdeConnectedMsg:
			// Fall through to normal handling below.
		default:
			cmd := m.search.update(msg)
			return m, cmd
		}
	}

	t, hasTab := m.activeTabState()

	switch msg := msg.(type) {
	case fileChangedMsg:
		if hasTab {
			t.lines = msg.lines
			t.highlightedLines = highlightFile(
				t.filePath, strings.Join(msg.lines, "\n"),
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
		dt := newDiffTab(msg.FilePath, lines)
		dt.highlightedLines = highlightFile(msg.FilePath, msg.Contents)
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
	case tea.MouseMsg:
		if m.search.active {
			if msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress {
				path, closeOverlay := m.search.handleClick(msg.X, msg.Y, m.width, m.height)
				if path != "" {
					absPath := filepath.Join(m.rootDir, path)
					m.search.close()
					m.openFileByPath(absPath)
				} else if closeOverlay {
					m.search.close()
				}
			}
			return m, nil
		}
		lo := m.computeLayout()

		borderX := lo.treeWidth
		isBorderArea := msg.X >= borderX && msg.X <= borderX+2 && msg.Y >= contentStartY

		if isBorderArea && msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress {
			m.resizingPane = true
			return m, nil
		}
		if m.resizingPane && msg.Action == tea.MouseActionMotion {
			m.treeWidth = msg.X
			return m, nil
		}
		if m.resizingPane && msg.Action == tea.MouseActionRelease {
			m.resizingPane = false
			return m, nil
		}

		if msg.X < lo.treeWidth && msg.Y >= contentStartY && msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress {
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

		if msg.X >= lo.editorStartX && msg.Y >= contentStartY {
			editorX := msg.X - lo.editorStartX - lineNumberWidth
			editorY := msg.Y - contentStartY
			offset := t.scrollOffset
			targetLine := offset + editorY

			if targetLine >= len(t.lines) {
				targetLine = len(t.lines) - 1
			}
			if targetLine < 0 {
				targetLine = 0
			}

			targetChar := max(editorX, 0)
			if targetLine < len(t.lines) {
				runeLen := len([]rune(t.lines[targetLine]))
				if targetChar > runeLen {
					targetChar = runeLen
				}
			}

			switch {
			case msg.Button == tea.MouseButtonLeft:
				m.focusPane = paneEditor
				switch msg.Action {
				case tea.MouseActionPress:
					t.cursorLine = targetLine
					t.cursorChar = targetChar
					t.anchorLine = targetLine
					t.anchorChar = targetChar
					t.selecting = false
					t.lineSelect = false
					m.mouseDown = true
					m.lastMouseLine = targetLine
					m.lastMouseChar = targetChar
				case tea.MouseActionMotion:
					if m.mouseDown && (targetLine != m.lastMouseLine || targetChar != m.lastMouseChar) {
						t.selecting = true
						t.cursorLine = targetLine
						t.cursorChar = targetChar
						m.lastMouseLine = targetLine
						m.lastMouseChar = targetChar
					}
				case tea.MouseActionRelease:
					m.mouseDown = false
					if t.selecting {
						t.cursorLine = targetLine
						t.cursorChar = targetChar
						m.notifySelectionChanged()
					}
				}
			case msg.Action == tea.MouseActionRelease:
				m.mouseDown = false
				if t.selecting {
					t.cursorLine = targetLine
					t.cursorChar = targetChar
					m.notifySelectionChanged()
				}
			case msg.Button == tea.MouseButtonWheelUp:
				t.scrollOffset -= scrollAmount
				if t.scrollOffset < 0 {
					t.scrollOffset = 0
				}
			case msg.Button == tea.MouseButtonWheelDown:
				maxOffset := max(len(t.lines)-lo.contentHeight, 0)
				t.scrollOffset += scrollAmount
				if t.scrollOffset > maxOffset {
					t.scrollOffset = maxOffset
				}
			}
		}
		return m, nil
	case tea.KeyMsg:
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
			case msg.Type == tea.KeyEnter:
				val := t.commentInput.Value()
				if val != "" {
					t.comments[t.inputLine] = val
					m.notifyComment(t.inputLine, val)
				}
				t.inputMode = false
				t.commentInput.Reset()
				t.commentInput.Blur()
			default:
				t.commentInput, cmd = t.commentInput.Update(msg)
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

		if m.search.active {
			switch {
			case key.Matches(msg, m.keys.Cancel):
				m.search.close()
				return m, nil
			case msg.Type == tea.KeyEnter:
				if p := m.search.selectedPath(); p != "" {
					absPath := filepath.Join(m.rootDir, p)
					m.search.close()
					m.openFileByPath(absPath)
				}
				return m, nil
			default:
				cmd = m.search.update(msg)
				return m, cmd
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
				t.inputLine = t.cursorLine
				t.commentInput.Reset()
				if existing, ok := t.comments[t.cursorLine]; ok {
					t.commentInput.SetValue(existing)
				}
				t.commentInput.Focus()
			}
		case key.Matches(msg, m.keys.Copy):
			if hasTab && m.focusPane == paneEditor && t.selecting {
				text := t.selectedText()
				if err := clipboard.WriteAll(text); err != nil {
					m.statusMsg = fmt.Sprintf("Copy failed: %v", err)
				} else {
					n := strings.Count(text, "\n") + 1
					m.statusMsg = fmt.Sprintf("Copied %d lines", n)
				}
				return m, tea.Tick(statusClearTimeout, func(time.Time) tea.Msg {
					return statusClearMsg{}
				})
			}
		case key.Matches(msg, m.keys.ClearAll):
			if hasTab && m.focusPane == paneEditor {
				t.comments = make(map[int]string)
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
		case key.Matches(msg, m.keys.Search):
			return m, m.search.open(m.rootDir)
		}
	}

	lo := m.computeLayout()
	if m.focusPane == paneTree {
		m.adjustTreeScroll(lo.contentHeight)
	} else if hasTab && len(t.lines) > 0 {
		t.adjustScrollForCursor(lo.contentHeight)
	}

	return m, nil
}

// closeTab removes the tab at idx and adjusts activeTab.
func (m *Model) closeTab(idx int) {
	t := m.tabs[idx]
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
		if t.kind != diffTab {
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
