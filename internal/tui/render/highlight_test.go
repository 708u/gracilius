package render

import (
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/muesli/termenv"
)

func TestHighlightFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		filename string
		source   string
		verify   func(t *testing.T, source string, result []HighlightedLine)
	}{
		{
			name:     "Go",
			filename: "example.go",
			source:   "package main\n\nfunc main() {\n\tx := 42\n}\n",
			verify: func(t *testing.T, source string, result []HighlightedLine) {
				t.Helper()
				if result == nil {
					t.Fatal("HighlightFile returned nil for Go source")
				}
				for i, hl := range result {
					sourceLine := strings.Split(source, "\n")[i]
					if sourceLine != "" && len(hl.Runs) == 0 {
						t.Errorf("line %d (%q): expected non-empty runs", i, sourceLine)
					}
				}
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
				if !strings.Contains(funcLine.Rendered, "func") {
					t.Error("expected 'func' in rendered output")
				}
			},
		},
		{
			name:     "Unknown",
			filename: "data.xyzunknown",
			source:   "line one\nline two\nline three",
			verify: func(t *testing.T, source string, result []HighlightedLine) {
				t.Helper()
				if result == nil {
					t.Fatal("HighlightFile returned nil for unknown extension")
				}
				lines := strings.Split(source, "\n")
				if len(result) != len(lines) {
					t.Errorf("expected %d lines, got %d", len(lines), len(result))
				}
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
			},
		},
		{
			name:     "UnknownExtension",
			filename: "file.unknownext12345",
			source:   "some random content\nline two",
			verify: func(t *testing.T, _ string, result []HighlightedLine) {
				t.Helper()
				if result == nil {
					t.Error("expected non-nil result for unknown extension (fallback lexer)")
				}
			},
		},
		{
			name:     "MultilineToken",
			filename: "main.go",
			source:   "package main\n\nvar s = `line1\nline2\nline3`\n",
			verify: func(t *testing.T, source string, result []HighlightedLine) {
				t.Helper()
				if result == nil {
					t.Fatal("HighlightFile returned nil")
				}
				lines := strings.Split(source, "\n")
				if len(result) != len(lines) {
					t.Errorf("expected %d lines, got %d", len(lines), len(result))
				}
				if len(result) > 3 && len(result[3].Runs) == 0 {
					t.Error("expected non-empty runs for middle of multi-line token")
				}
			},
		},
		{
			name:     "ValidGoSource",
			filename: "main.go",
			source:   "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n",
			verify: func(t *testing.T, source string, result []HighlightedLine) {
				t.Helper()
				if result == nil {
					t.Fatal("HighlightFile returned nil for valid Go source")
				}
				lines := strings.Split(source, "\n")
				if len(result) != len(lines) {
					t.Errorf("expected %d lines, got %d", len(lines), len(result))
				}
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
				if !strings.Contains(result[0].Rendered, "\033[") {
					t.Error("expected pre-rendered string to contain ANSI codes")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := HighlightFile(tt.filename, tt.source, Dark)
			tt.verify(t, tt.source, result)
		})
	}
}

func TestNewHighlightedLine(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	runs := []StyledRun{
		{Text: "hello world", ANSI: "\033[38;5;148m"},
	}

	var sb strings.Builder
	selBgSeq := Dark.SelectionBgSeq()
	RenderStyledLineWithSelection(&sb, runs, 2, 7, selBgSeq) // select "llo w"
	output := sb.String()

	if !strings.Contains(output, selBgSeq) {
		t.Error("expected selection background color in output")
	}
	if !strings.Contains(output, "\033[38;5;148m") {
		t.Error("expected foreground ANSI to be preserved in selection")
	}
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
	t.Parallel()

	style := styles.Get("github-dark")

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

	ansiBold := ResolveANSI(style, chroma.GenericStrong)
	if ansiBold != "" && !strings.Contains(ansiBold, termenv.BoldSeq) {
		t.Error("expected bold sequence for GenericStrong")
	}

	emptyStyle := styles.Register(chroma.MustNewStyle("_test_empty_render", chroma.StyleEntries{}))
	ansiEmpty := ResolveANSI(emptyStyle, chroma.Keyword)
	if ansiEmpty != "" {
		t.Errorf("expected empty ANSI for empty style, got %q", ansiEmpty)
	}
}

func TestRenderStyledLineWithHighlights(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		runs       []StyledRun
		highlights []HighlightRange
		verify     func(t *testing.T, output string)
	}{
		{
			name: "MultipleRanges",
			runs: []StyledRun{
				{Text: "abcdefghij", ANSI: "\033[31m"},
			},
			highlights: []HighlightRange{
				{Start: 1, End: 4, BgSeq: "\033[42m"},
				{Start: 6, End: 9, BgSeq: "\033[44m"},
			},
			verify: func(t *testing.T, output string) {
				t.Helper()
				if !strings.Contains(output, "\033[42m") {
					t.Error("expected first highlight background in output")
				}
				if !strings.Contains(output, "\033[44m") {
					t.Error("expected second highlight background in output")
				}
				if !strings.Contains(output, "a") {
					t.Error("expected 'a' (before first highlight) in output")
				}
				if !strings.Contains(output, "j") {
					t.Error("expected 'j' (after second highlight) in output")
				}
			},
		},
		{
			name: "Overlapping",
			runs: []StyledRun{
				{Text: "abcdef", ANSI: ""},
			},
			highlights: []HighlightRange{
				{Start: 1, End: 5, BgSeq: "\033[42m"},
				{Start: 3, End: 6, BgSeq: "\033[44m"},
			},
			verify: func(t *testing.T, output string) {
				t.Helper()
				if !strings.Contains(output, "\033[44m") {
					t.Error("expected later highlight to be present")
				}
			},
		},
		{
			name:       "EmptyRuns",
			runs:       nil,
			highlights: []HighlightRange{{Start: 0, End: 5, BgSeq: "\033[42m"}},
			verify: func(t *testing.T, output string) {
				t.Helper()
				if output != "" {
					t.Errorf("expected empty output for nil runs, got %q", output)
				}
			},
		},
		{
			name: "NoHighlights",
			runs: []StyledRun{
				{Text: "hello", ANSI: "\033[31m"},
			},
			highlights: nil,
			verify: func(t *testing.T, output string) {
				t.Helper()
				if !strings.Contains(output, "\033[31m") {
					t.Error("expected foreground ANSI in output")
				}
				if !strings.Contains(output, "hello") {
					t.Error("expected 'hello' in output")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var sb strings.Builder
			RenderStyledLineWithHighlights(&sb, tt.runs, tt.highlights)
			tt.verify(t, sb.String())
		})
	}
}
