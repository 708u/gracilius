package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// renderDiff is a test helper that pre-renders diff lines via viewport.
func renderDiff(data *diffData, theme themeConfig, width, height, offset int) []string {
	result := renderAllDiffLines(data, theme, width)
	// Simulate viewport slicing.
	start := min(offset, len(result.lines))
	end := min(start+height, len(result.lines))
	lines := make([]string, 0, height)
	lines = append(lines, result.lines[start:end]...)
	for len(lines) < height {
		lines = append(lines, padRight("", width))
	}
	return lines
}

func TestRenderSideBySide_LineCount(t *testing.T) {
	old := []string{"aaa", "bbb", "ccc"}
	new := []string{"aaa", "BBB", "ccc"}
	data := buildDiffData(old, new)

	heights := []int{5, 10, 20}
	for _, h := range heights {
		lines := renderDiff(data, darkTheme, 80, h, 0)
		if len(lines) != h {
			t.Errorf("height=%d: expected %d lines, got %d", h, h, len(lines))
		}
	}
}

func TestRenderSideBySide_EmptyDiff(t *testing.T) {
	data := buildDiffData(nil, nil)

	lines := renderDiff(data, darkTheme, 80, 10, 0)
	if len(lines) != 10 {
		t.Fatalf("expected 10 lines, got %d", len(lines))
	}
}

func TestRenderSideBySide_ColumnWidths(t *testing.T) {
	old := []string{"aaa", "bbb"}
	new := []string{"aaa", "BBB"}
	data := buildDiffData(old, new)

	width := 80
	lines := renderDiff(data, darkTheme, width, 5, 0)
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
	data := buildDiffData(old, new)

	lines := renderDiff(data, darkTheme, 80, 5, 0)
	for i, line := range lines[:len(data.rows)] {
		stripped := ansi.Strip(line)
		if !strings.Contains(stripped, "\u2502") {
			t.Errorf("line %d: missing separator", i)
		}
	}
}

func TestRenderSideBySide_ScrollOffset(t *testing.T) {
	old := []string{"aaa", "bbb", "ccc", "ddd", "eee"}
	new := []string{"aaa", "bbb", "ccc", "ddd", "eee"}
	data := buildDiffData(old, new)

	lines := renderDiff(data, darkTheme, 80, 5, 2)
	stripped := ansi.Strip(lines[0])
	if !strings.Contains(stripped, "3") {
		t.Errorf("expected line number 3 in first row, got %q", stripped)
	}
}

func TestRenderSideBySide_LineNumbers(t *testing.T) {
	old := []string{"aaa", "bbb", "ccc"}
	new := []string{"aaa", "BBB", "ccc"}
	data := buildDiffData(old, new)

	lines := renderDiff(data, darkTheme, 80, 5, 0)
	for i, row := range data.rows {
		stripped := ansi.Strip(lines[i])
		oldNum := strings.SplitN(stripped, "\u2502", 2)[0]
		if row.oldLineNum > 0 {
			numStr := fmt.Sprintf("%d", row.oldLineNum)
			if !strings.Contains(oldNum, numStr) {
				t.Errorf("row %d: expected old line number %s in %q", i, numStr, oldNum)
			}
		}
	}
}

func TestRenderSideBySide_FillerLine(t *testing.T) {
	data := buildDiffData(nil, []string{"aaa", "bbb"})

	lines := renderDiff(data, darkTheme, 80, 5, 0)
	for i := range data.rows {
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
	dark := diffColorsFor(darkTheme)
	light := diffColorsFor(lightTheme)

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
