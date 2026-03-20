package tui

import (
	"log"
	"os"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/708u/gracilius/internal/comment"
	"github.com/708u/gracilius/internal/diff"
	"github.com/708u/gracilius/internal/fileutil"
	"github.com/708u/gracilius/internal/tui/render"
	"github.com/google/uuid"
)

// handleFileChanged processes file change notifications.
func (m *Model) handleFileChanged(msg fileChangedMsg) (tea.Model, tea.Cmd) {
	// Update the active file tab if its path matches.
	if t, ok := m.activeTabState(); ok &&
		t.kind == fileTab && t.filePath == msg.path {
		t.lines = msg.lines
		t.syncContent(msg.lines)
		t.highlightedLines = render.HighlightFile(
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
		t.diffOldHighlights = render.HighlightFile(t.filePath, oldSource, m.theme)
		t.diffViewData = diff.Build(msg.lines, t.lines)
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
		if t.filePath == "" {
			continue
		}
		switch t.kind {
		case fileTab:
			stored, err := m.commentRepo.List(t.filePath, false)
			if err != nil {
				log.Printf("Failed to reload comments for %s: %v", t.filePath, err)
				continue
			}
			t.comments = stored
		case diffTab:
			stored, err := m.commentRepo.ListByScope(t.diffScope, t.filePath, false)
			if err != nil {
				log.Printf("Failed to reload diff comments for %s: %v", t.filePath, err)
				continue
			}
			t.comments = stored
			t.diffCacheWidth = 0 // force re-render
		}
	}
	cmd := m.watchComments()
	return m, cmd
}

// handleTreeChanged processes directory tree change notifications.
func (m *Model) handleTreeChanged() (tea.Model, tea.Cmd) {
	expanded := expandedPaths(m.fileTree)
	m.fileTree = buildFileTree(m.rootDir, m.excludeFunc)
	m.fileTree = restoreExpanded(m.fileTree, expanded, m.excludeFunc)
	if m.treeCursor >= len(m.fileTree) {
		m.treeCursor = max(0, len(m.fileTree)-1)
	}
	cmds := []tea.Cmd{m.watchDir()}
	if m.gitAnyLoaded {
		cmds = append(cmds, m.scheduleGitSync())
	}
	return m, tea.Batch(cmds...)
}

// handleGitDirChanged reloads git changes when .git/index or .git/HEAD changes.
func (m *Model) handleGitDirChanged(msg gitDirChangedMsg) (tea.Model, tea.Cmd) {
	cmds := []tea.Cmd{m.watchGitDir()}
	if msg.headChanged {
		m.gitMergeBase = ""
	}
	if m.gitAnyLoaded {
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
// Reloads the active mode; marks other loaded modes as stale.
func (m *Model) handleGitSync(msg gitSyncMsg) (tea.Model, tea.Cmd) {
	if msg.gen != m.gitSyncGen {
		return m, nil
	}
	// Mark all loaded modes as stale.
	for i := range m.gitModeState {
		if m.gitModeState[i].loaded {
			m.gitModeState[i].stale = true
		}
	}
	// Reload only the active mode.
	active := m.gitDiffMode
	gs := m.gitState()
	gs.stale = false
	var cmd tea.Cmd
	if active == gitModeBranch && m.gitMergeBase == "" {
		cmd = m.initGitBranchInfoAsync()
	} else {
		cmd = m.loadGitChangesForMode(active)
	}
	return m, cmd
}

// handleOpenDiff opens a new diff tab.
func (m *Model) handleOpenDiff(msg OpenDiffMsg) (tea.Model, tea.Cmd) {
	newLines := fileutil.SplitLines([]byte(msg.Contents))
	dt := newDiffTab(msg.FilePath, newLines, msg.Accept, msg.Reject)

	// Set review context with a unique session ID.
	sessionID, err := uuid.NewV7()
	if err != nil {
		log.Printf("Failed to generate session UUID: %v", err)
	}
	dt.diffScope = comment.DiffScope{Kind: "review", SessionID: sessionID.String()}
	dt.syncContent(newLines)
	dt.highlightedLines = render.HighlightFile(msg.FilePath, msg.Contents, m.theme)

	var oldLines []string
	if oldContent, err := os.ReadFile(msg.FilePath); err == nil {
		oldLines = fileutil.SplitLines(oldContent)
	}

	if len(oldLines) > 0 {
		oldSource := strings.Join(oldLines, "\n")
		dt.diffOldHighlights = render.HighlightFile(msg.FilePath, oldSource, m.theme)
		dt.diffOldSource = oldSource
	}
	dt.diffNewHighlights = render.HighlightFile(msg.FilePath, msg.Contents, m.theme)

	dt.diffViewData = diff.Build(oldLines, newLines)
	lo := m.computeLayout()
	dt.vp.SetWidth(lo.editorWidth)
	dt.vp.SetHeight(lo.paneBodyHeight)
	dt.initDiffContent(m.theme, lo.editorWidth, lo.paneBodyHeight)

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
		tab.vp.SetHeight(lo.paneBodyHeight)
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
		h := m.leftPaneBodyHeight(lo)
		switch m.activePanel {
		case panelGitDiff:
			gs := m.gitState()
			visualIdx := m.gitCursorVisualIdx()
			gs.scrollOffset = clampScroll(gs.scrollOffset, visualIdx, len(gs.visualRows), h)
		default:
			m.treeScrollOffset = clampScroll(m.treeScrollOffset, m.treeCursor, len(m.fileTree), h)
		}
	} else if t, ok := m.activeTabState(); ok {
		if t.diffViewData != nil {
			t.adjustDiffScrollForCursor(lo.paneBodyHeight)
			return
		}
		if len(t.lines) > 0 {
			t.adjustScrollForCursor(lo.paneBodyHeight, lo.textWidth)
		}
	}
}

// statusTickCmd returns a command that clears the status message after a delay.
func statusTickCmd() tea.Cmd {
	return tea.Tick(statusClearTimeout, func(time.Time) tea.Msg {
		return statusClearMsg{}
	})
}
