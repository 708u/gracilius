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
