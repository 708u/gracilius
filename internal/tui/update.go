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

// lineLen returns the rune-length of the given line.
func (m *Model) lineLen(line int) int {
	if line < 0 || line >= len(m.lines) {
		return 0
	}
	return len([]rune(m.lines[line]))
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

// adjustScrollForCursor adjusts the scroll so the cursor stays visible.
func (m *Model) adjustScrollForCursor() {
	contentHeight := m.getContentHeight()
	margin := contentHeight / 5

	if m.cursorLine < m.scrollOffset+margin {
		m.scrollOffset = m.cursorLine - margin
	}

	if m.cursorLine >= m.scrollOffset+contentHeight-margin {
		m.scrollOffset = m.cursorLine - contentHeight + margin + 1
	}

	maxOffset := max(len(m.lines)-contentHeight, 0)
	if m.scrollOffset > maxOffset {
		m.scrollOffset = maxOffset
	}
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
}

// Update implements tea.Model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case fileChangedMsg:
		m.lines = msg.lines
		m.highlightedLines = highlightFile(
			m.filePath, strings.Join(msg.lines, "\n"),
		)
		if m.cursorLine >= len(m.lines) {
			m.cursorLine = max(0, len(m.lines)-1)
		}
		if m.cursorLine < len(m.lines) {
			m.cursorChar = min(m.cursorChar, len(m.lines[m.cursorLine]))
		}
		if m.filePath != "" {
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
		if m.filePath != "" && len(m.lines) > 0 {
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
		if m.filePath != "" && len(m.lines) > 0 && m.focusPane == paneEditor {
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

		if len(m.lines) == 0 {
			return m, nil
		}

		editorStartX := treeWidth + separatorWidth

		if msg.X >= editorStartX && msg.Y >= headerHeight {
			editorX := msg.X - editorStartX - lineNumberWidth
			editorY := msg.Y - headerHeight
			offset := m.scrollOffset
			targetLine := offset + editorY

			if targetLine >= len(m.lines) {
				targetLine = len(m.lines) - 1
			}
			if targetLine < 0 {
				targetLine = 0
			}

			targetChar := max(editorX, 0)
			if targetLine < len(m.lines) {
				runeLen := len([]rune(m.lines[targetLine]))
				if targetChar > runeLen {
					targetChar = runeLen
				}
			}

			switch {
			case msg.Button == tea.MouseButtonLeft:
				m.focusPane = paneEditor
				switch msg.Action {
				case tea.MouseActionPress:
					m.cursorLine = targetLine
					m.cursorChar = targetChar
					m.anchorLine = targetLine
					m.anchorChar = targetChar
					m.selecting = true
					m.lastMouseLine = targetLine
					m.lastMouseChar = targetChar
				case tea.MouseActionMotion:
					if targetLine != m.lastMouseLine || targetChar != m.lastMouseChar {
						m.cursorLine = targetLine
						m.cursorChar = targetChar
						m.lastMouseLine = targetLine
						m.lastMouseChar = targetChar
					}
				}
			case msg.Action == tea.MouseActionRelease:
				if m.selecting {
					m.cursorLine = targetLine
					m.cursorChar = targetChar
					if m.cursorLine == m.anchorLine && m.cursorChar == m.anchorChar {
						m.selecting = false
					}
					m.notifySelectionChanged()
				}
			case msg.Button == tea.MouseButtonWheelUp:
				m.scrollOffset -= scrollAmount
				if m.scrollOffset < 0 {
					m.scrollOffset = 0
				}
			case msg.Button == tea.MouseButtonWheelDown:
				contentHeight := m.getContentHeight()
				maxOffset := max(len(m.lines)-contentHeight, 0)
				m.scrollOffset += scrollAmount
				if m.scrollOffset > maxOffset {
					m.scrollOffset = maxOffset
				}
			}
		}
		return m, nil
	case tea.KeyMsg:
		var cmd tea.Cmd
		if m.inputMode {
			switch {
			case key.Matches(msg, m.keys.Quit):
				m.inputMode = false
				m.commentInput.Reset()
				m.commentInput.Blur()
			case msg.Type == tea.KeyEnter:
				val := m.commentInput.Value()
				if val != "" {
					m.comments[m.inputLine] = val
					m.notifyComment(m.inputLine, val)
				}
				m.inputMode = false
				m.commentInput.Reset()
				m.commentInput.Blur()
			default:
				m.commentInput, cmd = m.commentInput.Update(msg)
				return m, cmd
			}
			return m, nil
		}

		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.SwitchPane):
			if len(m.lines) > 0 {
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
				if m.cursorChar > 0 {
					m.cursorChar--
				} else if m.cursorLine > 0 {
					m.cursorLine--
					m.cursorChar = m.lineLen(m.cursorLine)
				}
				m.syncAnchorToCursor()
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
				if m.cursorChar < m.lineLen(m.cursorLine) {
					m.cursorChar++
				} else if m.cursorLine < len(m.lines)-1 {
					m.cursorLine++
					m.cursorChar = 0
				}
				m.syncAnchorToCursor()
				m.notifySelectionChanged()
			}
		case key.Matches(msg, m.keys.Up):
			if m.focusPane == paneTree {
				if m.treeCursor > 0 {
					m.treeCursor--
				}
			} else {
				if m.cursorLine > 0 {
					m.cursorLine--
					m.cursorChar = min(m.cursorChar, m.lineLen(m.cursorLine))
					m.syncAnchorToCursor()
					m.notifySelectionChanged()
				}
			}
		case key.Matches(msg, m.keys.Down):
			if m.focusPane == paneTree {
				if m.treeCursor < len(m.fileTree)-1 {
					m.treeCursor++
				}
			} else {
				if m.cursorLine < len(m.lines)-1 {
					m.cursorLine++
					m.cursorChar = min(m.cursorChar, m.lineLen(m.cursorLine))
					m.syncAnchorToCursor()
					m.notifySelectionChanged()
				}
			}
		case key.Matches(msg, m.keys.ShiftUp):
			if m.cursorLine > 0 {
				m.startSelecting()
				m.cursorLine--
				m.cursorChar = min(m.cursorChar, m.lineLen(m.cursorLine))
				m.notifySelectionChanged()
			}
		case key.Matches(msg, m.keys.ShiftDown):
			if m.cursorLine < len(m.lines)-1 {
				m.startSelecting()
				m.cursorLine++
				m.cursorChar = min(m.cursorChar, m.lineLen(m.cursorLine))
				m.notifySelectionChanged()
			}
		case key.Matches(msg, m.keys.ShiftLeft):
			m.startSelecting()
			if m.cursorChar > 0 {
				m.cursorChar--
			} else if m.cursorLine > 0 {
				m.cursorLine--
				m.cursorChar = m.lineLen(m.cursorLine)
			}
			m.notifySelectionChanged()
		case key.Matches(msg, m.keys.ShiftRight):
			m.startSelecting()
			if m.cursorChar < m.lineLen(m.cursorLine) {
				m.cursorChar++
			} else if m.cursorLine < len(m.lines)-1 {
				m.cursorLine++
				m.cursorChar = 0
			}
			m.notifySelectionChanged()
		case key.Matches(msg, m.keys.Comment):
			if m.focusPane == paneEditor && len(m.lines) > 0 {
				m.inputMode = true
				m.inputLine = m.cursorLine
				m.commentInput.Reset()
				if existing, ok := m.comments[m.cursorLine]; ok {
					m.commentInput.SetValue(existing)
				}
				m.commentInput.Focus()
			}
		case key.Matches(msg, m.keys.ClearAll):
			if m.focusPane == paneEditor {
				m.comments = make(map[int]string)
			}
		}
	}

	if m.focusPane == paneTree {
		m.adjustTreeScroll()
	} else if len(m.lines) > 0 {
		m.adjustScrollForCursor()
	}

	return m, nil
}
