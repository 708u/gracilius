package tui

import (
	"strings"
	"testing"
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

	if !strings.Contains(output, "\033[7m") {
		t.Error("expected inverse video for cursor")
	}
	if !strings.Contains(output, "l") {
		t.Error("expected cursor character 'l'")
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

	if !strings.Contains(output, "\033[7m") {
		t.Error("expected inverse video for selection")
	}

	// Check that selection contains the right text
	invIdx := strings.Index(output, "\033[7m")
	resetIdx := strings.Index(output[invIdx:], "\033[0m")
	selected := output[invIdx+len("\033[7m") : invIdx+resetIdx]
	if selected != "llo w" {
		t.Errorf("expected selected text 'llo w', got %q", selected)
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
