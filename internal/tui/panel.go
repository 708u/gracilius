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
	baseName   string // filepath.Base(name), precomputed
	dirName    string // filepath.Dir(name), precomputed
	status     git.FileStatus
	absPath    string
	oldContent []string
	newContent []string
	binary     bool
	category   fileCategory
}

// gitVisualRow represents a visual row in the git panel.
type gitVisualRow struct {
	isHeader    bool   // category header (e.g., "Staged Changes (2)")
	isDirHeader bool   // directory header (e.g., "internal/tui/")
	label       string // header text (header/dir header rows only)
	entryIdx    int    // index into gitChangedFiles (file rows only)
}

// isFileRow returns true if this row represents an actual file entry.
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
		for _, e := range entries {
			if len(lines) >= height {
				break
			}
			lines = append(lines, padRight("  "+e.status.String()+" "+e.name, width))
		}
	}

	for len(lines) < height {
		lines = append(lines, padRight("", width))
	}

	return lines
}
