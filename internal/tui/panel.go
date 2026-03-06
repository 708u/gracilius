package tui

import (
	"charm.land/lipgloss/v2"
)

// changedFileEntry represents a file with changes (placeholder for future use).
type changedFileEntry struct {
	name   string
	status string // A, M, D, R
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
			lines = append(lines, padRight("  "+e.status+" "+e.name, width))
		}
	}

	for len(lines) < height {
		lines = append(lines, padRight("", width))
	}

	return lines
}
