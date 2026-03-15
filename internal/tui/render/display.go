package render

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/mattn/go-runewidth"
)

// PadRight pads a string with spaces to the given display width.
func PadRight(s string, width int) string {
	return lipgloss.NewStyle().Width(width).Render(s)
}

// ExpandTabs replaces tabs with 4 spaces.
func ExpandTabs(s string) string {
	return strings.ReplaceAll(s, "\t", "    ")
}

// RuneWidth returns the display width of a rune, treating tabs as 4 columns.
func RuneWidth(r rune) int {
	if r == '\t' {
		return 4
	}
	return runewidth.RuneWidth(r)
}

// WrapBreakpoints returns the rune indices at which line should be wrapped
// to fit within textWidth display columns. Returns nil if the line fits
// without wrapping or if textWidth <= 0.
func WrapBreakpoints(line string, textWidth int) []int {
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
		w := RuneWidth(r)
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

// DisplayWidthRange returns the display width of runes in line[from:to).
func DisplayWidthRange(line string, from, to int) int {
	runes := []rune(line)
	w := 0
	for i := from; i < to && i < len(runes); i++ {
		w += RuneWidth(runes[i])
	}
	return w
}

// CountWraps returns the number of visual rows a line occupies
// when wrapped at textWidth. Returns 1 if no wrapping is needed.
// Unlike WrapBreakpoints, this does not allocate a slice.
func CountWraps(line string, textWidth int) int {
	if textWidth <= 0 {
		return 1
	}

	count := 1
	col := 0
	charsInSeg := 0
	for _, r := range line {
		w := RuneWidth(r)
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
