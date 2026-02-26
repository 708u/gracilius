package tui

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
	result := highlightFile("main.go", source)
	if result == nil {
		t.Fatal("highlightFile returned nil for valid Go source")
	}

	// Source has 7 lines (6 lines + trailing newline)
	lines := strings.Split(source, "\n")
	if len(result) != len(lines) {
		t.Errorf("expected %d lines, got %d", len(lines), len(result))
	}

	// First line should have at least one run with ANSI (keyword "package")
	if len(result[0].runs) == 0 {
		t.Error("expected non-empty first line")
	}

	hasANSI := false
	for _, run := range result[0].runs {
		if run.ANSI != "" {
			hasANSI = true
			break
		}
	}
	if !hasANSI {
		t.Error("expected at least one run with ANSI code on the first line")
	}

	// Pre-rendered string should contain ANSI codes
	if !strings.Contains(result[0].rendered, "\033[") {
		t.Error("expected pre-rendered string to contain ANSI codes")
	}
}

func TestNewHighlightedLine(t *testing.T) {
	runs := []styledRun{
		{Text: "func", ANSI: "\033[38;5;148m"},
		{Text: " main", ANSI: ""},
		{Text: "()", ANSI: "\033[38;5;197m"},
	}

	hl := newHighlightedLine(runs)

	if !strings.Contains(hl.rendered, "\033[38;5;148m") {
		t.Error("expected SGR code for 'func' keyword")
	}
	if !strings.Contains(hl.rendered, "\033[0m") {
		t.Error("expected reset code")
	}
	if !strings.Contains(hl.rendered, "func") {
		t.Error("expected text 'func' in output")
	}
	if !strings.Contains(hl.rendered, " main") {
		t.Error("expected text ' main' in output")
	}
	if len(hl.runs) != 3 {
		t.Errorf("expected 3 runs, got %d", len(hl.runs))
	}
}

func TestRenderStyledLineWithCursor(t *testing.T) {
	runs := []styledRun{
		{Text: "hello", ANSI: "\033[38;5;148m"},
	}

	var sb strings.Builder
	renderStyledLineWithCursor(&sb, runs, 2) // cursor on 'l'
	output := sb.String()

	// Check that the cursor character is exactly "l"
	invIdx := strings.Index(output, "\033[7m")
	if invIdx < 0 {
		t.Fatal("expected inverse video for cursor")
	}
	resetIdx := strings.Index(output[invIdx:], "\033[0m")
	if resetIdx < 0 {
		t.Fatal("expected reset after inverse")
	}
	cursorText := output[invIdx+len("\033[7m") : invIdx+resetIdx]
	if cursorText != "l" {
		t.Errorf("expected cursor on 'l', got %q", cursorText)
	}

	// Cursor past end of line
	sb.Reset()
	renderStyledLineWithCursor(&sb, runs, 10)
	output = sb.String()

	if !strings.Contains(output, "\033[7m \033[0m") {
		t.Error("expected inverse space for EOL cursor")
	}
}

func TestRenderStyledLineWithSelection(t *testing.T) {
	runs := []styledRun{
		{Text: "hello world", ANSI: "\033[38;5;148m"},
	}

	var sb strings.Builder
	renderStyledLineWithSelection(&sb, runs, 2, 7) // select "llo w"
	output := sb.String()

	// Selection should use the active theme's selectionBg, not inverse video
	if !strings.Contains(output, activeTheme.selectionBgSeq()) {
		t.Error("expected selection background color in output")
	}

	// The run's foreground ANSI should be preserved within the selection
	if !strings.Contains(output, "\033[38;5;148m") {
		t.Error("expected foreground ANSI to be preserved in selection")
	}

	// Check that selection contains the right text
	selBgIdx := strings.Index(output, activeTheme.selectionBgSeq())
	afterSelBg := output[selBgIdx+len(activeTheme.selectionBgSeq()):]
	resetIdx := strings.Index(afterSelBg, "\033[0m")
	if resetIdx < 0 {
		t.Fatal("expected reset after selection background")
	}
	selected := afterSelBg[:resetIdx]
	if selected != "llo w" {
		t.Errorf("expected selected text 'llo w', got %q", selected)
	}
}

func TestResolveANSI(t *testing.T) {
	style := styles.Get("github-dark")

	// Keyword should have color
	ansi := resolveANSI(style, chroma.Keyword)
	if ansi == "" {
		t.Error("expected non-empty ANSI for Keyword token")
	}
	if !strings.Contains(ansi, termenv.CSI) {
		t.Error("expected CSI prefix in ANSI string")
	}
	if !strings.HasSuffix(ansi, "m") {
		t.Error("expected 'm' suffix in ANSI string")
	}

	// Bold token should contain bold sequence
	ansiBold := resolveANSI(style, chroma.GenericStrong)
	if ansiBold != "" && !strings.Contains(ansiBold, termenv.BoldSeq) {
		t.Error("expected bold sequence for GenericStrong")
	}

	// Empty style should return empty for any token
	emptyStyle := styles.Register(chroma.MustNewStyle("_test_empty", chroma.StyleEntries{}))
	ansiEmpty := resolveANSI(emptyStyle, chroma.Keyword)
	if ansiEmpty != "" {
		t.Errorf("expected empty ANSI for empty style, got %q", ansiEmpty)
	}
}

func TestSelRange(t *testing.T) {
	tests := []struct {
		name      string
		line      int
		startLine int
		endLine   int
		startChar int
		endChar   int
		content   string
		wantSC    int
		wantEC    int
	}{
		{
			name: "first line of selection",
			line: 5, startLine: 5, endLine: 8,
			startChar: 3, endChar: 10,
			content: "hello world",
			wantSC:  3, wantEC: 11,
		},
		{
			name: "last line of selection",
			line: 8, startLine: 5, endLine: 8,
			startChar: 3, endChar: 10,
			content: "hello world",
			wantSC:  0, wantEC: 10,
		},
		{
			name: "middle line of selection",
			line: 6, startLine: 5, endLine: 8,
			startChar: 3, endChar: 10,
			content: "hello",
			wantSC:  0, wantEC: 5,
		},
		{
			name: "single line selection",
			line: 5, startLine: 5, endLine: 5,
			startChar: 2, endChar: 7,
			content: "hello world",
			wantSC:  2, wantEC: 7,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sc, ec := selRange(tt.line, tt.startLine, tt.endLine,
				tt.startChar, tt.endChar, tt.content)
			if sc != tt.wantSC || ec != tt.wantEC {
				t.Errorf("selRange() = (%d, %d), want (%d, %d)",
					sc, ec, tt.wantSC, tt.wantEC)
			}
		})
	}
}

func TestHighlightFileUnknownExtension(t *testing.T) {
	source := "some random content\nline two"
	result := highlightFile("file.unknownext12345", source)
	if result == nil {
		t.Error("expected non-nil result for unknown extension (fallback lexer)")
	}
}

func TestHighlightFileMultilineToken(t *testing.T) {
	// Go raw string literal spans multiple lines
	source := "package main\n\nvar s = `line1\nline2\nline3`\n"
	result := highlightFile("main.go", source)
	if result == nil {
		t.Fatal("highlightFile returned nil")
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
	if len(result) > 3 && len(result[3].runs) == 0 {
		t.Error("expected non-empty runs for middle of multi-line token")
	}
}
