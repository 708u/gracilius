package tui

import (
	"charm.land/lipgloss/v2"
	"github.com/708u/gracilius/internal/git"
)

// fileCategory classifies a changed file entry.
type fileCategory int

const (
	categoryStaged fileCategory = iota
	categoryUnstaged
	categoryUntracked
)

// changedFileEntry represents a file with changes.
type changedFileEntry struct {
	name       string
	status     git.FileStatus
	absPath    string
	oldContent []string
	newContent []string
	binary     bool
	category   fileCategory
	baseName   string // filepath.Base(name)
	dirName    string // filepath.Dir(name)
}

// gitVisualRow represents a visual row in the git panel.
type gitVisualRow struct {
	isHeader    bool
	isDirHeader bool
	label       string // header text (header/dir header rows only)
	entryIdx    int    // index into gitChangedFiles (file rows only)
}

// isFileRow returns true if this row represents a file entry.
func (r gitVisualRow) isFileRow() bool {
	return !r.isHeader && !r.isDirHeader
}

// stylePanelHeader is the style for panel header labels.
var stylePanelHeader = lipgloss.NewStyle().Bold(true)

// renderPanelHeader renders a 1-line header for the left pane panel.
func renderPanelHeader(label string, width int, theme themeConfig) string {
	return padRight(stylePanelHeader.
		Foreground(lipgloss.Color(theme.tabActiveFg)).
		Render(label), width)
}

// renderChangedFiles renders the changed file list for git/PR panels.
func renderChangedFiles(entries []changedFileEntry, width, height int) []string {
	lines := make([]string, 0, height)

	if len(entries) == 0 {
		lines = append(lines, padRight("  No changed files", width))
	} else {
		for i := range entries {
			if len(lines) >= height {
				break
			}
			lines = append(lines, padRight("  "+entries[i].status.String()+" "+entries[i].name, width))
		}
	}

	for len(lines) < height {
		lines = append(lines, padRight("", width))
	}

	return lines
}
