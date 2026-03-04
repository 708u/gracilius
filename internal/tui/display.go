package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
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

// splitRunsAtBreakpoints divides styledRuns at the given rune-index
// breakpoints, returning one []styledRun per visual wrap segment.
// bp must be sorted in ascending order (as returned by wrapBreakpoints).
func splitRunsAtBreakpoints(runs []styledRun, bp []int) [][]styledRun {
	segments := make([][]styledRun, 0, len(bp)+1)
	var current []styledRun
	pos := 0
	bpIdx := 0

	for _, run := range runs {
		runes := []rune(run.Text)
		runEnd := pos + len(runes)
		consumed := 0

		for bpIdx < len(bp) && bp[bpIdx] >= pos && bp[bpIdx] < runEnd {
			splitAt := bp[bpIdx] - pos
			if splitAt > consumed {
				current = append(current, styledRun{
					Text: string(runes[consumed:splitAt]),
					ANSI: run.ANSI,
				})
			}
			segments = append(segments, current)
			current = nil
			consumed = splitAt
			bpIdx++
		}

		if consumed < len(runes) {
			current = append(current, styledRun{
				Text: string(runes[consumed:]),
				ANSI: run.ANSI,
			})
		}

		pos = runEnd
	}

	segments = append(segments, current)
	return segments
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
