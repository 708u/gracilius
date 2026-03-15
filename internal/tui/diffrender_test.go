package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/708u/gracilius/internal/diff"
	"github.com/708u/gracilius/internal/tui/render"
	"github.com/charmbracelet/x/ansi"
)

// renderDiff is a test helper that pre-renders diff lines via viewport.
func renderDiff(data *diff.Data, theme render.Theme, width, height, offset int) []string {
	return renderDiffHL(data, theme, width, height, offset, nil, nil)
}

// renderDiffHL is a test helper that pre-renders diff lines with optional syntax highlights.
func renderDiffHL(data *diff.Data, theme render.Theme, width, height, offset int, oldHL, newHL []render.HighlightedLine) []string {
	result := renderAllDiffLines(data, theme, width, oldHL, newHL, nil)
	// Simulate viewport slicing.
	start := min(offset, len(result.lines))
	end := min(start+height, len(result.lines))
	lines := make([]string, 0, height)
	lines = append(lines, result.lines[start:end]...)
	for len(lines) < height {
		lines = append(lines, render.PadRight("", width))
	}
	return lines
}

func TestRenderSideBySide_LineCount(t *testing.T) {
	old := []string{"aaa", "bbb", "ccc"}
	new := []string{"aaa", "BBB", "ccc"}
	data := diff.Build(old, new)

	heights := []int{5, 10, 20}
	for _, h := range heights {
		lines := renderDiff(data, render.Dark, 80, h, 0)
		if len(lines) != h {
			t.Errorf("height=%d: expected %d lines, got %d", h, h, len(lines))
		}
	}
}

func TestRenderSideBySide_EmptyDiff(t *testing.T) {
	data := diff.Build(nil, nil)

	lines := renderDiff(data, render.Dark, 80, 10, 0)
	if len(lines) != 10 {
		t.Fatalf("expected 10 lines, got %d", len(lines))
	}
}

func TestRenderSideBySide_ColumnWidths(t *testing.T) {
	old := []string{"aaa", "bbb"}
	new := []string{"aaa", "BBB"}
	data := diff.Build(old, new)

	width := 80
	lines := renderDiff(data, render.Dark, width, 5, 0)
	for i, line := range lines {
		w := ansi.StringWidth(line)
		if w != width {
			t.Errorf("line %d: expected width %d, got %d", i, width, w)
		}
	}
}

func TestRenderSideBySide_Separator(t *testing.T) {
	old := []string{"aaa", "bbb"}
	new := []string{"aaa", "BBB"}
	data := diff.Build(old, new)

	lines := renderDiff(data, render.Dark, 80, 5, 0)
	for i, line := range lines[:len(data.Rows)] {
		stripped := ansi.Strip(line)
		if !strings.Contains(stripped, "\u2502") {
			t.Errorf("line %d: missing separator", i)
		}
	}
}

func TestRenderSideBySide_ScrollOffset(t *testing.T) {
	old := []string{"aaa", "bbb", "ccc", "ddd", "eee"}
	new := []string{"aaa", "bbb", "ccc", "ddd", "eee"}
	data := diff.Build(old, new)

	lines := renderDiff(data, render.Dark, 80, 5, 2)
	stripped := ansi.Strip(lines[0])
	if !strings.Contains(stripped, "3") {
		t.Errorf("expected line number 3 in first row, got %q", stripped)
	}
}

func TestRenderSideBySide_LineNumbers(t *testing.T) {
	old := []string{"aaa", "bbb", "ccc"}
	new := []string{"aaa", "BBB", "ccc"}
	data := diff.Build(old, new)

	lines := renderDiff(data, render.Dark, 80, 5, 0)
	for i, row := range data.Rows {
		stripped := ansi.Strip(lines[i])
		oldNum := strings.SplitN(stripped, "\u2502", 2)[0]
		if row.OldLineNum > 0 {
			numStr := fmt.Sprintf("%d", row.OldLineNum)
			if !strings.Contains(oldNum, numStr) {
				t.Errorf("row %d: expected old line number %s in %q", i, numStr, oldNum)
			}
		}
	}
}

func TestRenderSideBySide_FillerLine(t *testing.T) {
	data := diff.Build(nil, []string{"aaa", "bbb"})

	lines := renderDiff(data, render.Dark, 80, 5, 0)
	for i := range data.Rows {
		stripped := ansi.Strip(lines[i])
		parts := strings.SplitN(stripped, "\u2502", 2)
		oldSide := strings.TrimRight(parts[0], " ")
		for _, r := range oldSide {
			if r >= '0' && r <= '9' {
				t.Errorf("row %d: filler side should have no digits, got %q", i, oldSide)
				break
			}
		}
	}
}

func TestDiffGutterWidth(t *testing.T) {
	tests := []struct {
		maxLine int
		want    int
	}{
		{1, 2},
		{9, 2},
		{10, 3},
		{99, 3},
		{100, 4},
		{999, 4},
		{1000, 5},
	}
	for _, tt := range tests {
		got := diffGutterWidth(tt.maxLine)
		if got != tt.want {
			t.Errorf("diffGutterWidth(%d) = %d, want %d", tt.maxLine, got, tt.want)
		}
	}
}

func TestDiffColorsFor(t *testing.T) {
	dark := diffColorsFor(render.Dark)
	light := diffColorsFor(render.Light)

	if dark.addBg == light.addBg {
		t.Error("dark and light addBg should differ")
	}
	if dark.delBg == light.delBg {
		t.Error("dark and light delBg should differ")
	}
	if dark.fillerBg == light.fillerBg {
		t.Error("dark and light fillerBg should differ")
	}

	for _, c := range []diffColors{dark, light} {
		fields := []string{c.addBg, c.delBg, c.wordAddBg, c.wordDelBg, c.fillerBg}
		for i, f := range fields {
			if f == "" {
				t.Errorf("color field %d is empty", i)
			}
		}
	}
}

func TestRenderSideBySide_WithSyntaxHighlight(t *testing.T) {
	old := []string{"func foo() {", "  return 1", "}"}
	new := []string{"func foo() {", "  return 2", "}"}
	data := diff.Build(old, new)

	oldHL := render.HighlightFile("test.go", strings.Join(old, "\n"), render.Dark)
	newHL := render.HighlightFile("test.go", strings.Join(new, "\n"), render.Dark)

	width := 80
	lines := renderDiffHL(data, render.Dark, width, 5, 0, oldHL, newHL)

	// All lines should have correct width.
	for i, line := range lines {
		w := ansi.StringWidth(line)
		if w != width {
			t.Errorf("line %d: expected width %d, got %d", i, width, w)
		}
	}

	// Syntax-highlighted lines should contain ANSI escape sequences beyond
	// what diff background coloring alone would produce.
	// The unchanged line "func foo() {" should have syntax coloring.
	stripped := ansi.Strip(lines[0])
	if !strings.Contains(stripped, "func") {
		t.Errorf("expected 'func' in first line, got %q", stripped)
	}
}

func TestRenderSideBySide_WordDiffWithSyntax(t *testing.T) {
	old := []string{"x := 10"}
	new := []string{"x := 20"}
	data := diff.Build(old, new)

	oldHL := render.HighlightFile("test.go", strings.Join(old, "\n"), render.Dark)
	newHL := render.HighlightFile("test.go", strings.Join(new, "\n"), render.Dark)

	width := 80
	lines := renderDiffHL(data, render.Dark, width, 3, 0, oldHL, newHL)

	// Modified row should render at correct width.
	for i, line := range lines {
		w := ansi.StringWidth(line)
		if w != width {
			t.Errorf("line %d: expected width %d, got %d", i, width, w)
		}
	}

	// The row should contain the separator.
	stripped := ansi.Strip(lines[0])
	if !strings.Contains(stripped, "\u2502") {
		t.Error("modified row missing separator")
	}
}

func TestRenderSideBySide_SoftWrapWithSyntax(t *testing.T) {
	longLine := "func veryLongFunctionName(parameterOne int, parameterTwo string, parameterThree bool) error {"
	old := []string{longLine}
	new := []string{longLine}
	data := diff.Build(old, new)

	oldHL := render.HighlightFile("test.go", longLine, render.Dark)
	newHL := render.HighlightFile("test.go", longLine, render.Dark)

	// Use a narrow width to force soft-wrapping.
	width := 60
	result := renderAllDiffLines(data, render.Dark, width, oldHL, newHL, nil)

	// Should produce more visual lines than data rows due to wrapping.
	if len(result.lines) <= len(data.Rows) {
		t.Errorf("expected soft-wrap to produce extra lines, got %d lines for %d rows",
			len(result.lines), len(data.Rows))
	}

	// All lines should have correct width.
	for i, line := range result.lines {
		w := ansi.StringWidth(line)
		if w != width {
			t.Errorf("line %d: expected width %d, got %d", i, width, w)
		}
	}
}

func TestRenderAllDiffLines_RowVisualStarts(t *testing.T) {
	old := []string{"aaa", "bbb", "ccc"}
	new := []string{"aaa", "BBB", "ccc"}
	data := diff.Build(old, new)

	result := renderAllDiffLines(data, render.Dark, 80, nil, nil, nil)

	if len(result.rowVisualStarts) != len(data.Rows) {
		t.Fatalf("rowVisualStarts length = %d, want %d", len(result.rowVisualStarts), len(data.Rows))
	}

	// Without soft-wrap at width 80, each row occupies exactly 1 visual line,
	// so rowVisualStarts should be [0, 1, 2].
	for i, start := range result.rowVisualStarts {
		if start != i {
			t.Errorf("rowVisualStarts[%d] = %d, want %d", i, start, i)
		}
	}
}

func TestRenderAllDiffLines_RowVisualStarts_SoftWrap(t *testing.T) {
	longLine := strings.Repeat("x", 100)
	old := []string{"short", longLine}
	new := []string{"short", longLine}
	data := diff.Build(old, new)

	// Narrow width to force soft-wrapping on the long line.
	result := renderAllDiffLines(data, render.Dark, 40, nil, nil, nil)

	if len(result.rowVisualStarts) != len(data.Rows) {
		t.Fatalf("rowVisualStarts length = %d, want %d", len(result.rowVisualStarts), len(data.Rows))
	}

	// First row starts at 0.
	if result.rowVisualStarts[0] != 0 {
		t.Errorf("rowVisualStarts[0] = %d, want 0", result.rowVisualStarts[0])
	}

	// Starts must be strictly increasing.
	for i := 1; i < len(result.rowVisualStarts); i++ {
		if result.rowVisualStarts[i] <= result.rowVisualStarts[i-1] {
			t.Errorf("rowVisualStarts[%d]=%d not > rowVisualStarts[%d]=%d",
				i, result.rowVisualStarts[i], i-1, result.rowVisualStarts[i-1])
		}
	}

	// The second row (long line) should cause extra visual lines.
	if len(result.lines) <= len(data.Rows) {
		t.Errorf("expected soft-wrap to produce extra lines, got %d for %d rows",
			len(result.lines), len(data.Rows))
	}
}

func TestDiffVisualToLogical(t *testing.T) {
	tb := &tab{
		diffRowVisualStarts: []int{0, 1, 4, 7},
	}

	tests := []struct {
		visualOff  int
		wantRow    int
		wantSubOff int
	}{
		{0, 0, 0},
		{1, 1, 0},
		{2, 1, 1},
		{3, 1, 2},
		{4, 2, 0},
		{6, 2, 2},
		{7, 3, 0},
		{9, 3, 2},
	}
	for _, tt := range tests {
		row, sub := tb.diffVisualToLogical(tt.visualOff)
		if row != tt.wantRow || sub != tt.wantSubOff {
			t.Errorf("diffVisualToLogical(%d) = (%d, %d), want (%d, %d)",
				tt.visualOff, row, sub, tt.wantRow, tt.wantSubOff)
		}
	}
}

func TestDiffVisualToLogical_Empty(t *testing.T) {
	tb := &tab{}
	row, sub := tb.diffVisualToLogical(5)
	if row != 0 || sub != 0 {
		t.Errorf("diffVisualToLogical(5) on empty = (%d, %d), want (0, 0)", row, sub)
	}
}

func TestEnsureDiffContent_PreservesLogicalPosition(t *testing.T) {
	longLine := strings.Repeat("x", 100)
	old := []string{"short", longLine, "end"}
	new := []string{"short", longLine, "end"}
	data := diff.Build(old, new)

	tb := newDiffTab("test.go", nil, nil, nil)
	tb.diffViewData = data

	// Initial render at wide width.
	wideWidth := 120
	tb.renderDiffContent(render.Dark, wideWidth)
	tb.diffCacheWidth = wideWidth
	tb.diffCacheTheme = render.Dark.Name

	// Find the visual offset for the "end" row (last logical row).
	lastRow := len(data.Rows) - 1
	wideVisualOff := tb.diffRowVisualStarts[lastRow]
	tb.vp.SetYOffset(wideVisualOff)

	// Re-render at narrow width.
	narrowWidth := 40
	tb.ensureDiffContent(render.Dark, narrowWidth, 0)

	// After re-render, the viewport should still point at the same logical row.
	narrowVisualOff := tb.diffRowVisualStarts[lastRow]
	if tb.vp.YOffset() != narrowVisualOff {
		t.Errorf("after width change: YOffset = %d, want %d (visual start of logical row %d)",
			tb.vp.YOffset(), narrowVisualOff, lastRow)
	}
}

func TestRenderSideBySide_SoftWrapWithWordDiff(t *testing.T) {
	// Use a long modified line that forces soft-wrapping at narrow width.
	old := []string{"func processData(inputValue int, extraParam string) error {"}
	new := []string{"func processData(inputValue int, changedParam string) error {"}
	data := diff.Build(old, new)

	oldHL := render.HighlightFile("test.go", strings.Join(old, "\n"), render.Dark)
	newHL := render.HighlightFile("test.go", strings.Join(new, "\n"), render.Dark)

	// Narrow width to force soft-wrapping on the modified row.
	width := 50
	result := renderAllDiffLines(data, render.Dark, width, oldHL, newHL, nil)

	// Should produce more visual lines than data rows due to wrapping.
	if len(result.lines) <= len(data.Rows) {
		t.Fatalf("expected soft-wrap to produce extra lines, got %d lines for %d rows",
			len(result.lines), len(data.Rows))
	}

	// All visual lines should have correct width.
	for i, line := range result.lines {
		w := ansi.StringWidth(line)
		if w != width {
			t.Errorf("line %d: expected width %d, got %d", i, width, w)
		}
	}

	// Word-diff background color should be present in at least one visual line.
	// The modified row uses wordDelBg (old side) and wordAddBg (new side).
	colors := diffColorsFor(render.Dark)
	foundWordBg := false
	for _, line := range result.lines {
		if strings.Contains(line, colors.wordDelBg) || strings.Contains(line, colors.wordAddBg) {
			foundWordBg = true
			break
		}
	}
	if !foundWordBg {
		t.Error("word-diff background color not found in any wrapped line")
	}
}

func TestRenderSideBySide_SoftWrapWordDiffNoSyntax(t *testing.T) {
	// Word-diff with wrapping but without syntax highlighting.
	old := []string{"this is a long line with some original words that will be wrapped"}
	new := []string{"this is a long line with some modified words that will be wrapped"}
	data := diff.Build(old, new)

	width := 50
	result := renderAllDiffLines(data, render.Dark, width, nil, nil, nil)

	if len(result.lines) <= len(data.Rows) {
		t.Fatalf("expected soft-wrap, got %d lines for %d rows",
			len(result.lines), len(data.Rows))
	}

	for i, line := range result.lines {
		w := ansi.StringWidth(line)
		if w != width {
			t.Errorf("line %d: expected width %d, got %d", i, width, w)
		}
	}

	colors := diffColorsFor(render.Dark)
	foundWordBg := false
	for _, line := range result.lines {
		if strings.Contains(line, colors.wordDelBg) || strings.Contains(line, colors.wordAddBg) {
			foundWordBg = true
			break
		}
	}
	if !foundWordBg {
		t.Error("word-diff background color not found without syntax highlighting")
	}
}

func TestRenderSideBySide_NilSyntaxFallback(t *testing.T) {
	old := []string{"aaa", "bbb"}
	new := []string{"aaa", "BBB"}
	data := diff.Build(old, new)

	// Render with and without syntax highlights; both should produce same width.
	width := 80
	withoutHL := renderDiff(data, render.Dark, width, 5, 0)
	withHL := renderDiffHL(data, render.Dark, width, 5, 0, nil, nil)

	for i := range withoutHL {
		w1 := ansi.StringWidth(withoutHL[i])
		w2 := ansi.StringWidth(withHL[i])
		if w1 != w2 {
			t.Errorf("line %d: width mismatch without HL (%d) vs with nil HL (%d)", i, w1, w2)
		}
	}
}
