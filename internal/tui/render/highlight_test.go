package render

import (
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/muesli/termenv"
)

func TestHighlightFile(t *testing.T) {
	source := `package main

import "fmt"

func main() {
	fmt.Println("hello")
}
`
	result := HighlightFile("main.go", source, Dark)
	if result == nil {
		t.Fatal("HighlightFile returned nil for valid Go source")
	}

	// Source has 7 lines (6 lines + trailing newline)
	lines := strings.Split(source, "\n")
	if len(result) != len(lines) {
		t.Errorf("expected %d lines, got %d", len(lines), len(result))
	}

	// First line should have at least one run with ANSI (keyword "package")
	if len(result[0].Runs) == 0 {
		t.Error("expected non-empty first line")
	}

	hasANSI := false
	for _, run := range result[0].Runs {
		if run.ANSI != "" {
			hasANSI = true
			break
		}
	}
	if !hasANSI {
		t.Error("expected at least one run with ANSI code on the first line")
	}

	// Pre-rendered string should contain ANSI codes
	if !strings.Contains(result[0].Rendered, "\033[") {
		t.Error("expected pre-rendered string to contain ANSI codes")
	}
}

func TestNewHighlightedLine(t *testing.T) {
	runs := []StyledRun{
		{Text: "func", ANSI: "\033[38;5;148m"},
		{Text: " main", ANSI: ""},
		{Text: "()", ANSI: "\033[38;5;197m"},
	}

	hl := NewHighlightedLine(runs)

	if !strings.Contains(hl.Rendered, "\033[38;5;148m") {
		t.Error("expected SGR code for 'func' keyword")
	}
	if !strings.Contains(hl.Rendered, "\033[0m") {
		t.Error("expected reset code")
	}
	if !strings.Contains(hl.Rendered, "func") {
		t.Error("expected text 'func' in output")
	}
	if !strings.Contains(hl.Rendered, " main") {
		t.Error("expected text ' main' in output")
	}
	if len(hl.Runs) != 3 {
		t.Errorf("expected 3 runs, got %d", len(hl.Runs))
	}
}

func TestRenderStyledLineWithSelection(t *testing.T) {
	runs := []StyledRun{
		{Text: "hello world", ANSI: "\033[38;5;148m"},
	}

	var sb strings.Builder
	selBgSeq := Dark.SelectionBgSeq()
	RenderStyledLineWithSelection(&sb, runs, 2, 7, selBgSeq) // select "llo w"
	output := sb.String()

	// Selection should use the theme's SelectionBg, not inverse video
	if !strings.Contains(output, selBgSeq) {
		t.Error("expected selection background color in output")
	}

	// The run's foreground ANSI should be preserved within the selection
	if !strings.Contains(output, "\033[38;5;148m") {
		t.Error("expected foreground ANSI to be preserved in selection")
	}

	// Check that selection contains the right text
	_, afterSelBg, _ := strings.Cut(output, selBgSeq)
	before, _, ok := strings.Cut(afterSelBg, "\033[0m")
	if !ok {
		t.Fatal("expected reset after selection background")
	}
	selected := before
	if selected != "llo w" {
		t.Errorf("expected selected text 'llo w', got %q", selected)
	}
}

func TestResolveANSI(t *testing.T) {
	style := styles.Get("github-dark")

	// Keyword should have color
	ansiCode := ResolveANSI(style, chroma.Keyword)
	if ansiCode == "" {
		t.Error("expected non-empty ANSI for Keyword token")
	}
	if !strings.Contains(ansiCode, termenv.CSI) {
		t.Error("expected CSI prefix in ANSI string")
	}
	if !strings.HasSuffix(ansiCode, "m") {
		t.Error("expected 'm' suffix in ANSI string")
	}

	// Bold token should contain bold sequence
	ansiBold := ResolveANSI(style, chroma.GenericStrong)
	if ansiBold != "" && !strings.Contains(ansiBold, termenv.BoldSeq) {
		t.Error("expected bold sequence for GenericStrong")
	}

	// Empty style should return empty for any token
	emptyStyle := styles.Register(chroma.MustNewStyle("_test_empty_render", chroma.StyleEntries{}))
	ansiEmpty := ResolveANSI(emptyStyle, chroma.Keyword)
	if ansiEmpty != "" {
		t.Errorf("expected empty ANSI for empty style, got %q", ansiEmpty)
	}
}

func TestHighlightFileUnknownExtension(t *testing.T) {
	source := "some random content\nline two"
	result := HighlightFile("file.unknownext12345", source, Dark)
	if result == nil {
		t.Error("expected non-nil result for unknown extension (fallback lexer)")
	}
}

func TestHighlightFileMultilineToken(t *testing.T) {
	// Go raw string literal spans multiple lines
	source := "package main\n\nvar s = `line1\nline2\nline3`\n"
	result := HighlightFile("main.go", source, Dark)
	if result == nil {
		t.Fatal("HighlightFile returned nil")
	}

	// Source has 5 newlines so 6 lines
	lines := strings.Split(source, "\n")
	if len(result) != len(lines) {
		t.Errorf("expected %d lines, got %d", len(lines), len(result))
	}

	// Lines containing multi-line string parts should have runs
	// line index 2: 'var s = `line1'
	// line index 3: 'line2'
	// line index 4: 'line3`'
	if len(result) > 3 && len(result[3].Runs) == 0 {
		t.Error("expected non-empty runs for middle of multi-line token")
	}
}

func TestHighlightFile_Go(t *testing.T) {
	source := "package main\n\nfunc main() {\n\tx := 42\n}\n"
	result := HighlightFile("example.go", source, Dark)
	if result == nil {
		t.Fatal("HighlightFile returned nil for Go source")
	}

	// Every non-empty source line should produce styled runs
	for i, hl := range result {
		sourceLine := strings.Split(source, "\n")[i]
		if sourceLine != "" && len(hl.Runs) == 0 {
			t.Errorf("line %d (%q): expected non-empty runs", i, sourceLine)
		}
	}

	// Keyword "func" line (index 2) should have ANSI
	funcLine := result[2]
	hasKeywordANSI := false
	for _, run := range funcLine.Runs {
		if strings.Contains(run.Text, "func") && run.ANSI != "" {
			hasKeywordANSI = true
			break
		}
	}
	if !hasKeywordANSI {
		t.Error("expected 'func' keyword to have ANSI styling")
	}

	// Rendered output should contain the text
	if !strings.Contains(funcLine.Rendered, "func") {
		t.Error("expected 'func' in rendered output")
	}
}

func TestHighlightFile_Unknown(t *testing.T) {
	source := "line one\nline two\nline three"
	result := HighlightFile("data.xyzunknown", source, Dark)
	if result == nil {
		t.Fatal("HighlightFile returned nil for unknown extension")
	}

	// Should still produce correct number of lines
	lines := strings.Split(source, "\n")
	if len(result) != len(lines) {
		t.Errorf("expected %d lines, got %d", len(lines), len(result))
	}

	// Each line should have at least one run with the text content
	for i, hl := range result {
		combined := ""
		for _, run := range hl.Runs {
			combined += run.Text
		}
		if combined != lines[i] {
			t.Errorf("line %d: combined run text %q != source line %q",
				i, combined, lines[i])
		}
	}
}

func TestRenderStyledLineWithHighlights_MultipleRanges(t *testing.T) {
	runs := []StyledRun{
		{Text: "abcdefghij", ANSI: "\033[31m"},
	}
	bg1 := "\033[42m"
	bg2 := "\033[44m"
	highlights := []HighlightRange{
		{Start: 1, End: 4, BgSeq: bg1}, // "bcd"
		{Start: 6, End: 9, BgSeq: bg2}, // "ghi"
	}

	var sb strings.Builder
	RenderStyledLineWithHighlights(&sb, runs, highlights)
	output := sb.String()

	if !strings.Contains(output, bg1) {
		t.Error("expected first highlight background in output")
	}
	if !strings.Contains(output, bg2) {
		t.Error("expected second highlight background in output")
	}
	// Non-highlighted parts should still render
	if !strings.Contains(output, "a") {
		t.Error("expected 'a' (before first highlight) in output")
	}
	if !strings.Contains(output, "j") {
		t.Error("expected 'j' (after second highlight) in output")
	}
}

func TestRenderStyledLineWithHighlights_Overlapping(t *testing.T) {
	runs := []StyledRun{
		{Text: "abcdef", ANSI: ""},
	}
	bg1 := "\033[42m"
	bg2 := "\033[44m"
	// Overlapping ranges: later wins
	highlights := []HighlightRange{
		{Start: 1, End: 5, BgSeq: bg1},
		{Start: 3, End: 6, BgSeq: bg2},
	}

	var sb strings.Builder
	RenderStyledLineWithHighlights(&sb, runs, highlights)
	output := sb.String()

	// bg2 should be present (overrides bg1 in the overlap)
	if !strings.Contains(output, bg2) {
		t.Error("expected later highlight to be present")
	}
}

func TestRenderStyledLineWithHighlights_EmptyRuns(t *testing.T) {
	var sb strings.Builder
	RenderStyledLineWithHighlights(&sb, nil, []HighlightRange{
		{Start: 0, End: 5, BgSeq: "\033[42m"},
	})
	output := sb.String()

	if output != "" {
		t.Errorf("expected empty output for nil runs, got %q", output)
	}
}

func TestRenderStyledLineWithHighlights_NoHighlights(t *testing.T) {
	runs := []StyledRun{
		{Text: "hello", ANSI: "\033[31m"},
	}

	var sb strings.Builder
	RenderStyledLineWithHighlights(&sb, runs, nil)
	output := sb.String()

	// Should render normally with ANSI codes
	if !strings.Contains(output, "\033[31m") {
		t.Error("expected foreground ANSI in output")
	}
	if !strings.Contains(output, "hello") {
		t.Error("expected 'hello' in output")
	}
}

func TestClampHighlightsToSegment(t *testing.T) {
	tests := []struct {
		name       string
		highlights []HighlightRange
		wrapOff    int
		segLen     int
		wantLen    int
		wantFirst  *HighlightRange // nil means no results expected
	}{
		{
			name: "fully within segment",
			highlights: []HighlightRange{
				{Start: 2, End: 5, BgSeq: "bg1"},
			},
			wrapOff:   0,
			segLen:    10,
			wantLen:   1,
			wantFirst: &HighlightRange{Start: 2, End: 5, BgSeq: "bg1"},
		},
		{
			name: "clamp to segment start",
			highlights: []HighlightRange{
				{Start: 3, End: 8, BgSeq: "bg1"},
			},
			wrapOff:   5,
			segLen:    10,
			wantLen:   1,
			wantFirst: &HighlightRange{Start: 0, End: 3, BgSeq: "bg1"},
		},
		{
			name: "clamp to segment end",
			highlights: []HighlightRange{
				{Start: 2, End: 15, BgSeq: "bg1"},
			},
			wrapOff:   0,
			segLen:    10,
			wantLen:   1,
			wantFirst: &HighlightRange{Start: 2, End: 10, BgSeq: "bg1"},
		},
		{
			name: "entirely before segment",
			highlights: []HighlightRange{
				{Start: 0, End: 3, BgSeq: "bg1"},
			},
			wrapOff: 5,
			segLen:  10,
			wantLen: 0,
		},
		{
			name: "entirely after segment",
			highlights: []HighlightRange{
				{Start: 20, End: 25, BgSeq: "bg1"},
			},
			wrapOff: 0,
			segLen:  10,
			wantLen: 0,
		},
		{
			name: "multiple highlights",
			highlights: []HighlightRange{
				{Start: 5, End: 8, BgSeq: "bg1"},
				{Start: 12, End: 18, BgSeq: "bg2"},
			},
			wrapOff: 10,
			segLen:  10,
			wantLen: 1, // only the second overlaps
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := ClampHighlightsToSegment(tc.highlights, tc.wrapOff, tc.segLen)
			if len(result) != tc.wantLen {
				t.Fatalf("expected %d results, got %d: %+v",
					tc.wantLen, len(result), result)
			}
			if tc.wantFirst != nil && len(result) > 0 {
				got := result[0]
				if got.Start != tc.wantFirst.Start ||
					got.End != tc.wantFirst.End ||
					got.BgSeq != tc.wantFirst.BgSeq {
					t.Errorf("first result = %+v, want %+v", got, *tc.wantFirst)
				}
			}
		})
	}
}
