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

// diffRenderResult holds pre-rendered diff lines and row-to-visual-line mapping.
type diffRenderResult struct {
	lines          []string // flat visual lines for viewport content
	hunkVisualOffs []int    // visual line offset for each hunk
}

// renderAllDiffLines pre-renders all diff rows into a flat visual line slice.
func renderAllDiffLines(data *diffData, theme themeConfig, width int, oldHL, newHL []highlightedLine) diffRenderResult {
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

	// Build row-start mapping for hunk offset conversion.
	rowVisualStart := make([]int, len(data.rows))
	var lines []string

	for i, row := range data.rows {
		rowVisualStart[i] = len(lines)

		var oldRuns, newRuns []styledRun
		if row.oldLineNum > 0 && oldHL != nil && row.oldLineNum-1 < len(oldHL) {
			oldRuns = oldHL[row.oldLineNum-1].runs
		}
		if row.newLineNum > 0 && newHL != nil && row.newLineNum-1 < len(newHL) {
			newRuns = newHL[row.newLineNum-1].runs
		}

		oldVisuals := wrapDiffSide(row.oldLineNum, row.oldText, row.oldSpans, oldRuns, row.rowType, true, ctx)
		newVisuals := wrapDiffSide(row.newLineNum, row.newText, row.newSpans, newRuns, row.rowType, false, ctx)

		rowCount := max(len(oldVisuals), len(newVisuals))
		for j := range rowCount {
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

	// Convert hunk start indices (row-based) to visual line offsets.
	hunkOffs := make([]int, len(data.hunks))
	for i, h := range data.hunks {
		if h.startIdx < len(rowVisualStart) {
			hunkOffs[i] = rowVisualStart[h.startIdx]
		}
	}

	return diffRenderResult{lines: lines, hunkVisualOffs: hunkOffs}
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
	syntaxRuns []styledRun,
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

	// Case A & C: no soft-wrap
	if bp == nil {
		var sb strings.Builder
		numStr := fmt.Sprintf("%*d ", digits, lineNum)
		writeStyledText(&sb, gutterStyle, numStr)
		switch {
		case spans != nil:
			// Case C: word-diff
			renderWordDiffWithSyntax(&sb, spans, syntaxRuns, lineBg, wordBg, ctx.textWidth)
		case syntaxRuns != nil:
			// Case A: syntax highlight without word-diff
			renderSyntaxWithBg(&sb, syntaxRuns, lineBg, ctx.textWidth)
		default:
			truncated := ansi.Truncate(expanded, ctx.textWidth, "")
			writePaddedText(&sb, truncated, ctx.textWidth, lineBg)
		}
		return []string{sb.String()}
	}

	// Case B: soft-wrapped with syntax highlighting
	if syntaxRuns != nil && spans == nil {
		expRuns := expandStyledRuns(syntaxRuns)
		runSegments := splitRunsAtBreakpoints(expRuns, bp)
		segments := make([]string, 0, len(runSegments))
		for si, seg := range runSegments {
			var sb strings.Builder
			if si == 0 {
				numStr := fmt.Sprintf("%*d ", digits, lineNum)
				writeStyledText(&sb, gutterStyle, numStr)
			} else {
				writeStyledText(&sb, gutterStyle, ctx.gutterPad)
			}
			renderSyntaxWithBg(&sb, seg, lineBg, ctx.textWidth)
			segments = append(segments, sb.String())
		}
		return segments
	}

	// Soft-wrapped without syntax: split into segments.
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

// writeColoredChunk writes text with optional foreground and background ANSI codes.
func writeColoredChunk(sb *strings.Builder, fg, bg, text string) {
	if fg != "" || bg != "" {
		sb.WriteString(fg)
		sb.WriteString(bg)
		sb.WriteString(text)
		sb.WriteString(ansiReset)
	} else {
		sb.WriteString(text)
	}
}

// expandStyledRuns returns a copy of runs with tabs expanded to spaces.
func expandStyledRuns(runs []styledRun) []styledRun {
	out := make([]styledRun, len(runs))
	for i, r := range runs {
		out[i] = styledRun{Text: expandTabs(r.Text), ANSI: r.ANSI}
	}
	return out
}

// renderSyntaxWithBg renders styledRuns with a diff background color,
// truncating to textWidth and padding with spaces.
func renderSyntaxWithBg(sb *strings.Builder, runs []styledRun, bg string, textWidth int) {
	var raw strings.Builder
	for _, r := range runs {
		expanded := expandTabs(r.Text)
		writeColoredChunk(&raw, r.ANSI, bg, expanded)
	}
	truncated := ansi.Truncate(raw.String(), textWidth, "")
	writePaddedText(sb, truncated, textWidth, bg)
}

// renderWordDiffWithSyntax merges word-diff spans with syntax runs,
// applying both foreground (syntax) and background (diff) colors.
// Falls back to renderWordDiffText when syntaxRuns is nil.
func renderWordDiffWithSyntax(
	sb *strings.Builder,
	spans []wordSpan,
	syntaxRuns []styledRun,
	lineBg, wordBg string,
	textWidth int,
) {
	if syntaxRuns == nil {
		renderWordDiffText(sb, spans, lineBg, wordBg, textWidth)
		return
	}

	// Build a flat slice of syntax foreground colors aligned by rune position.
	var syntaxFg []string
	for _, r := range syntaxRuns {
		for range []rune(r.Text) {
			syntaxFg = append(syntaxFg, r.ANSI)
		}
	}

	var raw strings.Builder
	syntaxPos := 0
	for _, span := range spans {
		expanded := expandTabs(span.text)
		bg := lineBg
		if span.op == diffOpInsert || span.op == diffOpDelete {
			bg = wordBg
		}

		spanRunes := []rune(span.text)
		expandedRunes := []rune(expanded)

		// Walk original runes to track the syntax position correctly.
		ei := 0
		for oi := range spanRunes {
			var fg string
			if syntaxPos < len(syntaxFg) {
				fg = syntaxFg[syntaxPos]
			}
			syntaxPos++

			// Determine how many expanded runes this original rune covers.
			advanceBy := 1
			if spanRunes[oi] == '\t' {
				advanceBy = 4 // expandTabs converts to 4 spaces
			}

			chunk := string(expandedRunes[ei : ei+advanceBy])
			ei += advanceBy
			writeColoredChunk(&raw, fg, bg, chunk)
		}
	}

	truncated := ansi.Truncate(raw.String(), textWidth, "")
	writePaddedText(sb, truncated, textWidth, lineBg)
}
