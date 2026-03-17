package render

import (
	"regexp"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-runewidth"
	"github.com/muesli/termenv"
)

// PadRight pads a string with spaces to the given display width.
func PadRight(s string, width int) string {
	return lipgloss.NewStyle().Width(width).Render(s)
}

// sgrResetRe matches all SGR full-reset variants: \033[m, \033[0m, \033[00m, etc.
var sgrResetRe = regexp.MustCompile("\x1b\\[0*m")

// PadRightWithBg pads s with spaces to width and applies bgSeq as
// background color across the entire line using raw ANSI sequences.
//
// Content may contain internal SGR full-resets (\033[0m, \033[m, etc.)
// from lipgloss or termenv renders. Each reset is followed by a bgSeq
// re-application so the background persists across the entire line.
func PadRightWithBg(s string, width int, bgSeq string) string {
	visW := ansi.StringWidth(s)
	pad := ""
	if visW < width {
		pad = strings.Repeat(" ", width-visW)
	}
	patched := sgrResetRe.ReplaceAllStringFunc(s, func(match string) string {
		return match + bgSeq
	})
	return bgSeq + patched + pad + termenv.CSI + "0m"
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

// PadBetween places left and right at opposite ends of a line of the given
// display width, filling the gap with spaces. If content is too wide, left
// is truncated with "..." to make room for right.
func PadBetween(left, right string, width int) string {
	leftW := ansi.StringWidth(left)
	rightW := ansi.StringWidth(right)
	gap := width - leftW - rightW
	if gap < 1 {
		left = ansi.Truncate(left, width-rightW-1, "...")
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

// SplitRunsAtBreakpoints divides StyledRuns at the given rune-index
// breakpoints, returning one []StyledRun per visual wrap segment.
// bp must be sorted in ascending order (as returned by WrapBreakpoints).
func SplitRunsAtBreakpoints(runs []StyledRun, bp []int) [][]StyledRun {
	segments := make([][]StyledRun, 0, len(bp)+1)
	var current []StyledRun
	pos := 0
	bpIdx := 0

	for _, run := range runs {
		runes := []rune(run.Text)
		runEnd := pos + len(runes)
		consumed := 0

		for bpIdx < len(bp) && bp[bpIdx] >= pos && bp[bpIdx] < runEnd {
			splitAt := bp[bpIdx] - pos
			if splitAt > consumed {
				current = append(current, StyledRun{
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
			current = append(current, StyledRun{
				Text: string(runes[consumed:]),
				ANSI: run.ANSI,
			})
		}

		pos = runEnd
	}

	segments = append(segments, current)
	return segments
}
