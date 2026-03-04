package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// padRight pads a string with spaces to the given display width.
func padRight(s string, width int) string {
	return lipgloss.NewStyle().Width(width).Render(s)
}

// expandTabs replaces tabs with 4 spaces.
func expandTabs(s string) string {
	return strings.ReplaceAll(s, "\t", "    ")
}
