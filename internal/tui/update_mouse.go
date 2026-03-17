package tui

import (
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// tabIndexAtX returns the tab index at screen X coordinate, or -1 if none.
func (m *Model) tabIndexAtX(lo layout, x int) int {
	pos := lo.editorStartX
	for i, t := range m.tabs {
		if i > 0 {
			pos += tabSeparatorWidth
		}
		label := tabLabel(t)
		w := ansi.StringWidth(label)
		if x >= pos && x < pos+w {
			return i
		}
		pos += w
	}
	return -1
}

// handleMouseClick handles mouse click events.
func (m *Model) handleMouseClick(msg tea.MouseClickMsg) (tea.Model, tea.Cmd) {
	t, hasTab := m.activeTabState()

	lo := m.computeLayout()

	if m.projectSearch.active {
		if msg.Button == tea.MouseLeft {
			absPath, line, closeOverlay := m.projectSearch.handleClick(msg.X, msg.Y, m.width, m.height)
			if absPath != "" {
				m.projectSearch.close()
				m.openFileAtLine(absPath, line)
			} else if closeOverlay {
				m.projectSearch.close()
			}
		}
		return m, nil
	}

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

	// Tab bar click: Y < paneHeaderRows and X in right pane area.
	if msg.Button == tea.MouseLeft &&
		msg.Y < paneHeaderRows && (!m.sidebarVisible || msg.X >= lo.editorStartX) {
		if idx := m.tabIndexAtX(lo, msg.X); idx >= 0 {
			m.activeTab = idx
		}
		return m, nil
	}

	borderX := lo.treeWidth
	isBorderArea := m.sidebarVisible && msg.X >= borderX && msg.X <= borderX+2 && msg.Y >= paneHeaderRows
	if isBorderArea && msg.Button == tea.MouseLeft {
		m.resizingPane = true
		return m, nil
	}

	panelBodyY := m.leftPaneHeaderRows()
	if m.sidebarVisible && msg.X < lo.treeWidth && msg.Y >= panelBodyY && msg.Button == tea.MouseLeft {
		switch m.activePanel {
		case panelGitDiff:
			gs := m.gitState()
			rowIdx := msg.Y - panelBodyY + gs.scrollOffset
			if rowIdx >= 0 && rowIdx < len(gs.visualRows) {
				row := gs.visualRows[rowIdx]
				if row.isFileRow() {
					gs.cursor = row.entryIdx
					m.openGitDiffEntry()
				}
			}
		default:
			treeIdx := msg.Y - panelBodyY + m.treeScrollOffset
			if treeIdx >= 0 && treeIdx < len(m.fileTree) {
				m.treeCursor = treeIdx
				m.toggleTreeEntry(treeIdx)
			}
		}
		return m, nil
	}

	if !hasTab {
		return m, nil
	}

	// Diff tab click: set diff cursor by visual line → row mapping.
	if t.diffViewData != nil && msg.Button == tea.MouseLeft && msg.X >= lo.editorStartX && msg.Y >= paneHeaderRows {
		visualLine := msg.Y - paneHeaderRows + t.vp.YOffset()
		row := t.diffVisualLineToRow(visualLine)
		m.focusPane = paneEditor
		t.diffCursor = row
		t.diffAnchor = row
		t.diffSelecting = false
		t.diffSide = m.diffSideFromX(lo, msg.X)
		t.snapDiffSide()
		m.mouseDown = true
		m.lastMouseLine = row
		m.notifySelectionChanged()
		return m, nil
	}

	if len(t.lines) == 0 {
		return m, nil
	}

	if msg.Button == tea.MouseLeft && msg.X >= lo.editorStartX && msg.Y >= paneHeaderRows {
		targetLine, targetChar := m.editorTarget(t, lo, msg.X, msg.Y)
		m.focusPane = paneEditor
		t.cursorLine = targetLine
		t.cursorChar = targetChar
		t.anchorLine = targetLine
		t.anchorChar = targetChar
		t.selecting = false
		m.mouseDown = true
		m.lastMouseLine = targetLine
		m.lastMouseChar = targetChar
	}
	return m, nil
}

// handleMouseMotion handles mouse drag events.
func (m *Model) handleMouseMotion(msg tea.MouseMotionMsg) (tea.Model, tea.Cmd) {
	if m.resizingPane {
		m.treeWidth = msg.X
		return m, nil
	}

	t, hasTab := m.activeTabState()
	if !hasTab || !m.mouseDown {
		return m, nil
	}

	lo := m.computeLayout()

	// Diff tab drag: select rows.
	if t.diffViewData != nil && msg.X >= lo.editorStartX && msg.Y >= paneHeaderRows {
		visualLine := msg.Y - paneHeaderRows + t.vp.YOffset()
		row := t.diffVisualLineToRow(visualLine)
		if row != m.lastMouseLine {
			t.diffSelecting = true
			t.diffCursor = row
			t.snapDiffSide()
			m.lastMouseLine = row
		}
		return m, nil
	}

	if len(t.lines) == 0 {
		return m, nil
	}

	if msg.X >= lo.editorStartX && msg.Y >= paneHeaderRows {
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
}

// handleMouseRelease handles mouse button release events.
func (m *Model) handleMouseRelease(msg tea.MouseReleaseMsg) (tea.Model, tea.Cmd) {
	if m.openFile.active {
		return m, nil
	}
	if m.resizingPane {
		m.resizingPane = false
		return m, nil
	}

	wasDown := m.mouseDown
	m.mouseDown = false

	t, hasTab := m.activeTabState()
	if !hasTab {
		return m, nil
	}

	// Diff tab release: finalize selection and notify.
	if wasDown && t.diffViewData != nil && t.diffSelecting {
		lo := m.computeLayout()
		if msg.X >= lo.editorStartX && msg.Y >= paneHeaderRows {
			visualLine := msg.Y - paneHeaderRows + t.vp.YOffset()
			row := t.diffVisualLineToRow(visualLine)
			t.diffCursor = row
			t.snapDiffSide()
		}
		m.notifySelectionChanged()
		return m, nil
	}

	if len(t.lines) == 0 {
		return m, nil
	}

	if wasDown && t.selecting {
		lo := m.computeLayout()
		if msg.X >= lo.editorStartX && msg.Y >= paneHeaderRows {
			targetLine, targetChar := m.editorTarget(t, lo, msg.X, msg.Y)
			t.cursorLine = targetLine
			t.cursorChar = targetChar
		}
		m.notifySelectionChanged()
	}
	return m, nil
}

// diffSideFromX returns the diff side based on the mouse X coordinate.
func (m *Model) diffSideFromX(lo layout, x int) diffSide {
	sideWidth := (lo.editorWidth - diffSeparatorWidth) / 2
	if x-lo.editorStartX < sideWidth {
		return diffSideOld
	}
	return diffSideNew
}

// handleMouseWheel handles mouse scroll events.
func (m *Model) handleMouseWheel(msg tea.MouseWheelMsg) (tea.Model, tea.Cmd) {
	if m.openFile.active {
		return m, nil
	}
	lo := m.computeLayout()

	// Left pane scrolling.
	if m.sidebarVisible && msg.X < lo.treeWidth {
		delta := 3
		if msg.Button == tea.MouseWheelUp {
			delta = -3
		}
		bodyHeight := m.leftPaneBodyHeight(lo)
		switch m.activePanel {
		case panelGitDiff:
			gs := m.gitState()
			gs.scrollOffset = max(0, gs.scrollOffset+delta)
			maxOff := max(len(gs.visualRows)-bodyHeight, 0)
			gs.scrollOffset = min(gs.scrollOffset, maxOff)
		default:
			m.treeScrollOffset = max(0, m.treeScrollOffset+delta)
			maxOff := max(len(m.fileTree)-bodyHeight, 0)
			m.treeScrollOffset = min(m.treeScrollOffset, maxOff)
		}
		return m, nil
	}

	t, hasTab := m.activeTabState()
	if !hasTab {
		return m, nil
	}
	if msg.X >= lo.editorStartX && msg.Y >= paneHeaderRows {
		if t.diffViewData != nil {
			t.vp, _ = t.vp.Update(msg)
		} else if len(t.lines) > 0 {
			t.vp, _ = t.vp.Update(msg)
			maxOff := t.maxScrollOffset(lo.paneBodyHeight, lo.textWidth)
			if t.vp.YOffset() > maxOff {
				t.vp.SetYOffset(maxOff)
			}
		}
	}
	return m, nil
}
