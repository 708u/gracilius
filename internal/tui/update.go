package tui

import (
	"slices"
	"strings"
	"time"
	"unicode"

	"charm.land/bubbles/v2/help"
	tea "charm.land/bubbletea/v2"
)

const (
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

	switch msg := msg.(type) {
	case tea.KeyboardEnhancementsMsg:
		m.enhancedKeyboard = msg.SupportsKeyDisambiguation()
		return m, nil
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
		return m.handleFileChanged(msg)
	case treeChangedMsg:
		return m.handleTreeChanged()
	case OpenDiffMsg:
		return m.handleOpenDiff(msg)
	case CloseDiffMsg:
		return m.handleCloseDiff()
	case quitTimeoutMsg:
		return m.handleQuitTimeout()
	case statusClearMsg:
		return m.handleStatusClear()
	case IdeConnectedMsg:
		return m.handleIdeConnected()
	case tea.WindowSizeMsg:
		return m.handleWindowSize(msg)
	case tea.MouseClickMsg:
		return m.handleMouseClick(msg)
	case tea.MouseMotionMsg:
		return m.handleMouseMotion(msg)
	case tea.MouseReleaseMsg:
		return m.handleMouseRelease(msg)
	case tea.MouseWheelMsg:
		return m.handleMouseWheel(msg)
	case tea.KeyPressMsg:
		return m.handleKeyPress(msg)
	}

	return m, nil
}

// editorTarget converts mouse coordinates to editor line and character.
func (m *Model) editorTarget(t *tab, lo layout, mouseX, mouseY int) (int, int) {
	editorX := mouseX - lo.editorStartX - lo.lineNumWidth
	editorY := mouseY - contentStartY

	targetLine := t.vp.YOffset() + editorY
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
