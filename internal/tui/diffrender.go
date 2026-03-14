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
			addBg:     bg("#0d2818"),
			delBg:     bg("#2c1519"),
			wordAddBg: bg("#174928"),
			wordDelBg: bg("#6e302b"),
			fillerBg:  bg("#1e1e1e"),
		}
	}
	return diffColors{
		addBg:     bg("#dafbe1"),
		delBg:     bg("#ffebe9"),
		wordAddBg: bg("#ccffd8"),
		wordDelBg: bg("#ffd7d5"),
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
	sideWidth       int
	gutterW         int
	textWidth       int
	colors          diffColors
	fillerPad       string // precomputed spaces for filler lines (sideWidth)
	gutterPad       string // precomputed spaces for continuation gutter
	gutterHighlight string // if non-empty, override gutter background for cursor/selection
}

// diffRenderResult holds pre-rendered diff lines and row-to-visual-line mapping.
type diffRenderResult struct {
	lines           []string // flat visual lines for viewport content
	hunkVisualOffs  []int    // visual line offset for each hunk
	rowVisualStarts []int    // logical row index → visual line offset
}

// newDiffSideCtx creates a diffSideCtx from diff data, theme, and viewport width.
func newDiffSideCtx(data *diffData, theme themeConfig, width int) diffSideCtx {
	colors := diffColorsFor(theme)
	maxLine := max(data.maxLineNum, 1)
	sideWidth := (width - diffSeparatorWidth) / 2
	gutterW := diffGutterWidth(maxLine)
	return diffSideCtx{
		sideWidth: sideWidth,
		gutterW:   gutterW,
		textWidth: max(sideWidth-gutterW, 1),
		colors:    colors,
		fillerPad: strings.Repeat(" ", sideWidth),
		gutterPad: strings.Repeat(" ", gutterW),
	}
}

// renderSingleDiffRow renders one diff row into visual lines.
// ctx.gutterHighlight controls the gutter background color for cursor/selection.
// oldSearchHL/newSearchHL are search match highlights for this row (may be nil).
func renderSingleDiffRow(
	row diffRow,
	oldHL, newHL []highlightedLine,
	ctx diffSideCtx,
	width int,
	oldSearchHL, newSearchHL []highlightRange,
) []string {
	var oldRuns, newRuns []styledRun
	if row.oldLineNum > 0 && oldHL != nil && row.oldLineNum-1 < len(oldHL) {
		oldRuns = oldHL[row.oldLineNum-1].runs
	}
	if row.newLineNum > 0 && newHL != nil && row.newLineNum-1 < len(newHL) {
		newRuns = newHL[row.newLineNum-1].runs
	}

	oldVisuals := wrapDiffSide(row.oldLineNum, row.oldText, row.oldSpans, oldRuns, row.rowType, true, ctx, oldSearchHL)
	newVisuals := wrapDiffSide(row.newLineNum, row.newText, row.newSpans, newRuns, row.rowType, false, ctx, newSearchHL)

	rowCount := max(len(oldVisuals), len(newVisuals))
	result := make([]string, 0, rowCount)
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
		result = append(result, padRight(sb.String(), width))
	}
	return result
}

// renderAllDiffLines pre-renders all diff rows into a flat visual line slice.
func renderAllDiffLines(data *diffData, theme themeConfig, width int, oldHL, newHL []highlightedLine, searchMatches []diffSearchMatch) diffRenderResult {
	ctx := newDiffSideCtx(data, theme, width)

	// Index search matches by row (skip allocation when empty).
	var oldSearchByRow, newSearchByRow map[int][]highlightRange
	if len(searchMatches) > 0 {
		searchMatchBg := theme.searchMatchBgSeq()
		oldSearchByRow = make(map[int][]highlightRange)
		newSearchByRow = make(map[int][]highlightRange)
		for _, sm := range searchMatches {
			hr := highlightRange{start: sm.startChar, end: sm.endChar, bgSeq: searchMatchBg}
			if sm.isOld {
				oldSearchByRow[sm.rowIdx] = append(oldSearchByRow[sm.rowIdx], hr)
			} else {
				newSearchByRow[sm.rowIdx] = append(newSearchByRow[sm.rowIdx], hr)
			}
		}
	}

	rowVisualStart := make([]int, len(data.rows))
	var lines []string

	for i, row := range data.rows {
		rowVisualStart[i] = len(lines)
		rowLines := renderSingleDiffRow(row, oldHL, newHL, ctx, width, oldSearchByRow[i], newSearchByRow[i])
		lines = append(lines, rowLines...)
	}

	// Convert hunk start indices (row-based) to visual line offsets.
	hunkOffs := make([]int, len(data.hunks))
	for i, h := range data.hunks {
		if h.startIdx < len(rowVisualStart) {
			hunkOffs[i] = rowVisualStart[h.startIdx]
		}
	}

	return diffRenderResult{lines: lines, hunkVisualOffs: hunkOffs, rowVisualStarts: rowVisualStart}
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
	searchHL []highlightRange,
) []string {
	if lineNum == 0 {
		filler := ctx.colors.fillerBg + ctx.fillerPad + ansiReset
		return []string{filler}
	}

	lineBg, wordBg := diffSideBg(rowType, isOld, ctx.colors)
	digits := ctx.gutterW - 1
	gutterBg := lineBg
	if ctx.gutterHighlight != "" {
		gutterBg = ctx.gutterHighlight
	}
	gutterStyle := ansiFaint
	if gutterBg != "" {
		gutterStyle = gutterBg + ansiFaint
	}

	expanded := expandTabs(text)
	bp := wrapBreakpoints(expanded, ctx.textWidth)

	// No soft-wrap: single visual line.
	if bp == nil {
		var sb strings.Builder
		numStr := fmt.Sprintf("%*d ", digits, lineNum)
		writeStyledText(&sb, gutterStyle, numStr)
		switch {
		case len(searchHL) > 0 && syntaxRuns != nil && spans == nil:
			// Search highlight with syntax: merge search bg into syntax runs.
			renderSyntaxWithBgAndHighlights(&sb, syntaxRuns, lineBg, ctx.textWidth, searchHL)
		case spans != nil:
			renderWordDiffWithSyntax(&sb, spans, syntaxRuns, lineBg, wordBg, ctx.textWidth)
		case syntaxRuns != nil:
			renderSyntaxWithBg(&sb, syntaxRuns, lineBg, ctx.textWidth)
		case len(searchHL) > 0:
			// Plain text with search highlights.
			runs := []styledRun{{Text: text}}
			renderSyntaxWithBgAndHighlights(&sb, runs, lineBg, ctx.textWidth, searchHL)
		default:
			truncated := ansi.Truncate(expanded, ctx.textWidth, "")
			writePaddedText(&sb, truncated, ctx.textWidth, lineBg)
		}
		return []string{sb.String()}
	}

	// Soft-wrapped with syntax or word-diff styling.
	if syntaxRuns != nil || spans != nil {
		var runs []styledRun
		if spans != nil {
			runs = wordDiffToStyledRuns(spans, syntaxRuns, lineBg, wordBg)
		} else {
			runs = expandStyledRuns(syntaxRuns)
		}
		renderFn := renderMergedRuns
		if spans == nil {
			renderFn = renderSyntaxWithBg
		}
		if len(searchHL) > 0 && spans == nil {
			return renderWrappedSegmentsWithHighlights(runs, bp, digits, lineNum, gutterStyle, lineBg, ctx, searchHL)
		}
		return renderWrappedSegments(runs, bp, digits, lineNum, gutterStyle, lineBg, ctx, renderFn)
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

// renderWrappedSegments builds visual lines from run segments split at
// breakpoints, prepending the gutter (line number or padding) to each.
func renderWrappedSegments(
	runs []styledRun,
	bp []int,
	digits, lineNum int,
	gutterStyle, lineBg string,
	ctx diffSideCtx,
	renderFn func(sb *strings.Builder, runs []styledRun, bg string, textWidth int),
) []string {
	runSegments := splitRunsAtBreakpoints(runs, bp)
	segments := make([]string, 0, len(runSegments))
	for si, seg := range runSegments {
		var sb strings.Builder
		if si == 0 {
			numStr := fmt.Sprintf("%*d ", digits, lineNum)
			writeStyledText(&sb, gutterStyle, numStr)
		} else {
			writeStyledText(&sb, gutterStyle, ctx.gutterPad)
		}
		renderFn(&sb, seg, lineBg, ctx.textWidth)
		segments = append(segments, sb.String())
	}
	return segments
}

// renderWrappedSegmentsWithHighlights is like renderWrappedSegments but
// overlays search highlights on each segment.
func renderWrappedSegmentsWithHighlights(
	runs []styledRun,
	bp []int,
	digits, lineNum int,
	gutterStyle, lineBg string,
	ctx diffSideCtx,
	searchHL []highlightRange,
) []string {
	runSegments := splitRunsAtBreakpoints(runs, bp)
	segments := make([]string, 0, len(runSegments))
	wrapOff := 0
	for si, seg := range runSegments {
		var sb strings.Builder
		if si == 0 {
			numStr := fmt.Sprintf("%*d ", digits, lineNum)
			writeStyledText(&sb, gutterStyle, numStr)
		} else {
			writeStyledText(&sb, gutterStyle, ctx.gutterPad)
		}
		segLen := 0
		for _, r := range seg {
			segLen += len([]rune(r.Text))
		}
		segHL := clampHighlightsToSegment(searchHL, wrapOff, segLen)
		renderSyntaxWithBgAndHighlights(&sb, seg, lineBg, ctx.textWidth, segHL)
		wrapOff += segLen
		segments = append(segments, sb.String())
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

// renderSyntaxWithBgAndHighlights renders styledRuns with a diff background
// and overlaid search highlights, truncating to textWidth and padding.
func renderSyntaxWithBgAndHighlights(sb *strings.Builder, runs []styledRun, bg string, textWidth int, highlights []highlightRange) {
	if len(highlights) == 0 {
		renderSyntaxWithBg(sb, runs, bg, textWidth)
		return
	}
	var raw strings.Builder
	renderStyledLineWithHighlights(&raw, runs, highlights)
	truncated := ansi.Truncate(raw.String(), textWidth, "")
	writePaddedText(sb, truncated, textWidth, bg)
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

	runs := wordDiffToStyledRuns(spans, syntaxRuns, lineBg, wordBg)
	renderMergedRuns(sb, runs, lineBg, textWidth)
}

// wordDiffToStyledRuns converts word-diff spans and optional syntax runs
// into a flat slice of styledRuns with fg+bg merged into the ANSI field.
// The returned runs have tabs expanded.
func wordDiffToStyledRuns(
	spans []wordSpan,
	syntaxRuns []styledRun,
	lineBg, wordBg string,
) []styledRun {
	// Build a flat slice of syntax foreground colors aligned by rune position.
	var syntaxFg []string
	for _, r := range syntaxRuns {
		for range []rune(r.Text) {
			syntaxFg = append(syntaxFg, r.ANSI)
		}
	}

	var out []styledRun
	syntaxPos := 0
	for _, span := range spans {
		expanded := expandTabs(span.text)
		bg := lineBg
		if span.op == diffOpInsert || span.op == diffOpDelete {
			bg = wordBg
		}

		spanRunes := []rune(span.text)
		expandedRunes := []rune(expanded)

		ei := 0
		for oi := range spanRunes {
			var fg string
			if syntaxPos < len(syntaxFg) {
				fg = syntaxFg[syntaxPos]
			}
			syntaxPos++

			advanceBy := 1
			if spanRunes[oi] == '\t' {
				advanceBy = 4
			}

			chunk := string(expandedRunes[ei : ei+advanceBy])
			ei += advanceBy

			ansiCode := fg + bg
			// Merge with previous run if same ANSI code.
			if len(out) > 0 && out[len(out)-1].ANSI == ansiCode {
				out[len(out)-1].Text += chunk
			} else {
				out = append(out, styledRun{Text: chunk, ANSI: ansiCode})
			}
		}
	}
	return out
}

// renderMergedRuns renders styledRuns whose ANSI fields already contain
// combined fg+bg codes. Unlike renderSyntaxWithBg, no additional bg
// is applied per-run; padBg is used only for trailing padding.
func renderMergedRuns(sb *strings.Builder, runs []styledRun, padBg string, textWidth int) {
	var raw strings.Builder
	for _, r := range runs {
		writeStyledText(&raw, r.ANSI, r.Text)
	}
	truncated := ansi.Truncate(raw.String(), textWidth, "")
	writePaddedText(sb, truncated, textWidth, padBg)
}
