package tui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/708u/gracilius/internal/diff"
	"github.com/708u/gracilius/internal/tui/render"
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

func diffColorsFor(theme render.Theme) diffColors {
	bg := func(hex string) string {
		return termenv.CSI + termenv.RGBColor(hex).Sequence(true) + "m"
	}
	if theme.Name == "github-dark" {
		return diffColors{
			addBg:     bg("#122d1e"),
			delBg:     bg("#351c20"),
			wordAddBg: bg("#1f5c34"),
			wordDelBg: bg("#7e3834"),
			fillerBg:  bg("#222222"),
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
func newDiffSideCtx(data *diff.Data, theme render.Theme, width int) diffSideCtx {
	colors := diffColorsFor(theme)
	maxLine := max(data.MaxLineNum, 1)
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

// spliceGutter replaces the gutter (first gutterW display columns) of one side
// within a pre-rendered diff line. side selects old (offset 0) or new
// (offset sideWidth+separatorWidth). highlightBg is the ANSI background
// sequence to apply; row/isOld determine the line-level background that
// must be restored after the gutter.
func spliceGutter(
	line string,
	side diffSide,
	row diff.Row,
	lineIdx int, // 0 = first visual line, >0 = continuation
	ctx diffSideCtx,
	highlightBg string,
) string {
	sideOff := 0
	lineNum := row.OldLineNum
	isOld := true
	if side == diffSideNew {
		sideOff = ctx.sideWidth + diffSeparatorWidth
		lineNum = row.NewLineNum
		isOld = false
	}

	// Filler sides (lineNum==0) have no gutter to highlight.
	if lineNum == 0 {
		return line
	}

	gutterW := ctx.gutterW

	// Build the highlighted gutter string.
	gutterStyle := highlightBg + render.AnsiFaint
	var gb strings.Builder
	if lineIdx == 0 {
		digits := gutterW - 1
		numStr := fmt.Sprintf("%*d ", digits, lineNum)
		render.WriteStyledText(&gb, gutterStyle, numStr)
	} else {
		render.WriteStyledText(&gb, gutterStyle, ctx.gutterPad)
	}

	// Restore the line-level background after the gutter so that
	// subsequent content retains its original styling.
	lineBg, _ := diffSideBg(row.Type, isOld, ctx.colors)
	if lineBg != "" {
		gb.WriteString(lineBg)
	}

	// Splice: [before sideOff] + [new gutter] + [after sideOff+gutterW]
	before := ansi.Truncate(line, sideOff, "")
	after := ansi.TruncateLeft(line, sideOff+gutterW, "")
	return before + gb.String() + after
}

// renderSingleDiffRow renders one diff row into visual lines.
// oldCtx/newCtx control the gutter background color independently for each side.
// oldSearchHL/newSearchHL are search match highlights for this row (may be nil).
func renderSingleDiffRow(
	row diff.Row,
	oldHL, newHL []render.HighlightedLine,
	oldCtx, newCtx diffSideCtx,
	width int,
	oldSearchHL, newSearchHL []render.HighlightRange,
) []string {
	var oldRuns, newRuns []render.StyledRun
	if row.OldLineNum > 0 && oldHL != nil && row.OldLineNum-1 < len(oldHL) {
		oldRuns = oldHL[row.OldLineNum-1].Runs
	}
	if row.NewLineNum > 0 && newHL != nil && row.NewLineNum-1 < len(newHL) {
		newRuns = newHL[row.NewLineNum-1].Runs
	}

	oldVisuals := wrapDiffSide(row.OldLineNum, row.OldText, row.OldSpans, oldRuns, row.Type, true, oldCtx, oldSearchHL)
	newVisuals := wrapDiffSide(row.NewLineNum, row.NewText, row.NewSpans, newRuns, row.Type, false, newCtx, newSearchHL)

	rowCount := max(len(oldVisuals), len(newVisuals))
	result := make([]string, 0, rowCount)
	for j := range rowCount {
		var sb strings.Builder
		if j < len(oldVisuals) {
			sb.WriteString(oldVisuals[j])
		} else {
			writeDiffFiller(&sb, row.OldLineNum, row.Type, true, oldCtx)
		}
		sb.WriteString(diffSeparator)
		if j < len(newVisuals) {
			sb.WriteString(newVisuals[j])
		} else {
			writeDiffFiller(&sb, row.NewLineNum, row.Type, false, newCtx)
		}
		result = append(result, render.PadRight(sb.String(), width))
	}
	return result
}

// renderAllDiffLines pre-renders all diff rows into a flat visual line slice.
func renderAllDiffLines(data *diff.Data, ctx diffSideCtx, theme render.Theme, width int, oldHL, newHL []render.HighlightedLine, searchMatches []diffSearchMatch) diffRenderResult {
	// Index search matches by row (skip allocation when empty).
	var oldSearchByRow, newSearchByRow map[int][]render.HighlightRange
	if len(searchMatches) > 0 {
		searchMatchBg := theme.SearchMatchBgSeq()
		oldSearchByRow = make(map[int][]render.HighlightRange)
		newSearchByRow = make(map[int][]render.HighlightRange)
		for _, sm := range searchMatches {
			hr := render.HighlightRange{Start: sm.startChar, End: sm.endChar, BgSeq: searchMatchBg}
			if sm.isOld {
				oldSearchByRow[sm.rowIdx] = append(oldSearchByRow[sm.rowIdx], hr)
			} else {
				newSearchByRow[sm.rowIdx] = append(newSearchByRow[sm.rowIdx], hr)
			}
		}
	}

	rowVisualStart := make([]int, len(data.Rows))
	var lines []string

	for i, row := range data.Rows {
		rowVisualStart[i] = len(lines)
		rowLines := renderSingleDiffRow(row, oldHL, newHL, ctx, ctx, width, oldSearchByRow[i], newSearchByRow[i])
		lines = append(lines, rowLines...)
	}

	// Convert hunk start indices (row-based) to visual line offsets.
	hunkOffs := make([]int, len(data.Hunks))
	for i, h := range data.Hunks {
		if h.StartIdx < len(rowVisualStart) {
			hunkOffs[i] = rowVisualStart[h.StartIdx]
		}
	}

	return diffRenderResult{lines: lines, hunkVisualOffs: hunkOffs, rowVisualStarts: rowVisualStart}
}

// diffSideBg returns the line and word background colors for a diff side.
func diffSideBg(rowType diff.RowType, isOld bool, colors diffColors) (lineBg, wordBg string) {
	switch rowType {
	case diff.RowModified:
		if isOld {
			return colors.delBg, colors.wordDelBg
		}
		return colors.addBg, colors.wordAddBg
	case diff.RowAdded:
		return colors.addBg, ""
	case diff.RowDeleted:
		return colors.delBg, ""
	}
	return "", ""
}

// prepareRuns normalises diff-side inputs into a uniform []StyledRun slice.
// When merged=true, the returned runs have fg+bg already combined in ANSI
// and tabs expanded (produced by wordDiffToStyledRuns).
// When merged=false, ANSI contains fg only and tabs are unexpanded.
func prepareRuns(
	text string,
	spans []diff.WordSpan,
	syntaxRuns []render.StyledRun,
	lineBg, wordBg string,
) (runs []render.StyledRun, merged bool) {
	switch {
	case spans != nil:
		return wordDiffToStyledRuns(spans, syntaxRuns, lineBg, wordBg), true
	case syntaxRuns != nil:
		return syntaxRuns, false
	default:
		return []render.StyledRun{{Text: text}}, false
	}
}

// renderRuns is the unified render function for all diff-side content.
// It writes styled, truncated, and padded text into sb.
//   - merged=true: runs already have fg+bg in ANSI, tabs expanded
//   - merged=false: runs have fg only, tabs need expanding, lineBg applied per-chunk
//   - highlights: optional search highlight ranges (later wins)
func renderRuns(
	sb *strings.Builder,
	runs []render.StyledRun,
	textWidth int,
	lineBg string,
	merged bool,
	highlights []render.HighlightRange,
) {
	var raw strings.Builder
	switch {
	case len(highlights) > 0:
		render.RenderStyledLineWithHighlights(&raw, runs, highlights)
	case merged:
		for _, r := range runs {
			render.WriteStyledText(&raw, r.ANSI, r.Text)
		}
	default:
		for _, r := range runs {
			render.WriteColoredChunk(&raw, r.ANSI, lineBg, render.ExpandTabs(r.Text))
		}
	}
	truncated := ansi.Truncate(raw.String(), textWidth, "")
	render.WritePaddedText(sb, truncated, textWidth, lineBg)
}

// runsText concatenates the raw text of all runs.
func runsText(runs []render.StyledRun) string {
	var sb strings.Builder
	for _, r := range runs {
		sb.WriteString(r.Text)
	}
	return sb.String()
}

// wrapDiffSide renders one side of a diff row, returning one string per
// visual line. Long text is soft-wrapped at ctx.textWidth boundaries.
func wrapDiffSide(
	lineNum int,
	text string,
	spans []diff.WordSpan,
	syntaxRuns []render.StyledRun,
	rowType diff.RowType,
	isOld bool,
	ctx diffSideCtx,
	searchHL []render.HighlightRange,
) []string {
	if lineNum == 0 {
		filler := ctx.colors.fillerBg + ctx.fillerPad + render.AnsiReset
		return []string{filler}
	}

	lineBg, wordBg := diffSideBg(rowType, isOld, ctx.colors)
	digits := ctx.gutterW - 1
	gutterBg := lineBg
	if ctx.gutterHighlight != "" {
		gutterBg = ctx.gutterHighlight
	}
	gutterStyle := render.AnsiFaint
	if gutterBg != "" {
		gutterStyle = gutterBg + render.AnsiFaint
	}

	runs, merged := prepareRuns(text, spans, syntaxRuns, lineBg, wordBg)

	var expanded string
	if merged {
		expanded = runsText(runs)
	} else {
		expanded = render.ExpandTabs(text)
	}
	bp := render.WrapBreakpoints(expanded, ctx.textWidth)

	// No soft-wrap: single visual line.
	if bp == nil {
		var sb strings.Builder
		numStr := fmt.Sprintf("%*d ", digits, lineNum)
		render.WriteStyledText(&sb, gutterStyle, numStr)
		renderRuns(&sb, runs, ctx.textWidth, lineBg, merged, searchHL)
		return []string{sb.String()}
	}

	// Soft-wrap: expand tabs for non-merged runs, then split at breakpoints.
	if !merged {
		runs = expandStyledRuns(runs)
	}
	return renderWrappedLines(runs, bp, digits, lineNum, gutterStyle, lineBg, ctx, merged, searchHL)
}

// renderWrappedLines builds visual lines from run segments split at
// breakpoints, prepending the gutter (line number or padding) to each.
// Search highlights are clamped per-segment when present.
func renderWrappedLines(
	runs []render.StyledRun,
	bp []int,
	digits, lineNum int,
	gutterStyle, lineBg string,
	ctx diffSideCtx,
	merged bool,
	searchHL []render.HighlightRange,
) []string {
	runSegments := render.SplitRunsAtBreakpoints(runs, bp)
	segments := make([]string, 0, len(runSegments))
	wrapOff := 0
	for si, seg := range runSegments {
		var sb strings.Builder
		if si == 0 {
			numStr := fmt.Sprintf("%*d ", digits, lineNum)
			render.WriteStyledText(&sb, gutterStyle, numStr)
		} else {
			render.WriteStyledText(&sb, gutterStyle, ctx.gutterPad)
		}
		segLen := 0
		for _, r := range seg {
			segLen += utf8.RuneCountInString(r.Text)
		}
		var segHL []render.HighlightRange
		if len(searchHL) > 0 {
			segHL = render.ClampHighlightsToSegment(searchHL, wrapOff, segLen)
		}
		renderRuns(&sb, seg, ctx.textWidth, lineBg, merged, segHL)
		wrapOff += segLen
		segments = append(segments, sb.String())
	}
	return segments
}

// writeDiffFiller writes a continuation filler line for a side whose
// content has fewer wrap rows than the other side.
func writeDiffFiller(sb *strings.Builder, lineNum int, rowType diff.RowType, isOld bool, ctx diffSideCtx) {
	if lineNum == 0 {
		sb.WriteString(ctx.colors.fillerBg)
		sb.WriteString(ctx.fillerPad)
		sb.WriteString(render.AnsiReset)
		return
	}
	lineBg, _ := diffSideBg(rowType, isOld, ctx.colors)
	gutterStyle := render.AnsiFaint
	if lineBg != "" {
		gutterStyle = lineBg + render.AnsiFaint
	}
	render.WriteStyledText(sb, gutterStyle, ctx.gutterPad)
	render.WritePaddedText(sb, "", ctx.textWidth, lineBg)
}

// expandStyledRuns returns a copy of runs with tabs expanded to spaces.
func expandStyledRuns(runs []render.StyledRun) []render.StyledRun {
	out := make([]render.StyledRun, len(runs))
	for i, r := range runs {
		out[i] = render.StyledRun{Text: render.ExpandTabs(r.Text), ANSI: r.ANSI}
	}
	return out
}

// wordDiffToStyledRuns converts word-diff spans and optional syntax runs
// into a flat slice of styledRuns with fg+bg merged into the ANSI field.
// The returned runs have tabs expanded.
func wordDiffToStyledRuns(
	spans []diff.WordSpan,
	syntaxRuns []render.StyledRun,
	lineBg, wordBg string,
) []render.StyledRun {
	// Build a flat slice of syntax foreground colors aligned by rune position.
	var syntaxFg []string
	for _, r := range syntaxRuns {
		for range []rune(r.Text) {
			syntaxFg = append(syntaxFg, r.ANSI)
		}
	}

	var out []render.StyledRun
	syntaxPos := 0
	for _, span := range spans {
		expanded := render.ExpandTabs(span.Text)
		bg := lineBg
		if span.Op == diff.OpInsert || span.Op == diff.OpDelete {
			bg = wordBg
		}

		spanRunes := []rune(span.Text)
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
				out = append(out, render.StyledRun{Text: chunk, ANSI: ansiCode})
			}
		}
	}
	return out
}

// renderDiffCommentLines renders a comment block inside a single side panel,
// filling the opposite side with spaces. Each returned string is a full-width
// diff line in "old | sep | new" format.
func renderDiffCommentLines(
	blockRows []string,
	side diffSide,
	sideWidth, width int,
) []string {
	filler := strings.Repeat(" ", sideWidth)
	lines := make([]string, len(blockRows))
	for i, r := range blockRows {
		// Truncate before padding to prevent lipgloss wrapping.
		sideContent := render.PadRight(ansi.Truncate(r, sideWidth, ""), sideWidth)
		var sb strings.Builder
		if side == diffSideOld {
			sb.WriteString(sideContent)
			sb.WriteString(diffSeparator)
			sb.WriteString(filler)
		} else {
			sb.WriteString(filler)
			sb.WriteString(diffSeparator)
			sb.WriteString(sideContent)
		}
		lines[i] = render.PadRight(sb.String(), width)
	}
	return lines
}

// interleaveCommentBlocks inserts rendered comment blocks into the diff
// render result after rows that have comments ending on them.
// Active textarea blocks (inputMode) are NOT included here; they are
// overlaid later in renderDiffEditor.
func (t *tab) interleaveCommentBlocks(result diffRenderResult, sideWidth, width int) diffRenderResult {
	if len(t.comments) == 0 {
		return result
	}

	// Build a map: diff row index → comment index for comments ending at that row.
	type commentRef struct {
		idx  int
		side diffSide
	}
	rowComments := map[int][]commentRef{}
	for ci := range t.comments {
		if t.comments[ci].Side == "" {
			continue
		}
		side := diffSideFromString(t.comments[ci].Side)
		endLine := t.comments[ci].EndLine
		// Find the diff row matching this endLine + side.
		if t.diffViewData != nil {
			for ri, row := range t.diffViewData.Rows {
				if diffRowLineNumForSide(row, side) == endLine &&
					diffRowAvailableSide(row, side) == side {
					rowComments[ri] = append(rowComments[ri], commentRef{idx: ci, side: side})
					break
				}
			}
		}
	}

	if len(rowComments) == 0 {
		return result
	}

	var newLines []string
	newRowStarts := make([]int, len(result.rowVisualStarts))
	hunkOffs := make([]int, len(result.hunkVisualOffs))

	// Build reverse map from original visual offset to hunk indices.
	hunkByOrigOff := make(map[int][]int, len(result.hunkVisualOffs))
	for hi, ho := range result.hunkVisualOffs {
		hunkByOrigOff[ho] = append(hunkByOrigOff[ho], hi)
	}

	for ri := range result.rowVisualStarts {
		newRowStarts[ri] = len(newLines)

		// Update hunk offsets via map lookup.
		if his, ok := hunkByOrigOff[result.rowVisualStarts[ri]]; ok {
			for _, hi := range his {
				hunkOffs[hi] = len(newLines)
			}
		}

		// Copy this row's visual lines.
		rowEnd := len(result.lines)
		if ri+1 < len(result.rowVisualStarts) {
			rowEnd = result.rowVisualStarts[ri+1]
		}
		for j := result.rowVisualStarts[ri]; j < rowEnd; j++ {
			newLines = append(newLines, result.lines[j])
		}

		// Insert comment blocks after this row.
		if refs, ok := rowComments[ri]; ok {
			for _, ref := range refs {
				c := &t.comments[ref.idx]
				label := formatDiffCommentLabel(c, ref.side)
				blockBodyWidth := sideWidth - commentBlockMargin
				blockRows := renderBlock(
					c.Text, label, blockBodyWidth, styleComment, styleBodyWhite)
				newLines = append(newLines,
					renderDiffCommentLines(blockRows, ref.side, sideWidth, width)...)
			}
		}
	}

	return diffRenderResult{
		lines:           newLines,
		hunkVisualOffs:  hunkOffs,
		rowVisualStarts: newRowStarts,
	}
}
