package tui

import (
	"log"
	"os"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
)

// handleFileChanged processes file change notifications.
func (m *Model) handleFileChanged(msg fileChangedMsg) (tea.Model, tea.Cmd) {
	// Update the active file tab if its path matches.
	if t, ok := m.activeTabState(); ok &&
		t.kind == fileTab && t.filePath == msg.path {
		t.lines = msg.lines
		t.syncContent(msg.lines)
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

	// Update diff tabs whose old-side file matches.
	oldSource := strings.Join(msg.lines, "\n")
	for _, t := range m.tabs {
		if t.kind != diffTab || t.filePath != msg.path {
			continue
		}
		t.diffOldSource = oldSource
		t.diffOldHighlights = highlightFile(t.filePath, oldSource, m.theme)
		t.diffViewData = buildDiffData(msg.lines, t.lines)
		if t.vp.Width() > diffSeparatorWidth {
			off := t.vp.YOffset()
			t.renderDiffContent(m.theme, t.vp.Width())
			t.vp.SetYOffset(off)
		}
	}

	if m.search.query != "" {
		m.refreshSearchMatches()
	}

	cmd := m.watchFile()
	return m, cmd
}

// handleCommentsChanged reloads comments from the store for all open tabs.
func (m *Model) handleCommentsChanged() (tea.Model, tea.Cmd) {
	for _, t := range m.tabs {
		if t.filePath == "" || t.kind != fileTab {
			continue
		}
		stored, err := m.commentRepo.List(t.filePath, false)
		if err != nil {
			log.Printf("Failed to reload comments for %s: %v", t.filePath, err)
			continue
		}
		t.comments = stored
	}
	cmd := m.watchComments()
	return m, cmd
}

// handleTreeChanged processes directory tree change notifications.
func (m *Model) handleTreeChanged() (tea.Model, tea.Cmd) {
	expanded := expandedPaths(m.fileTree)
	m.fileTree = buildFileTree(m.rootDir)
	m.fileTree = restoreExpanded(m.fileTree, expanded)
	if m.treeCursor >= len(m.fileTree) {
		m.treeCursor = max(0, len(m.fileTree)-1)
	}
	cmds := []tea.Cmd{m.watchDir()}
	if m.gitLoaded {
		cmds = append(cmds, m.scheduleGitSync())
	}
	return m, tea.Batch(cmds...)
}

// handleGitIndexChanged reloads git changes when .git/index changes
// (e.g. after commit, add, reset).
func (m *Model) handleGitIndexChanged() (tea.Model, tea.Cmd) {
	cmds := []tea.Cmd{m.watchGitIndex()}
	if m.gitLoaded {
		cmds = append(cmds, m.scheduleGitSync())
	}
	return m, tea.Batch(cmds...)
}

// scheduleGitSync bumps the generation counter and schedules a
// debounced git reload. Only the latest scheduled sync fires.
func (m *Model) scheduleGitSync() tea.Cmd {
	m.gitSyncGen++
	gen := m.gitSyncGen
	return tea.Tick(gitSyncDebounce, func(time.Time) tea.Msg {
		return gitSyncMsg{gen: gen}
	})
}

// handleGitSync executes the git reload if the generation still matches.
func (m *Model) handleGitSync(msg gitSyncMsg) (tea.Model, tea.Cmd) {
	if msg.gen != m.gitSyncGen {
		return m, nil
	}
	cmd := m.loadGitChanges()
	return m, cmd
}

// handleOpenDiff opens a new diff tab.
func (m *Model) handleOpenDiff(msg OpenDiffMsg) (tea.Model, tea.Cmd) {
	newLines := splitLines([]byte(msg.Contents))
	dt := newDiffTab(msg.FilePath, newLines, msg.Accept, msg.Reject)
	dt.syncContent(newLines)
	dt.highlightedLines = highlightFile(msg.FilePath, msg.Contents, m.theme)

	var oldLines []string
	if oldContent, err := os.ReadFile(msg.FilePath); err == nil {
		oldLines = splitLines(oldContent)
	}

	if len(oldLines) > 0 {
		oldSource := strings.Join(oldLines, "\n")
		dt.diffOldHighlights = highlightFile(msg.FilePath, oldSource, m.theme)
		dt.diffOldSource = oldSource
	}
	dt.diffNewHighlights = highlightFile(msg.FilePath, msg.Contents, m.theme)

	dt.diffViewData = buildDiffData(oldLines, newLines)
	lo := m.computeLayout()
	dt.vp.SetWidth(lo.editorWidth)
	dt.vp.SetHeight(lo.contentHeight)
	dt.initDiffContent(m.theme, lo.editorWidth, lo.contentHeight)

	m.tabs = append(m.tabs, dt)
	m.activeTab = len(m.tabs) - 1
	m.focusPane = paneEditor

	if m.watcher != nil {
		if err := m.watcher.Add(msg.FilePath); err != nil {
			log.Printf("Failed to watch diff file: %v", err)
		}
	}

	return m, nil
}

// handleCloseDiff closes all diff tabs.
func (m *Model) handleCloseDiff() (tea.Model, tea.Cmd) {
	m.closeDiffTabs()
	return m, nil
}

// handleQuitTimeout resets the quit confirmation state.
func (m *Model) handleQuitTimeout() (tea.Model, tea.Cmd) {
	m.quitPending = false
	return m, nil
}

// handleStatusClear clears the temporary status message.
func (m *Model) handleStatusClear() (tea.Model, tea.Cmd) {
	m.statusMsg = ""
	return m, nil
}

// handleIdeConnected handles IDE connection notifications.
func (m *Model) handleIdeConnected() (tea.Model, tea.Cmd) {
	if t, ok := m.activeTabState(); ok && t.filePath != "" && len(t.lines) > 0 {
		m.notifySelectionChanged()
	}
	return m, nil
}

// handleWindowSize handles terminal resize events.
func (m *Model) handleWindowSize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	m.width = msg.Width
	m.height = msg.Height
	maxWidth := m.width * maxTreeWidthPercent / 100
	if m.treeWidth > maxWidth {
		m.treeWidth = maxWidth
	}
	lo := m.computeLayout()
	for _, tab := range m.tabs {
		tab.vp.SetWidth(lo.editorWidth)
		tab.vp.SetHeight(lo.contentHeight)
	}
	if t, ok := m.activeTabState(); ok && t.filePath != "" && len(t.lines) > 0 && m.focusPane == paneEditor {
		m.notifySelectionChanged()
	}
	m.adjustScroll()
	return m, nil
}

// adjustScroll adjusts scroll position for the focused pane.
func (m *Model) adjustScroll() {
	lo := m.computeLayout()
	if m.focusPane == paneTree {
		h := lo.contentHeight - 1 // -1 for panel header
		switch m.activePanel {
		case panelGitDiff:
			visualIdx := m.gitCursorVisualIdx()
			m.gitScrollOffset = clampScroll(m.gitScrollOffset, visualIdx, len(m.gitVisualRows), h)
		default:
			m.treeScrollOffset = clampScroll(m.treeScrollOffset, m.treeCursor, len(m.fileTree), h)
		}
	} else if t, ok := m.activeTabState(); ok {
		if t.diffViewData != nil {
			return // viewport manages its own scroll limits
		}
		if len(t.lines) > 0 {
			t.adjustScrollForCursor(lo.contentHeight, lo.textWidth)
		}
	}
}

// statusTickCmd returns a command that clears the status message after a delay.
func statusTickCmd() tea.Cmd {
	return tea.Tick(statusClearTimeout, func(time.Time) tea.Msg {
		return statusClearMsg{}
	})
}
