package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

// padRight pads a string with spaces to the given display width.
func padRight(s string, width int) string {
	return lipgloss.NewStyle().Width(width).Render(s)
}

// expandTabs replaces tabs with 4 spaces.
func expandTabs(s string) string {
	return strings.ReplaceAll(s, "\t", "    ")
}

// runeWidth returns the display width of a rune, treating tabs as 4 columns.
func runeWidth(r rune) int {
	if r == '\t' {
		return 4
	}
	return runewidth.RuneWidth(r)
}

// wrapBreakpoints returns the rune indices at which line should be wrapped
// to fit within textWidth display columns. Returns nil if the line fits
// without wrapping or if textWidth <= 0.
func wrapBreakpoints(line string, textWidth int) []int {
	if textWidth <= 0 {
		return nil
	}
	runes := []rune(line)
	if len(runes) == 0 {
		return nil
	}

	var breaks []int
	col := 0
	segStart := 0
	for i, r := range runes {
		w := runeWidth(r)
		if col+w > textWidth && i > segStart {
			breaks = append(breaks, i)
			col = w
			segStart = i
		} else {
			col += w
		}
	}
	return breaks
}

// countWraps returns the number of visual rows a line occupies
// when wrapped at textWidth. Returns 1 if no wrapping is needed.
// Unlike wrapBreakpoints, this does not allocate a slice.
func countWraps(line string, textWidth int) int {
	if textWidth <= 0 {
		return 1
	}

	count := 1
	col := 0
	charsInSeg := 0
	for _, r := range line {
		w := runeWidth(r)
		if col+w > textWidth && charsInSeg > 0 {
			count++
			col = w
			charsInSeg = 1
		} else {
			col += w
			charsInSeg++
		}
	}
	return count
}
