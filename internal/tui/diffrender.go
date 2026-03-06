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
}

// renderSideBySide renders a side-by-side diff view.
// It returns exactly height lines, each padded to width.
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
	}

	lines := make([]string, 0, height)

	for i := offset; i < len(data.rows) && len(lines) < height; i++ {
		row := data.rows[i]

		var sb strings.Builder
		renderDiffSide(&sb, row.oldLineNum, row.oldText, row.oldSpans, row.rowType, true, ctx)
		sb.WriteString(diffSeparator)
		renderDiffSide(&sb, row.newLineNum, row.newText, row.newSpans, row.rowType, false, ctx)

		lines = append(lines, padRight(sb.String(), width))
	}

	for len(lines) < height {
		lines = append(lines, padRight("", width))
	}

	return lines
}

// renderDiffSide renders one side (old or new) of a diff row.
func renderDiffSide(
	sb *strings.Builder,
	lineNum int,
	text string,
	spans []wordSpan,
	rowType diffRowType,
	isOld bool,
	ctx diffSideCtx,
) {
	if lineNum == 0 {
		sb.WriteString(ctx.colors.fillerBg)
		sb.WriteString(strings.Repeat(" ", ctx.sideWidth))
		sb.WriteString(ansiReset)
		return
	}

	var lineBg, wordBg string
	switch rowType {
	case diffRowModified:
		if isOld {
			lineBg = ctx.colors.delBg
			wordBg = ctx.colors.wordDelBg
		} else {
			lineBg = ctx.colors.addBg
			wordBg = ctx.colors.wordAddBg
		}
	case diffRowAdded:
		lineBg = ctx.colors.addBg
	case diffRowDeleted:
		lineBg = ctx.colors.delBg
	}

	digits := ctx.gutterW - 1
	numStr := fmt.Sprintf("%*d ", digits, lineNum)

	if lineBg != "" {
		writeStyledText(sb, lineBg+ansiFaint, numStr)
	} else {
		writeStyledText(sb, ansiFaint, numStr)
	}

	if spans != nil {
		renderWordDiffText(sb, spans, lineBg, wordBg, ctx.textWidth)
	} else {
		expanded := expandTabs(text)
		truncated := ansi.Truncate(expanded, ctx.textWidth, "")
		writePaddedText(sb, truncated, ctx.textWidth, lineBg)
	}
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
