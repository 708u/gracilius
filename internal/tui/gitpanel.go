package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/708u/gracilius/internal/diff"
	"github.com/708u/gracilius/internal/git"
	"github.com/708u/gracilius/internal/tui/render"
	"github.com/charmbracelet/x/ansi"
)

// toEntries converts git.ChangedFile slice to changedFileEntry slice.
func toEntries(dir string, files []git.ChangedFile, cat fileCategory) []changedFileEntry {
	entries := make([]changedFileEntry, len(files))
	for i, f := range files {
		entries[i] = changedFileEntry{
			name:       f.Path,
			baseName:   filepath.Base(f.Path),
			dirName:    filepath.Dir(f.Path),
			status:     f.Status,
			absPath:    filepath.Join(dir, f.Path),
			oldContent: f.OldContent,
			newContent: f.NewContent,
			binary:     f.Binary,
			category:   cat,
		}
	}
	return entries
}

// loadGitChangesForMode returns a tea.Cmd that fetches git changed files
// for the given diff mode.
func (m *Model) loadGitChangesForMode(mode gitDiffMode) tea.Cmd {
	dir := m.rootDir
	switch mode {
	case gitModeBranch:
		baseRef := m.gitMergeBase
		return func() tea.Msg {
			files, err := git.BranchDiff(dir, baseRef)
			if err != nil {
				return gitChangedFilesMsg{mode: mode, err: err}
			}
			entries := toEntries(dir, files, categoryUnstaged)
			return gitChangedFilesMsg{mode: mode, entries: entries}
		}
	default: // gitModeWorking
		return func() tea.Msg {
			reader, err := git.NewStatusReader(dir)
			if err != nil {
				return gitChangedFilesMsg{mode: mode, err: err}
			}
			return loadWorkingChanges(reader, dir, mode)
		}
	}
}

// loadWorkingChanges fetches staged, unstaged, and untracked files
// in parallel using the given StatusReader.
func loadWorkingChanges(reader *git.StatusReader, dir string, mode gitDiffMode) gitChangedFilesMsg {
	type catResult struct {
		files []git.ChangedFile
		cat   fileCategory
		err   error
	}

	results := make([]catResult, 3)
	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		f, e := reader.StagedFiles()
		results[0] = catResult{f, categoryStaged, e}
	}()
	go func() {
		defer wg.Done()
		f, e := reader.ChangedFiles()
		results[1] = catResult{f, categoryUnstaged, e}
	}()
	go func() {
		defer wg.Done()
		f, e := reader.UntrackedFiles()
		results[2] = catResult{f, categoryUntracked, e}
	}()
	wg.Wait()

	var entries []changedFileEntry
	for _, r := range results {
		if r.err != nil {
			return gitChangedFilesMsg{mode: mode, err: r.err}
		}
		entries = append(entries, toEntries(dir, r.files, r.cat)...)
	}
	return gitChangedFilesMsg{mode: mode, entries: entries}
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
		base, err := git.MergeBase(dir, "origin/"+branch)
		if err != nil {
			return gitBranchInfoMsg{
				defaultBranch: branch,
				err:           fmt.Sprintf("No merge-base with origin/%s", branch),
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
		m.initialDiffAutoOpened = true
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
		if !m.initialDiffAutoOpened && msg.mode == gitModeBranch {
			m.initialDiffAutoOpened = true
		}
		return m, statusTickCmd()
	}
	gs.entries = msg.entries
	gs.visualRows, gs.entryToVisualIdx = buildGitVisualRows(msg.entries)
	gs.loaded = true
	gs.stale = false
	m.gitAnyLoaded = true
	if gs.cursor >= len(gs.entries) {
		gs.cursor = max(0, len(gs.entries)-1)
	}

	// Auto-open the first file on initial branch mode load.
	if !m.initialDiffAutoOpened && msg.mode == gitModeBranch {
		m.autoOpenFirstDiff()
	}

	return m, nil
}

// buildGitVisualRows builds a flat list of visual rows
// with category headers and directory grouping.
// Hierarchy: Category > Directory > File.
// Also returns a reverse map from entryIdx to visual row index.
func buildGitVisualRows(entries []changedFileEntry) ([]gitVisualRow, map[int]int) {
	type section struct {
		cat     fileCategory
		label   string
		indices []int
	}
	sections := []section{
		{cat: categoryStaged, label: "Staged Changes"},
		{cat: categoryUnstaged, label: "Changes"},
		{cat: categoryUntracked, label: "Untracked Files"},
	}

	// Single pass: collect entry indices per category.
	for i := range entries {
		for j := range sections {
			if sections[j].cat == entries[i].category {
				sections[j].indices = append(sections[j].indices, i)
				break
			}
		}
	}

	type dirGroup struct {
		dir     string
		indices []int
	}

	var rows []gitVisualRow
	reverseMap := make(map[int]int)
	var dirs []dirGroup
	dirIdx := map[string]int{} // dir -> index in dirs slice

	for _, sec := range sections {
		if len(sec.indices) == 0 {
			continue
		}

		// Category header.
		rows = append(rows, gitVisualRow{
			isHeader: true,
			label:    fmt.Sprintf("  %s (%d)", sec.label, len(sec.indices)),
		})

		// Group entries by directory, preserving order of first appearance.
		dirs = dirs[:0]
		clear(dirIdx)
		for _, idx := range sec.indices {
			d := entries[idx].dirName
			if i, ok := dirIdx[d]; ok {
				dirs[i].indices = append(dirs[i].indices, idx)
			} else {
				dirIdx[d] = len(dirs)
				dirs = append(dirs, dirGroup{dir: d, indices: []int{idx}})
			}
		}

		for _, dg := range dirs {
			rows = append(rows, gitVisualRow{
				isDirHeader: true,
				label:       "    " + dg.dir + "/",
			})
			for _, idx := range dg.indices {
				reverseMap[idx] = len(rows)
				rows = append(rows, gitVisualRow{entryIdx: idx})
			}
		}
	}
	return rows, reverseMap
}

// autoOpenFirstDiff opens the first non-binary changed file as a diff tab
// on initial startup. Called once when the first branch-mode result arrives.
func (m *Model) autoOpenFirstDiff() {
	m.initialDiffAutoOpened = true

	gs := &m.gitModeState[gitModeBranch]
	if len(gs.entries) == 0 {
		return
	}

	idx := -1
	for i := range gs.entries {
		if !gs.entries[i].binary {
			idx = i
			break
		}
	}
	if idx < 0 {
		return
	}

	gs.cursor = idx
	m.openGitDiffEntry()
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
		gitDiffLabel:      m.gitDiffMode.tabPrefix(m.gitDefaultBranch),
	}
	dt.vp.SetWidth(lo.editorWidth)
	dt.vp.SetHeight(lo.contentHeight)

	if len(oldContent) > 0 {
		oldSource := strings.Join(oldContent, "\n")
		dt.diffOldHighlights = render.HighlightFile(entry.absPath, oldSource, m.theme)
		dt.diffOldSource = oldSource
	}
	if len(newContent) > 0 {
		dt.diffNewHighlights = render.HighlightFile(entry.absPath, strings.Join(newContent, "\n"), m.theme)
	}

	dt.diffViewData = diff.Build(oldContent, newContent)
	dt.initDiffContent(m.theme, lo.editorWidth, lo.contentHeight)

	m.tabs = append(m.tabs, dt)
	m.activeTab = len(m.tabs) - 1
	m.focusPane = paneEditor
}

var gitStatusStyles = map[git.FileStatus]lipgloss.Style{
	git.StatusAdded:     lipgloss.NewStyle().Foreground(lipgloss.Color("2")), // green
	git.StatusDeleted:   lipgloss.NewStyle().Foreground(lipgloss.Color("1")), // red
	git.StatusModified:  lipgloss.NewStyle().Foreground(lipgloss.Color("3")), // yellow
	git.StatusRenamed:   lipgloss.NewStyle().Foreground(lipgloss.Color("6")), // cyan
	git.StatusUntracked: lipgloss.NewStyle().Foreground(lipgloss.Color("2")), // green
}

// styleCategoryHeader is the style for category header lines.
var styleCategoryHeader = lipgloss.NewStyle().Bold(true)

// styleDirHeader is the style for directory header lines.
var styleDirHeader = lipgloss.NewStyle().Faint(true)

// renderGitPanel renders the git changed files list.
func (m *Model) renderGitPanel(width, height int) []string {
	gs := m.gitState()
	lines := make([]string, 0, height)

	if !gs.loaded {
		lines = append(lines, render.PadRight("  Loading...", width))
		for len(lines) < height {
			lines = append(lines, render.PadRight("", width))
		}
		return lines
	}

	if len(gs.entries) == 0 {
		lines = append(lines, render.PadRight("  No changed files", width))
		for len(lines) < height {
			lines = append(lines, render.PadRight("", width))
		}
		return lines
	}

	for i := gs.scrollOffset; i < len(gs.visualRows) && len(lines) < height; i++ {
		row := gs.visualRows[i]

		if row.isHeader {
			headerLine := styleCategoryHeader.Render(row.label)
			headerLine = render.PadRight(headerLine, width)
			lines = append(lines, headerLine)
			continue
		}

		if row.isDirHeader {
			dirLine := styleDirHeader.Render(row.label)
			dirLine = render.PadRight(dirLine, width)
			lines = append(lines, dirLine)
			continue
		}

		e := gs.entries[row.entryIdx]
		isCursor := row.entryIdx == gs.cursor && m.focusPane == paneTree

		style := gitStatusStyles[e.status]
		statusIcon := style.Render(e.status.String())
		line := "      " + statusIcon + " " + e.baseName
		displayLine := ansi.Truncate(line, width, "...")
		displayLine = render.PadRight(displayLine, width)

		if isCursor {
			displayLine = styleTreeCursor(m.theme).Render(displayLine)
		}

		lines = append(lines, displayLine)
	}

	for len(lines) < height {
		lines = append(lines, render.PadRight("", width))
	}

	return lines
}

// gitCursorUp moves the git cursor up one entry.
func (m *Model) gitCursorUp() {
	gs := m.gitState()
	if gs.cursor <= 0 {
		return
	}
	gs.cursor--
}

// gitCursorDown moves the git cursor down one entry.
func (m *Model) gitCursorDown() {
	gs := m.gitState()
	if gs.cursor >= len(gs.entries)-1 {
		return
	}
	gs.cursor++
}

// gitCursorVisualIdx returns the visual row index for the current gitCursor.
func (m *Model) gitCursorVisualIdx() int {
	gs := m.gitState()
	if idx, ok := gs.entryToVisualIdx[gs.cursor]; ok {
		return idx
	}
	return 0
}

// firstGitEntryIdx returns the entryIdx of the first file row.
func firstGitEntryIdx(rows []gitVisualRow) int {
	for _, row := range rows {
		if row.isFileRow() {
			return row.entryIdx
		}
	}
	return 0
}

// lastGitEntryIdx returns the entryIdx of the last file row.
func lastGitEntryIdx(rows []gitVisualRow) int {
	for i := len(rows) - 1; i >= 0; i-- {
		if rows[i].isFileRow() {
			return rows[i].entryIdx
		}
	}
	return 0
}
