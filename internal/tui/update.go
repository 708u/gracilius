package tui

import (
	"log"

	tea "github.com/charmbracelet/bubbletea"
)

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.watchFile(), m.watchDir())
}

// lineLen returns the rune-length of the given line.
func (m Model) lineLen(line int) int {
	if line < 0 || line >= len(m.lines) {
		return 0
	}
	return len([]rune(m.lines[line]))
}

// getTreeWidth returns the tree pane width.
func (m Model) getTreeWidth() int {
	if m.treeWidth > 0 {
		tw := m.treeWidth
		if tw < 15 {
			tw = 15
		}
		maxWidth := m.width * 70 / 100
		if tw > maxWidth {
			tw = maxWidth
		}
		return tw
	}
	return m.width * 30 / 100
}

// getContentHeight returns the content area height.
func (m Model) getContentHeight() int {
	contentHeight := m.height - 6
	if contentHeight < 5 {
		contentHeight = 5
	}
	return contentHeight
}

// getScrollOffset returns the current scroll offset.
func (m Model) getScrollOffset() int {
	return m.scrollOffset
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

	maxOffset := len(m.lines) - contentHeight
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.scrollOffset > maxOffset {
		m.scrollOffset = maxOffset
	}
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case fileChangedMsg:
		m.lines = msg.lines
		m.previewLines = nil
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
	case FilePreviewMsg:
		if m.filePath != msg.FilePath {
			if err := m.loadFile(msg.FilePath); err != nil {
				m.err = err
				return m, nil
			}
			m.focusPane = 1
		}
		m.previewLines = msg.Lines

		diffLines := computeLineDiff(m.lines, m.previewLines)
		diffLineNum := 0
		for _, dl := range diffLines {
			if dl.op == '+' || dl.op == '-' {
				break
			}
			if dl.op == ' ' {
				diffLineNum++
			}
		}
		m.cursorLine = diffLineNum

		return m, nil
	case ClearPreviewMsg:
		m.previewLines = nil
		return m, nil
	case IdeConnectedMsg:
		if m.filePath != "" && len(m.lines) > 0 {
			m.notifySelectionChanged()
		}
		return m, nil
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		maxWidth := m.width * 70 / 100
		if m.treeWidth > maxWidth {
			m.treeWidth = maxWidth
		}
		if m.filePath != "" && len(m.lines) > 0 && m.focusPane == 1 {
			m.notifySelectionChanged()
		}
	case tea.MouseMsg:
		treeWidth := m.getTreeWidth()
		headerHeight := 2

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
			treeY := msg.Y - headerHeight
			log.Printf("Tree click: treeY=%d, len(fileTree)=%d", treeY, len(m.fileTree))
			if treeY < len(m.fileTree) {
				m.treeCursor = treeY
				entry := m.fileTree[treeY]
				log.Printf("Selected entry: name=%s, path=%s, isDir=%v", entry.name, entry.path, entry.isDir)
				if entry.isDir {
					if entry.expanded {
						m.fileTree = collapseDir(m.fileTree, treeY)
					} else {
						m.fileTree = expandDir(m.fileTree, treeY)
					}
				} else {
					if err := m.loadFile(entry.path); err == nil {
						log.Printf("loadFile success: lines=%d, scrollOffset=%d", len(m.lines), m.scrollOffset)
						m.focusPane = 1
						m.notifySelectionChanged()
					} else {
						log.Printf("loadFile error: %v", err)
					}
				}
			}
			return m, nil
		}

		if len(m.lines) == 0 {
			return m, nil
		}

		editorStartX := treeWidth + 3

		if msg.X >= editorStartX && msg.Y >= headerHeight {
			editorX := msg.X - editorStartX - 4
			editorY := msg.Y - headerHeight
			offset := m.getScrollOffset()
			targetLine := offset + editorY

			if targetLine >= len(m.lines) {
				targetLine = len(m.lines) - 1
			}
			if targetLine < 0 {
				targetLine = 0
			}

			targetChar := editorX
			if targetChar < 0 {
				targetChar = 0
			}
			if targetLine < len(m.lines) {
				runeLen := len([]rune(m.lines[targetLine]))
				if targetChar > runeLen {
					targetChar = runeLen
				}
			}

			switch msg.Type {
			case tea.MouseLeft:
				m.focusPane = 1
				if msg.Action == tea.MouseActionPress {
					m.cursorLine = targetLine
					m.cursorChar = targetChar
					m.anchorLine = targetLine
					m.anchorChar = targetChar
					m.selecting = true
					m.lastMouseLine = targetLine
					m.lastMouseChar = targetChar
				} else if msg.Action == tea.MouseActionMotion {
					if targetLine != m.lastMouseLine || targetChar != m.lastMouseChar {
						m.cursorLine = targetLine
						m.cursorChar = targetChar
						m.lastMouseLine = targetLine
						m.lastMouseChar = targetChar
					}
				}
			case tea.MouseRelease:
				if m.selecting {
					m.cursorLine = targetLine
					m.cursorChar = targetChar
					if m.cursorLine == m.anchorLine && m.cursorChar == m.anchorChar {
						m.selecting = false
					}
					m.notifySelectionChanged()
				}
			case tea.MouseWheelUp:
				scrollAmount := 3
				m.scrollOffset -= scrollAmount
				if m.scrollOffset < 0 {
					m.scrollOffset = 0
				}
			case tea.MouseWheelDown:
				scrollAmount := 3
				contentHeight := m.getContentHeight()
				maxOffset := len(m.lines) - contentHeight
				if maxOffset < 0 {
					maxOffset = 0
				}
				m.scrollOffset += scrollAmount
				if m.scrollOffset > maxOffset {
					m.scrollOffset = maxOffset
				}
			}
		}
		return m, nil
	case tea.KeyMsg:
		if m.inputMode {
			switch msg.Type {
			case tea.KeyEsc:
				m.inputMode = false
				m.commentInput = ""
			case tea.KeyEnter:
				if m.commentInput != "" {
					m.comments[m.inputLine] = m.commentInput
					m.notifyComment(m.inputLine, m.commentInput)
				}
				m.inputMode = false
				m.commentInput = ""
			case tea.KeyBackspace:
				if len(m.commentInput) > 0 {
					runes := []rune(m.commentInput)
					m.commentInput = string(runes[:len(runes)-1])
				}
			case tea.KeyRunes, tea.KeySpace:
				m.commentInput += msg.String()
			}
			return m, nil
		}

		switch msg.Type {
		case tea.KeyEsc, tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyTab:
			if len(m.lines) > 0 {
				if m.focusPane == 1 {
					m.notifyClearSelection()
				}
				m.focusPane = (m.focusPane + 1) % 2
			}
		case tea.KeyEnter:
			if m.focusPane == 0 && len(m.fileTree) > 0 {
				entry := m.fileTree[m.treeCursor]
				if entry.isDir {
					if entry.expanded {
						m.fileTree = collapseDir(m.fileTree, m.treeCursor)
					} else {
						m.fileTree = expandDir(m.fileTree, m.treeCursor)
					}
				} else {
					if err := m.loadFile(entry.path); err != nil {
						m.err = err
					} else {
						m.focusPane = 1
						m.notifySelectionChanged()
					}
				}
			}
		case tea.KeyLeft:
			if m.focusPane == 0 {
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
				if !m.selecting {
					m.anchorLine = m.cursorLine
					m.anchorChar = m.cursorChar
				}
				m.notifySelectionChanged()
			}
		case tea.KeyRight:
			if m.focusPane == 0 {
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
				if !m.selecting {
					m.anchorLine = m.cursorLine
					m.anchorChar = m.cursorChar
				}
				m.notifySelectionChanged()
			}
		case tea.KeyUp:
			if m.focusPane == 0 {
				if m.treeCursor > 0 {
					m.treeCursor--
				}
			} else {
				if m.cursorLine > 0 {
					m.cursorLine--
					m.cursorChar = min(m.cursorChar, m.lineLen(m.cursorLine))
					if !m.selecting {
						m.anchorLine = m.cursorLine
						m.anchorChar = m.cursorChar
					}
					m.notifySelectionChanged()
				}
			}
		case tea.KeyDown:
			if m.focusPane == 0 {
				if m.treeCursor < len(m.fileTree)-1 {
					m.treeCursor++
				}
			} else {
				if m.cursorLine < len(m.lines)-1 {
					m.cursorLine++
					m.cursorChar = min(m.cursorChar, m.lineLen(m.cursorLine))
					if !m.selecting {
						m.anchorLine = m.cursorLine
						m.anchorChar = m.cursorChar
					}
					m.notifySelectionChanged()
				}
			}
		case tea.KeyShiftUp:
			if m.cursorLine > 0 {
				if !m.selecting {
					m.selecting = true
					m.anchorLine = m.cursorLine
					m.anchorChar = m.cursorChar
				}
				m.cursorLine--
				m.cursorChar = min(m.cursorChar, m.lineLen(m.cursorLine))
				m.notifySelectionChanged()
			}
		case tea.KeyShiftDown:
			if m.cursorLine < len(m.lines)-1 {
				if !m.selecting {
					m.selecting = true
					m.anchorLine = m.cursorLine
					m.anchorChar = m.cursorChar
				}
				m.cursorLine++
				m.cursorChar = min(m.cursorChar, m.lineLen(m.cursorLine))
				m.notifySelectionChanged()
			}
		case tea.KeyShiftLeft:
			if !m.selecting {
				m.selecting = true
				m.anchorLine = m.cursorLine
				m.anchorChar = m.cursorChar
			}
			if m.cursorChar > 0 {
				m.cursorChar--
			} else if m.cursorLine > 0 {
				m.cursorLine--
				m.cursorChar = m.lineLen(m.cursorLine)
			}
			m.notifySelectionChanged()
		case tea.KeyShiftRight:
			if !m.selecting {
				m.selecting = true
				m.anchorLine = m.cursorLine
				m.anchorChar = m.cursorChar
			}
			if m.cursorChar < m.lineLen(m.cursorLine) {
				m.cursorChar++
			} else if m.cursorLine < len(m.lines)-1 {
				m.cursorLine++
				m.cursorChar = 0
			}
			m.notifySelectionChanged()
		default:
			if msg.String() == "i" && m.focusPane == 1 && len(m.lines) > 0 {
				m.inputMode = true
				m.inputLine = m.cursorLine
				m.commentInput = ""
				if existing, ok := m.comments[m.cursorLine]; ok {
					m.commentInput = existing
				}
			}
			if msg.String() == "D" && m.focusPane == 1 {
				m.comments = make(map[int]string)
			}
		}
	}

	if m.focusPane == 1 && len(m.lines) > 0 {
		m.adjustScrollForCursor()
	}

	return m, nil
}
