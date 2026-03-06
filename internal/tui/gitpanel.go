package tui

import (
	"fmt"
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/708u/gracilius/internal/git"
	"github.com/charmbracelet/x/ansi"
)

// loadGitChanges returns a tea.Cmd that fetches git changed files.
func (m *Model) loadGitChanges() tea.Cmd {
	dir := m.rootDir
	return func() tea.Msg {
		files, err := git.ChangedFiles(dir)
		if err != nil {
			return gitChangedFilesMsg{err: err}
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
	m.gitLoaded = true
	if m.gitCursor >= len(m.gitChangedFiles) {
		m.gitCursor = max(0, len(m.gitChangedFiles)-1)
	}
	return m, nil
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
}

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

	for i := m.gitScrollOffset; i < len(m.gitChangedFiles) && len(lines) < height; i++ {
		e := m.gitChangedFiles[i]
		isCursor := i == m.gitCursor && m.focusPane == paneTree

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
