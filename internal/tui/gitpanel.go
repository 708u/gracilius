package tui

import (
	"fmt"
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/708u/gracilius/internal/git"
	"github.com/charmbracelet/x/ansi"
)

// gitBranchInfoMsg carries the result of async branch info resolution.
type gitBranchInfoMsg struct {
	mergeBase     string
	defaultBranch string
	err           string
}

// loadGitChangesForMode returns a tea.Cmd that fetches git changed files
// for the given diff mode.
func (m *Model) loadGitChangesForMode(mode gitDiffMode) tea.Cmd {
	dir := m.rootDir
	opts := m.diffOptionsForMode(mode)
	return func() tea.Msg {
		files, err := git.ChangedFilesWithOptions(dir, opts)
		if err != nil {
			return gitChangedFilesMsg{mode: mode, err: err}
		}
		entries := make([]changedFileEntry, len(files))
		for i, f := range files {
			entries[i] = changedFileEntry{
				name:       f.Path,
				status:     f.Status,
				absPath:    filepath.Join(dir, f.Path),
				oldContent: f.OldContent,
				newContent: f.NewContent,
				binary:     f.Binary,
			}
		}
		return gitChangedFilesMsg{mode: mode, entries: entries}
	}
}

// diffOptionsForMode builds DiffOptions for the given mode.
func (m *Model) diffOptionsForMode(mode gitDiffMode) git.DiffOptions {
	opts := git.DiffOptions{Mode: git.DiffMode(mode)}
	if mode == gitModeBranch {
		opts.BaseRef = m.gitMergeBase
	}
	return opts
}

// initGitBranchInfoAsync returns a tea.Cmd that resolves the default branch
// and merge-base in the background.
func (m *Model) initGitBranchInfoAsync() tea.Cmd {
	dir := m.rootDir
	cached := m.gitDefaultBranch
	return func() tea.Msg {
		branch := cached
		if branch == "" {
			var err error
			branch, err = git.DefaultBranch(dir)
			if err != nil {
				return gitBranchInfoMsg{err: "No base branch found"}
			}
		}
		base, err := git.MergeBase(dir, branch)
		if err != nil {
			return gitBranchInfoMsg{
				defaultBranch: branch,
				err:           fmt.Sprintf("No merge-base with %s", branch),
			}
		}
		return gitBranchInfoMsg{mergeBase: base, defaultBranch: branch}
	}
}

// handleGitBranchInfo processes the async branch info result.
func (m *Model) handleGitBranchInfo(msg gitBranchInfoMsg) (tea.Model, tea.Cmd) {
	if msg.defaultBranch != "" {
		m.gitDefaultBranch = msg.defaultBranch
	}
	if msg.err != "" {
		m.statusMsg = msg.err
		return m, statusTickCmd()
	}
	m.gitMergeBase = msg.mergeBase
	cmd := m.loadGitChangesForMode(gitModeBranch)
	return m, cmd
}

// handleGitChangedFiles processes the result of loading git changed files.
func (m *Model) handleGitChangedFiles(msg gitChangedFilesMsg) (tea.Model, tea.Cmd) {
	gs := &m.gitModeState[msg.mode]
	if msg.err != nil {
		m.statusMsg = fmt.Sprintf("git diff: %v", msg.err)
		gs.loaded = true
		m.gitAnyLoaded = true
		return m, statusTickCmd()
	}
	gs.entries = msg.entries
	gs.loaded = true
	gs.stale = false
	m.gitAnyLoaded = true
	if gs.cursor >= len(gs.entries) {
		gs.cursor = max(0, len(gs.entries)-1)
	}
	return m, nil
}

// openGitDiffEntry opens a diff tab for the selected git changed file.
func (m *Model) openGitDiffEntry() {
	gs := m.gitState()
	if gs.cursor < 0 || gs.cursor >= len(gs.entries) {
		return
	}
	entry := gs.entries[gs.cursor]

	if entry.binary {
		m.statusMsg = fmt.Sprintf("Binary file: %s", entry.name)
		return
	}

	if i := m.findTabByPath(entry.absPath); i >= 0 {
		m.activeTab = i
		m.focusPane = paneEditor
		return
	}

	oldContent := entry.oldContent
	newContent := entry.newContent
	if oldContent == nil {
		oldContent = []string{}
	}
	if newContent == nil {
		newContent = []string{}
	}

	lo := m.computeLayout()
	dt := &tab{
		kind:              diffTab,
		filePath:          entry.absPath,
		lines:             newContent,
		commentInput:      newTextarea(),
		vp:                newViewport(),
		gitDiffModeTag:    m.gitDiffMode,
		hasGitDiffModeTag: true,
	}
	dt.vp.SetWidth(lo.editorWidth)
	dt.vp.SetHeight(lo.contentHeight)
	dt.diffViewData = buildDiffData(oldContent, newContent)
	dt.initDiffContent(m.theme, lo.editorWidth)

	m.tabs = append(m.tabs, dt)
	m.activeTab = len(m.tabs) - 1
	m.focusPane = paneEditor
}

var gitStatusStyles = map[string]lipgloss.Style{
	"A": lipgloss.NewStyle().Foreground(lipgloss.Color("2")), // green
	"D": lipgloss.NewStyle().Foreground(lipgloss.Color("1")), // red
	"M": lipgloss.NewStyle().Foreground(lipgloss.Color("3")), // yellow
	"R": lipgloss.NewStyle().Foreground(lipgloss.Color("6")), // cyan
	"?": lipgloss.NewStyle().Foreground(lipgloss.Color("5")), // magenta
}

// renderGitPanel renders the git changed files list.
func (m *Model) renderGitPanel(width, height int) []string {
	gs := m.gitState()
	lines := make([]string, 0, height)

	if !gs.loaded {
		lines = append(lines, padRight("  Loading...", width))
		for len(lines) < height {
			lines = append(lines, padRight("", width))
		}
		return lines
	}

	if len(gs.entries) == 0 {
		lines = append(lines, padRight("  No changed files", width))
		for len(lines) < height {
			lines = append(lines, padRight("", width))
		}
		return lines
	}

	for i := gs.scrollOffset; i < len(gs.entries) && len(lines) < height; i++ {
		e := gs.entries[i]
		isCursor := i == gs.cursor && m.focusPane == paneTree

		style := gitStatusStyles[e.status]
		statusIcon := style.Render(e.status)
		line := "  " + statusIcon + " " + e.name
		displayLine := ansi.Truncate(line, width, "...")
		displayLine = padRight(displayLine, width)

		if isCursor {
			displayLine = styleTreeCursor(m.theme).Render(displayLine)
		}

		lines = append(lines, displayLine)
	}

	for len(lines) < height {
		lines = append(lines, padRight("", width))
	}

	return lines
}
