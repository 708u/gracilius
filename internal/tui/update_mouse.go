package tui

import (
	"path/filepath"

	tea "charm.land/bubbletea/v2"
)

// tabIndexAtX returns the tab index at screen X coordinate, or -1 if none.
func (m *Model) tabIndexAtX(x int) int {
	lo := m.computeLayout()
	pos := lo.editorStartX
	for i, t := range m.tabs {
		if i > 0 {
			pos += tabSeparatorWidth
		}
		label := tabLabel(t)
		w := len([]rune(label))
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

	if msg.Button == tea.MouseLeft &&
		msg.Y >= headerHeight && msg.Y < headerHeight+tabBarHeight {
		if idx := m.tabIndexAtX(msg.X); idx >= 0 {
			m.activeTab = idx
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
}

// handleMouseMotion handles mouse drag events.
func (m *Model) handleMouseMotion(msg tea.MouseMotionMsg) (tea.Model, tea.Cmd) {
	if m.resizingPane {
		m.treeWidth = msg.X
		return m, nil
	}

	t, hasTab := m.activeTabState()
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
}

// handleMouseWheel handles mouse scroll events.
func (m *Model) handleMouseWheel(msg tea.MouseWheelMsg) (tea.Model, tea.Cmd) {
	if m.openFile.active {
		return m, nil
	}
	t, hasTab := m.activeTabState()
	if !hasTab {
		return m, nil
	}
	lo := m.computeLayout()
	if msg.X >= lo.editorStartX && msg.Y >= contentStartY {
		if t.diffViewData != nil {
			delta := 3
			if msg.Button == tea.MouseWheelUp {
				delta = -3
			}
			newOff := t.vp.YOffset() + delta
			maxOff := t.diffMaxOffset()
			newOff = max(0, min(newOff, maxOff))
			t.vp.SetYOffset(newOff)
		} else if len(t.lines) > 0 {
			t.vp, _ = t.vp.Update(msg)
			maxOff := t.maxScrollOffset(lo.contentHeight, lo.textWidth)
			if t.vp.YOffset() > maxOff {
				t.vp.SetYOffset(maxOff)
			}
		}
	}
	return m, nil
}
