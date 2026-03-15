package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/708u/gracilius/internal/git"
	"github.com/charmbracelet/x/ansi"
)

// toEntries converts git.ChangedFile slice to changedFileEntry slice.
func toEntries(dir string, files []git.ChangedFile, cat fileCategory) []changedFileEntry {
	entries := make([]changedFileEntry, len(files))
	for i, f := range files {
		entries[i] = changedFileEntry{
			name:       f.Path,
			status:     f.Status,
			absPath:    filepath.Join(dir, f.Path),
			oldContent: f.OldContent,
			newContent: f.NewContent,
			binary:     f.Binary,
			category:   cat,
			baseName:   filepath.Base(f.Path),
			dirName:    filepath.Dir(f.Path),
		}
	}
	return entries
}

// loadGitChanges returns a tea.Cmd that fetches git changed files.
func (m *Model) loadGitChanges() tea.Cmd {
	dir := m.rootDir
	return func() tea.Msg {
		reader, err := git.NewStatusReader(dir)
		if err != nil {
			return gitChangedFilesMsg{err: err}
		}

		var entries []changedFileEntry

		staged, err := reader.StagedFiles()
		if err != nil {
			return gitChangedFilesMsg{err: err}
		}
		entries = append(entries, toEntries(dir, staged, categoryStaged)...)

		unstaged, err := reader.ChangedFiles()
		if err != nil {
			return gitChangedFilesMsg{err: err}
		}
		entries = append(entries, toEntries(dir, unstaged, categoryUnstaged)...)

		untracked, err := reader.UntrackedFiles()
		if err != nil {
			return gitChangedFilesMsg{err: err}
		}
		entries = append(entries, toEntries(dir, untracked, categoryUntracked)...)

		return gitChangedFilesMsg{entries: entries}
	}
}

// handleGitChangedFiles processes the result of loading git changed files.
func (m *Model) handleGitChangedFiles(msg gitChangedFilesMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.statusMsg = fmt.Sprintf("git diff: %v", msg.err)
		m.gitLoaded = true
		return m, statusTickCmd()
	}
	m.gitChangedFiles = msg.entries
	m.gitVisualRows, m.gitEntryToVisualIdx = buildGitVisualRows(msg.entries)
	m.gitLoaded = true
	if m.gitCursor >= len(m.gitChangedFiles) {
		m.gitCursor = max(0, len(m.gitChangedFiles)-1)
	}
	return m, nil
}

// dirGroup holds entry indices for a single directory within a category.
type dirGroup struct {
	dir     string
	indices []int
}

// buildGitVisualRows builds a flat list of visual rows
// with category headers and directory sub-headers inserted.
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

	var rows []gitVisualRow
	reverseMap := make(map[int]int)

	// Reusable directory grouping state.
	var dirs []dirGroup
	dirIdx := make(map[string]int)

	for _, sec := range sections {
		if len(sec.indices) == 0 {
			continue
		}

		// Category header.
		rows = append(rows, gitVisualRow{
			isHeader: true,
			label:    fmt.Sprintf("  %s (%d)", sec.label, len(sec.indices)),
		})

		// Group entries by directory (preserving order of appearance).
		dirs = dirs[:0]
		clear(dirIdx)
		for _, idx := range sec.indices {
			d := entries[idx].dirName
			if pos, ok := dirIdx[d]; ok {
				dirs[pos].indices = append(dirs[pos].indices, idx)
			} else {
				dirIdx[d] = len(dirs)
				dirs = append(dirs, dirGroup{dir: d, indices: []int{idx}})
			}
		}

		// Emit directory sub-headers and file rows.
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

// openGitDiffEntry opens a diff tab for the selected git changed file.
func (m *Model) openGitDiffEntry() {
	if m.gitCursor < 0 || m.gitCursor >= len(m.gitChangedFiles) {
		return
	}
	entry := m.gitChangedFiles[m.gitCursor]

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
		kind:         diffTab,
		filePath:     entry.absPath,
		lines:        newContent,
		commentInput: newTextarea(),
		vp:           newViewport(),
	}
	dt.vp.SetWidth(lo.editorWidth)
	dt.vp.SetHeight(lo.contentHeight)

	if len(oldContent) > 0 {
		oldSource := strings.Join(oldContent, "\n")
		dt.diffOldHighlights = highlightFile(entry.absPath, oldSource, m.theme)
		dt.diffOldSource = oldSource
	}
	if len(newContent) > 0 {
		dt.diffNewHighlights = highlightFile(entry.absPath, strings.Join(newContent, "\n"), m.theme)
	}

	dt.diffViewData = buildDiffData(oldContent, newContent)
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

// styleDirHeader is the style for directory sub-header lines.
var styleDirHeader = lipgloss.NewStyle().Faint(true)

// renderGitPanel renders the git changed files list.
func (m *Model) renderGitPanel(width, height int) []string {
	lines := make([]string, 0, height)

	if !m.gitLoaded {
		lines = append(lines, padRight("  Loading...", width))
		for len(lines) < height {
			lines = append(lines, padRight("", width))
		}
		return lines
	}

	if len(m.gitChangedFiles) == 0 {
		lines = append(lines, padRight("  No changed files", width))
		for len(lines) < height {
			lines = append(lines, padRight("", width))
		}
		return lines
	}

	for i := m.gitScrollOffset; i < len(m.gitVisualRows) && len(lines) < height; i++ {
		row := m.gitVisualRows[i]

		if row.isHeader {
			headerLine := styleCategoryHeader.Render(row.label)
			headerLine = padRight(headerLine, width)
			lines = append(lines, headerLine)
			continue
		}

		if row.isDirHeader {
			dirLine := styleDirHeader.Render(row.label)
			dirLine = padRight(dirLine, width)
			lines = append(lines, dirLine)
			continue
		}

		e := m.gitChangedFiles[row.entryIdx]
		isCursor := row.entryIdx == m.gitCursor && m.focusPane == paneTree

		style := gitStatusStyles[e.status]
		statusIcon := style.Render(e.status.String())
		line := "      " + statusIcon + " " + e.baseName
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

// gitCursorUp moves the git cursor up one entry.
func (m *Model) gitCursorUp() {
	if m.gitCursor <= 0 {
		return
	}
	m.gitCursor--
}

// gitCursorDown moves the git cursor down one entry.
func (m *Model) gitCursorDown() {
	if m.gitCursor >= len(m.gitChangedFiles)-1 {
		return
	}
	m.gitCursor++
}

// gitCursorVisualIdx returns the visual row index for the current gitCursor.
func (m *Model) gitCursorVisualIdx() int {
	if idx, ok := m.gitEntryToVisualIdx[m.gitCursor]; ok {
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
