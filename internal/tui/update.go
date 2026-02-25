package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

const (
	scrollAmount        = 3
	headerHeight        = 1
	separatorWidth      = 3
	lineNumberWidth     = 4
	maxTreeWidthPercent = 70
)

// Init implements tea.Model.
func (m *Model) Init() tea.Cmd {
	return tea.Batch(m.watchFile(), m.watchDir())
}

// getTreeWidth returns the tree pane width.
func (m *Model) getTreeWidth() int {
	if m.treeWidth > 0 {
		tw := max(m.treeWidth, 15)
		maxWidth := m.width * maxTreeWidthPercent / 100
		if tw > maxWidth {
			tw = maxWidth
		}
		return tw
	}
	return m.width * 30 / 100
}

// getContentHeight returns the content area height.
func (m *Model) getContentHeight() int {
	return max(m.height-5, 5)
}

// adjustTreeScroll adjusts the tree scroll so the tree cursor
// stays visible.
func (m *Model) adjustTreeScroll() {
	contentHeight := m.getContentHeight()
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
	t := m.activeTabState()

	switch msg := msg.(type) {
	case fileChangedMsg:
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
		return m, m.watchFile()
	case treeChangedMsg:
		m.fileTree = buildFileTree(m.rootDir)
		if m.treeCursor >= len(m.fileTree) {
			m.treeCursor = max(0, len(m.fileTree)-1)
		}
		return m, m.watchDir()
	case IdeConnectedMsg:
		if t.filePath != "" && len(t.lines) > 0 {
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
		if t.filePath != "" && len(t.lines) > 0 && m.focusPane == paneEditor {
			m.notifySelectionChanged()
		}
	case tea.MouseMsg:
		treeWidth := m.getTreeWidth()

		borderX := treeWidth
		isBorderArea := msg.X >= borderX && msg.X <= borderX+2 && msg.Y >= headerHeight

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

		if msg.X < treeWidth && msg.Y >= headerHeight && msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress {
			treeIdx := msg.Y - headerHeight + m.treeScrollOffset
			if treeIdx >= 0 && treeIdx < len(m.fileTree) {
				m.treeCursor = treeIdx
				m.toggleTreeEntry(treeIdx)
			}
			return m, nil
		}

		if len(t.lines) == 0 {
			return m, nil
		}

		editorStartX := treeWidth + separatorWidth

		if msg.X >= editorStartX && msg.Y >= headerHeight {
			editorX := msg.X - editorStartX - lineNumberWidth
			editorY := msg.Y - headerHeight
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
					t.selecting = true
					m.lastMouseLine = targetLine
					m.lastMouseChar = targetChar
				case tea.MouseActionMotion:
					if targetLine != m.lastMouseLine || targetChar != m.lastMouseChar {
						t.cursorLine = targetLine
						t.cursorChar = targetChar
						m.lastMouseLine = targetLine
						m.lastMouseChar = targetChar
					}
				}
			case msg.Action == tea.MouseActionRelease:
				if t.selecting {
					t.cursorLine = targetLine
					t.cursorChar = targetChar
					if t.cursorLine == t.anchorLine && t.cursorChar == t.anchorChar {
						t.selecting = false
					}
					m.notifySelectionChanged()
				}
			case msg.Button == tea.MouseButtonWheelUp:
				t.scrollOffset -= scrollAmount
				if t.scrollOffset < 0 {
					t.scrollOffset = 0
				}
			case msg.Button == tea.MouseButtonWheelDown:
				contentHeight := m.getContentHeight()
				maxOffset := max(len(t.lines)-contentHeight, 0)
				t.scrollOffset += scrollAmount
				if t.scrollOffset > maxOffset {
					t.scrollOffset = maxOffset
				}
			}
		}
		return m, nil
	case tea.KeyMsg:
		var cmd tea.Cmd
		if t.inputMode {
			switch {
			case key.Matches(msg, m.keys.Quit):
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

		switch {
		case key.Matches(msg, m.keys.Quit):
			if t.selecting {
				t.selecting = false
				t.lineSelect = false
				m.notifyClearSelection()
				return m, nil
			}
			return m, tea.Quit
		case key.Matches(msg, m.keys.SwitchPane):
			if len(t.lines) > 0 {
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
			} else {
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
			} else {
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
			} else {
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
			} else {
				if t.cursorLine < len(t.lines)-1 {
					t.cursorLine++
					t.cursorChar = min(t.cursorChar, t.lineLen(t.cursorLine))
					t.syncAnchorToCursor()
					m.notifySelectionChanged()
				}
			}
		case key.Matches(msg, m.keys.CharSelect):
			if m.focusPane == paneEditor && len(t.lines) > 0 {
				if t.selecting && !t.lineSelect {
					t.selecting = false
					m.notifyClearSelection()
				} else if t.selecting && t.lineSelect {
					t.lineSelect = false
					m.notifySelectionChanged()
				} else {
					t.startSelecting()
				}
			}
		case key.Matches(msg, m.keys.LineSelect):
			if m.focusPane == paneEditor && len(t.lines) > 0 {
				if t.selecting && t.lineSelect {
					t.selecting = false
					t.lineSelect = false
					m.notifyClearSelection()
				} else if t.selecting && !t.lineSelect {
					t.lineSelect = true
					m.notifySelectionChanged()
				} else {
					t.startSelecting()
					t.lineSelect = true
					m.notifySelectionChanged()
				}
			}
		case key.Matches(msg, m.keys.Comment):
			if m.focusPane == paneEditor && len(t.lines) > 0 {
				t.inputMode = true
				t.inputLine = t.cursorLine
				t.commentInput.Reset()
				if existing, ok := t.comments[t.cursorLine]; ok {
					t.commentInput.SetValue(existing)
				}
				t.commentInput.Focus()
			}
		case key.Matches(msg, m.keys.ClearAll):
			if m.focusPane == paneEditor {
				t.comments = make(map[int]string)
			}
		}
	}

	if m.focusPane == paneTree {
		m.adjustTreeScroll()
	} else if len(t.lines) > 0 {
		t.adjustScrollForCursor(m.getContentHeight())
	}

	return m, nil
}
