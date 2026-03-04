package tui

import (
	"charm.land/lipgloss/v2"
)

const scrollbarWidth = 1

// scrollbarBlock returns the styled thumb character for the given color.
func scrollbarBlock(fgColor string) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(fgColor)).Render("\u2588")
}

// renderScrollbar generates a column of scrollbar characters.
// Each element is a single-character string: either a styled thumb block
// or a space (track). When totalItems <= height, all entries are spaces.
func renderScrollbar(height, totalItems, offset int, block string) []string {
	col := make([]string, height)
	for i := range col {
		col[i] = " "
	}

	if totalItems <= height || height <= 0 {
		return col
	}

	thumbSize := max(1, height*height/totalItems)
	maxOffset := totalItems - height
	thumbPos := 0
	if maxOffset > 0 {
		thumbPos = offset * (height - thumbSize) / maxOffset
	}

	for i := thumbPos; i < thumbPos+thumbSize && i < height; i++ {
		col[i] = block
	}

	return col
}

// appendScrollbar appends scrollbar column entries to each line.
func appendScrollbar(lines []string, scrollbar []string) {
	for i := range lines {
		lines[i] += scrollbar[i]
	}
}
