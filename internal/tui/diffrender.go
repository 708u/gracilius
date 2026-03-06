package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

const (
	diffSeparator      = " \u2502 "
	diffSeparatorWidth = 3
)

// diffColors holds ANSI SGR sequences for diff row backgrounds.
type diffColors struct {
	addBg     string // added line background
	delBg     string // deleted line background
	wordAddBg string // word-level added highlight
	wordDelBg string // word-level deleted highlight
	fillerBg  string // filler line background
}

func diffColorsFor(theme themeConfig) diffColors {
	bg := func(hex string) string {
		return termenv.CSI + termenv.RGBColor(hex).Sequence(true) + "m"
	}
	if theme.name == "github-dark" {
		return diffColors{
			addBg:     bg("#1a3a2a"),
			delBg:     bg("#3a1a1a"),
			wordAddBg: bg("#2ea043"),
			wordDelBg: bg("#f85149"),
			fillerBg:  bg("#1e1e1e"),
		}
	}
	return diffColors{
		addBg:     bg("#d4f8d4"),
		delBg:     bg("#f8d4d4"),
		wordAddBg: bg("#acf2bd"),
		wordDelBg: bg("#fdb8c0"),
		fillerBg:  bg("#f0f0f0"),
	}
}

// diffGutterWidth returns the gutter width for line numbers.
// The result includes a trailing space.
func diffGutterWidth(maxLineNum int) int {
	digits := 1
	n := maxLineNum
	for n >= 10 {
		n /= 10
		digits++
	}
	return digits + 1
}

// diffSideCtx holds layout and color parameters for rendering one side.
type diffSideCtx struct {
	sideWidth int
	gutterW   int
	textWidth int
	colors    diffColors
	fillerPad string // precomputed spaces for filler lines (sideWidth)
	gutterPad string // precomputed spaces for continuation gutter
}

// renderSideBySide renders a side-by-side diff view.
// It returns exactly height lines, each padded to width.
// Long lines are soft-wrapped within each side.
func renderSideBySide(
	data *diffData,
	theme themeConfig,
	width int,
	height int,
	offset int,
) []string {
	if height <= 0 {
		return nil
	}

	colors := diffColorsFor(theme)
	maxLine := max(data.maxLineNum, 1)

	sideWidth := (width - diffSeparatorWidth) / 2
	gutterW := diffGutterWidth(maxLine)
	ctx := diffSideCtx{
		sideWidth: sideWidth,
		gutterW:   gutterW,
		textWidth: max(sideWidth-gutterW, 1),
		colors:    colors,
		fillerPad: strings.Repeat(" ", sideWidth),
		gutterPad: strings.Repeat(" ", gutterW),
	}

	lines := make([]string, 0, height)

	for i := offset; i < len(data.rows) && len(lines) < height; i++ {
		row := data.rows[i]

		oldVisuals := wrapDiffSide(row.oldLineNum, row.oldText, row.oldSpans, row.rowType, true, ctx)
		newVisuals := wrapDiffSide(row.newLineNum, row.newText, row.newSpans, row.rowType, false, ctx)

		rowCount := max(len(oldVisuals), len(newVisuals))
		for j := range rowCount {
			if len(lines) >= height {
				break
			}
			var sb strings.Builder
			if j < len(oldVisuals) {
				sb.WriteString(oldVisuals[j])
			} else {
				writeDiffFiller(&sb, row.oldLineNum, row.rowType, true, ctx)
			}
			sb.WriteString(diffSeparator)
			if j < len(newVisuals) {
				sb.WriteString(newVisuals[j])
			} else {
				writeDiffFiller(&sb, row.newLineNum, row.rowType, false, ctx)
			}
			lines = append(lines, padRight(sb.String(), width))
		}
	}

	for len(lines) < height {
		lines = append(lines, padRight("", width))
	}

	return lines
}

// diffSideBg returns the line and word background colors for a diff side.
func diffSideBg(rowType diffRowType, isOld bool, colors diffColors) (lineBg, wordBg string) {
	switch rowType {
	case diffRowModified:
		if isOld {
			return colors.delBg, colors.wordDelBg
		}
		return colors.addBg, colors.wordAddBg
	case diffRowAdded:
		return colors.addBg, ""
	case diffRowDeleted:
		return colors.delBg, ""
	}
	return "", ""
}

// wrapDiffSide renders one side of a diff row, returning one string per
// visual line. Long text is soft-wrapped at ctx.textWidth boundaries.
func wrapDiffSide(
	lineNum int,
	text string,
	spans []wordSpan,
	rowType diffRowType,
	isOld bool,
	ctx diffSideCtx,
) []string {
	if lineNum == 0 {
		filler := ctx.colors.fillerBg + ctx.fillerPad + ansiReset
		return []string{filler}
	}

	lineBg, wordBg := diffSideBg(rowType, isOld, ctx.colors)
	digits := ctx.gutterW - 1
	gutterStyle := ansiFaint
	if lineBg != "" {
		gutterStyle = lineBg + ansiFaint
	}

	expanded := expandTabs(text)
	bp := wrapBreakpoints(expanded, ctx.textWidth)

	if bp == nil {
		var sb strings.Builder
		numStr := fmt.Sprintf("%*d ", digits, lineNum)
		writeStyledText(&sb, gutterStyle, numStr)
		if spans != nil {
			renderWordDiffText(&sb, spans, lineBg, wordBg, ctx.textWidth)
		} else {
			truncated := ansi.Truncate(expanded, ctx.textWidth, "")
			writePaddedText(&sb, truncated, ctx.textWidth, lineBg)
		}
		return []string{sb.String()}
	}

	// Soft-wrapped: split into segments.
	runes := []rune(expanded)
	segments := make([]string, 0, len(bp)+1)
	prev := 0
	for si := 0; si <= len(bp); si++ {
		end := len(runes)
		if si < len(bp) {
			end = bp[si]
		}
		seg := string(runes[prev:end])

		var sb strings.Builder
		if si == 0 {
			numStr := fmt.Sprintf("%*d ", digits, lineNum)
			writeStyledText(&sb, gutterStyle, numStr)
		} else {
			writeStyledText(&sb, gutterStyle, ctx.gutterPad)
		}
		writePaddedText(&sb, seg, ctx.textWidth, lineBg)
		segments = append(segments, sb.String())
		prev = end
	}
	return segments
}

// writeDiffFiller writes a continuation filler line for a side whose
// content has fewer wrap rows than the other side.
func writeDiffFiller(sb *strings.Builder, lineNum int, rowType diffRowType, isOld bool, ctx diffSideCtx) {
	if lineNum == 0 {
		sb.WriteString(ctx.colors.fillerBg)
		sb.WriteString(ctx.fillerPad)
		sb.WriteString(ansiReset)
		return
	}
	lineBg, _ := diffSideBg(rowType, isOld, ctx.colors)
	gutterStyle := ansiFaint
	if lineBg != "" {
		gutterStyle = lineBg + ansiFaint
	}
	writeStyledText(sb, gutterStyle, ctx.gutterPad)
	writePaddedText(sb, "", ctx.textWidth, lineBg)
}

// renderWordDiffText renders word-level diff spans with background highlights.
func renderWordDiffText(
	sb *strings.Builder,
	spans []wordSpan,
	lineBg string,
	wordBg string,
	textWidth int,
) {
	var raw strings.Builder
	for _, s := range spans {
		expanded := expandTabs(s.text)
		bg := lineBg
		if s.op == diffOpInsert || s.op == diffOpDelete {
			bg = wordBg
		}
		if bg != "" {
			raw.WriteString(bg)
			raw.WriteString(expanded)
			raw.WriteString(ansiReset)
		} else {
			raw.WriteString(expanded)
		}
	}

	truncated := ansi.Truncate(raw.String(), textWidth, "")
	writePaddedText(sb, truncated, textWidth, lineBg)
}

// writePaddedText writes truncated text to sb, padding to targetWidth
// with optional background color.
func writePaddedText(sb *strings.Builder, truncated string, targetWidth int, bg string) {
	if bg != "" {
		sb.WriteString(bg)
	}
	sb.WriteString(truncated)
	if visW := ansi.StringWidth(truncated); visW < targetWidth {
		sb.WriteString(strings.Repeat(" ", targetWidth-visW))
	}
	if bg != "" {
		sb.WriteString(ansiReset)
	}
}
