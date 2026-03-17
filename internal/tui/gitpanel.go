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
		if !f.Binary && (len(f.OldContent) > 0 || len(f.NewContent) > 0) {
			d := diff.Build(f.OldContent, f.NewContent)
			entries[i].stats = d.Summary
			entries[i].diffData = d
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
	gs.recomputeViewedCount()
	gs.visualRows, gs.entryToVisualIdx = buildGitVisualRows(msg.entries, gs.viewed)
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
func buildGitVisualRows(entries []changedFileEntry, viewed map[string]bool) ([]gitVisualRow, map[int]int) {
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

		// Category header with viewed count.
		viewedInCat := 0
		for _, idx := range sec.indices {
			if viewed[entries[idx].name] {
				viewedInCat++
			}
		}
		headerLabel := fmt.Sprintf("%s (%d/%d)", sec.label, viewedInCat, len(sec.indices))
		rows = append(rows, gitVisualRow{
			isHeader: true,
			label:    headerLabel,
			catStats: categoryStats(entries, sec.cat),
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
				label:       "  " + dg.dir + "/",
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
	dt.vp.SetHeight(lo.paneBodyHeight)

	if len(oldContent) > 0 {
		oldSource := strings.Join(oldContent, "\n")
		dt.diffOldHighlights = render.HighlightFile(entry.absPath, oldSource, m.theme)
		dt.diffOldSource = oldSource
	}
	if len(newContent) > 0 {
		dt.diffNewHighlights = render.HighlightFile(entry.absPath, strings.Join(newContent, "\n"), m.theme)
	}

	if entry.diffData != nil {
		dt.diffViewData = entry.diffData
		gs.entries[gs.cursor].diffData = nil // release reference after handoff
	} else {
		dt.diffViewData = diff.Build(oldContent, newContent)
	}
	dt.initDiffContent(m.theme, lo.editorWidth, lo.paneBodyHeight)

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

// formatDiffStats returns a colored "+N -N ~N" string.
// Parts with zero count are omitted. Returns "" if all zero.
func formatDiffStats(s diff.Stats, theme render.Theme) string {
	coloredStat := func(hex, prefix string, count int) string {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(hex)).
			Render(fmt.Sprintf("%s%d", prefix, count))
	}

	var parts []string
	if s.Additions > 0 {
		parts = append(parts, coloredStat(theme.DiffAddFg, "+", s.Additions))
	}
	if s.Deletions > 0 {
		parts = append(parts, coloredStat(theme.DiffDelFg, "-", s.Deletions))
	}
	if s.Modified > 0 {
		parts = append(parts, coloredStat(theme.DiffModFg, "~", s.Modified))
	}
	return strings.Join(parts, " ")
}

// categoryStats returns the sum of diff stats for entries matching
// the given category.
func categoryStats(entries []changedFileEntry, cat fileCategory) diff.Stats {
	var s diff.Stats
	for i := range entries {
		if entries[i].category == cat {
			s.Additions += entries[i].stats.Additions
			s.Deletions += entries[i].stats.Deletions
			s.Modified += entries[i].stats.Modified
		}
	}
	return s
}

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
			headerLabel := styleCategoryHeader.Render(row.label)
			statsStr := formatDiffStats(row.catStats, m.theme)
			headerLine := render.PadBetween(headerLabel, statsStr, width)
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
		isViewed := gs.viewed[e.name]

		style := gitStatusStyles[e.status]
		statusIcon := style.Render(e.status.String())
		line := "    " + statusIcon + " " + e.baseName

		var displayLine string
		if isCursor {
			displayLine = renderTreeCursor(line, width, m.theme)
		} else {
			displayLine = render.PadRight(ansi.Truncate(line, width, "..."), width)
			if isViewed {
				displayLine = styleDirHeader.Render(displayLine)
			}
		}

		lines = append(lines, displayLine)
	}

	for len(lines) < height {
		lines = append(lines, render.PadRight("", width))
	}

	return lines
}

// gitCursorUp moves the git cursor to the previous file in visual order.
func (m *Model) gitCursorUp() {
	gs := m.gitState()
	curVisual, ok := gs.entryToVisualIdx[gs.cursor]
	if !ok {
		return
	}
	for i := curVisual - 1; i >= 0; i-- {
		if gs.visualRows[i].isFileRow() {
			gs.cursor = gs.visualRows[i].entryIdx
			return
		}
	}
}

// gitCursorDown moves the git cursor to the next file in visual order.
func (m *Model) gitCursorDown() {
	gs := m.gitState()
	curVisual, ok := gs.entryToVisualIdx[gs.cursor]
	if !ok {
		return
	}
	for i := curVisual + 1; i < len(gs.visualRows); i++ {
		if gs.visualRows[i].isFileRow() {
			gs.cursor = gs.visualRows[i].entryIdx
			return
		}
	}
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

// findUnviewedEntry searches for the nearest unviewed entry in the given
// direction (1 for next, -1 for previous). Wraps around.
// Returns -1 if all entries are viewed.
func (gs *gitPanelState) findUnviewedEntry(from, dir int) int {
	n := len(gs.entries)
	if n == 0 {
		return -1
	}
	for i := 1; i <= n; i++ {
		idx := (from + i*dir%n + n) % n
		if !gs.viewed[gs.entries[idx].name] {
			return idx
		}
	}
	return -1
}

// navigateUnviewed jumps to the next/prev unviewed file and opens its diff tab.
func (m *Model) navigateUnviewed(dir int) (tea.Model, tea.Cmd) {
	gs := m.gitState()
	if len(gs.entries) == 0 {
		return m, nil
	}

	// Determine starting position.
	from := gs.cursor
	if t, ok := m.activeTabState(); ok && t.hasGitDiffModeTag {
		for i := range gs.entries {
			if gs.entries[i].absPath == t.filePath {
				from = i
				break
			}
		}
	}

	target := gs.findUnviewedEntry(from, dir)
	if target < 0 {
		m.statusMsg = "All files viewed"
		return m, statusTickCmd()
	}

	gs.cursor = target
	m.openGitDiffEntry()
	// Adjust git panel scroll for the new cursor.
	if idx, ok := gs.entryToVisualIdx[gs.cursor]; ok {
		lo := m.computeLayout()
		panelHeight := lo.contentHeight - 1
		if panelHeight > 0 && idx >= gs.scrollOffset+panelHeight {
			gs.scrollOffset = idx - panelHeight + 1
		} else if idx < gs.scrollOffset {
			gs.scrollOffset = idx
		}
	}
	return m, nil
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
