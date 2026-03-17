package render

import (
	"strings"
	"testing"
)

func TestPadRight(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  string
		width  int
		verify func(t *testing.T, result string)
	}{
		{
			name:  "ASCII",
			input: "abc",
			width: 10,
			verify: func(t *testing.T, result string) {
				t.Helper()
				if len(result) < 10 {
					t.Errorf("expected padded string length >= 10, got %d: %q", len(result), result)
				}
				if !strings.HasPrefix(result, "abc") {
					t.Errorf("expected prefix 'abc', got %q", result)
				}
			},
		},
		{
			name:  "CJK",
			input: "\u4e16\u754c\u4eba",
			width: 10,
			verify: func(t *testing.T, result string) {
				t.Helper()
				if !strings.Contains(result, "\u4e16\u754c\u4eba") {
					t.Errorf("expected CJK text preserved, got %q", result)
				}
				if !strings.Contains(result, "    ") {
					t.Errorf("expected 4 spaces padding for CJK at width 10, got %q", result)
				}
			},
		},
		{
			name:  "AlreadyWide",
			input: "abcdefghij",
			width: 10,
			verify: func(t *testing.T, result string) {
				t.Helper()
				if !strings.Contains(result, "abcdefghij") {
					t.Errorf("expected original text preserved, got %q", result)
				}
				result2 := PadRight("abcdefghijkl", 10)
				if result2 == "" {
					t.Error("expected non-empty result for over-width string")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := PadRight(tt.input, tt.width)
			tt.verify(t, result)
		})
	}
}

func TestPadRightWithBg_PlainText(t *testing.T) {
	t.Parallel()
	bgSeq := "\033[48;2;80;80;80m"
	reset := "\033[0m"

	result := PadRightWithBg("hello", 10, bgSeq)

	// Must start with bgSeq and end with reset.
	if !strings.HasPrefix(result, bgSeq) {
		t.Errorf("expected prefix %q, got %q", bgSeq, result)
	}
	if !strings.HasSuffix(result, reset) {
		t.Errorf("expected suffix %q, got %q", reset, result)
	}
	// Content preserved.
	if !strings.Contains(result, "hello") {
		t.Error("content 'hello' not found in output")
	}
}

func TestPadRightWithBg_InternalResetReappliesBg(t *testing.T) {
	t.Parallel()
	bgSeq := "\033[48;2;80;80;80m"

	// SGR full-reset has multiple forms. All must be handled.
	tests := []struct {
		name  string
		reset string
	}{
		{"explicit_reset_0m", "\033[0m"},
		{"short_reset_m", "\033[m"},
		{"double_zero_00m", "\033[00m"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			content := "\033[33mM" + tc.reset + " theme.go"
			result := PadRightWithBg(content, 20, bgSeq)

			reapplied := tc.reset + bgSeq
			if !strings.Contains(result, reapplied) {
				t.Errorf(
					"background must be re-applied after internal %q;\n"+
						"expected %q in output,\n"+
						"got %q",
					tc.reset, reapplied, result,
				)
			}
		})
	}
}

func TestPadRightWithBg_MultipleInternalResets(t *testing.T) {
	t.Parallel()
	bgSeq := "\033[48;2;80;80;80m"

	// Content with mixed reset formats (both \033[0m and \033[m).
	content := "\033[33mA\033[0m mid \033[32mB\033[m end"
	result := PadRightWithBg(content, 30, bgSeq)

	reapply0m := "\033[0m" + bgSeq
	reapplyM := "\033[m" + bgSeq
	total := strings.Count(result, reapply0m) + strings.Count(result, reapplyM)
	if total < 2 {
		t.Errorf(
			"expected bgSeq re-applied at least 2 times after internal resets, got %d;\nresult: %q",
			total, result,
		)
	}
}

func TestPadRightWithBg_NoInternalReset(t *testing.T) {
	t.Parallel()
	bgSeq := "\033[48;2;80;80;80m"
	reset := "\033[0m"

	// Content with only foreground color, no full reset.
	content := "\033[33mhello"
	result := PadRightWithBg(content, 10, bgSeq)

	// Should start with bgSeq and end with reset,
	// with no unnecessary re-application.
	if !strings.HasPrefix(result, bgSeq) {
		t.Errorf("expected prefix %q, got %q", bgSeq, result)
	}
	if !strings.HasSuffix(result, reset) {
		t.Errorf("expected suffix %q, got %q", reset, result)
	}
}

func TestExpandTabs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{"\t", "    "},
		{"a\tb", "a    b"},
		{"\t\t", "        "},
		{"no tabs", "no tabs"},
		{"", ""},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			got := ExpandTabs(tc.input)
			if got != tc.want {
				t.Errorf("ExpandTabs(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestRuneWidth(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		runes string
		want  int
	}{
		{
			name:  "ASCII",
			runes: "abcABC123!@#",
			want:  1,
		},
		{
			name:  "CJK",
			runes: "\u4e16\u754c\u4eba\u3042\uff21",
			want:  2,
		},
		{
			name:  "Tab",
			runes: "\t",
			want:  4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			for _, r := range tt.runes {
				w := RuneWidth(r)
				if w != tt.want {
					t.Errorf("RuneWidth(%q) = %d, want %d", r, w, tt.want)
				}
			}
		})
	}
}

func TestWrapBreakpoints(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		line   string
		width  int
		verify func(t *testing.T, bp []int)
	}{
		{
			name:  "NoWrap",
			line:  "short",
			width: 80,
			verify: func(t *testing.T, bp []int) {
				t.Helper()
				if bp != nil {
					t.Errorf("expected nil breakpoints for short line, got %v", bp)
				}
			},
		},
		{
			name:  "BasicWrap",
			line:  "abcdefghij",
			width: 5,
			verify: func(t *testing.T, bp []int) {
				t.Helper()
				if len(bp) != 1 {
					t.Fatalf("expected 1 breakpoint, got %d: %v", len(bp), bp)
				}
				if bp[0] != 5 {
					t.Errorf("expected breakpoint at 5, got %d", bp[0])
				}
			},
		},
		{
			name:  "CJK",
			line:  "\u4e16\u754c\u4eba\u985e\u5b9d",
			width: 6,
			verify: func(t *testing.T, bp []int) {
				t.Helper()
				if len(bp) < 1 {
					t.Fatalf("expected at least 1 breakpoint for CJK wrapping, got %v", bp)
				}
				if bp[0] != 3 {
					t.Errorf("expected first breakpoint at 3 (3 CJK chars = 6 cols), got %d", bp[0])
				}
			},
		},
		{
			name:  "ZeroWidth",
			line:  "anything",
			width: 0,
			verify: func(t *testing.T, bp []int) {
				t.Helper()
				if bp != nil {
					t.Errorf("expected nil breakpoints for zero width, got %v", bp)
				}
			},
		},
		{
			name:  "EmptyLine",
			line:  "",
			width: 10,
			verify: func(t *testing.T, bp []int) {
				t.Helper()
				if bp != nil {
					t.Errorf("expected nil breakpoints for empty line, got %v", bp)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			bp := WrapBreakpoints(tt.line, tt.width)
			tt.verify(t, bp)
		})
	}
}

func TestDisplayWidthRange(t *testing.T) {
	t.Parallel()

	tests := []struct {
		line     string
		from, to int
		want     int
	}{
		{"abcdef", 0, 3, 3},
		{"abcdef", 2, 5, 3},
		{"abcdef", 0, 6, 6},
		{"\u4e16\u754c", 0, 2, 4}, // 2 CJK chars = 4 columns
		{"\u4e16\u754c", 0, 1, 2}, // 1 CJK char = 2 columns
		{"a\u4e16b", 0, 3, 4},     // a(1) + CJK(2) + b(1) = 4
		{"abc", 0, 0, 0},          // empty range
		{"abc", 1, 1, 0},          // empty range
		{"abc", 0, 10, 3},         // to beyond length is clamped
	}
	for _, tc := range tests {
		t.Run(tc.line, func(t *testing.T) {
			t.Parallel()
			got := DisplayWidthRange(tc.line, tc.from, tc.to)
			if got != tc.want {
				t.Errorf("DisplayWidthRange(%q, %d, %d) = %d, want %d",
					tc.line, tc.from, tc.to, got, tc.want)
			}
		})
	}
}

func TestCountWraps(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		line  string
		width int
		want  int
	}{
		{
			name:  "NoWrap",
			line:  "short",
			width: 80,
			want:  1,
		},
		{
			name:  "WithWrap_10at5",
			line:  "abcdefghij",
			width: 5,
			want:  2,
		},
		{
			name:  "WithWrap_15at5",
			line:  "abcdefghijklmno",
			width: 5,
			want:  3,
		},
		{
			name:  "ZeroWidth",
			line:  "anything",
			width: 0,
			want:  1,
		},
		{
			name:  "CJK",
			line:  "\u4e16\u754c\u4eba\u985e\u5b9d",
			width: 6,
			want:  2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			count := CountWraps(tt.line, tt.width)
			if count != tt.want {
				t.Errorf("CountWraps(%q, %d) = %d, want %d", tt.line, tt.width, count, tt.want)
			}
		})
	}
}

func TestSplitRunsAtBreakpoints(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		runs   []StyledRun
		bp     []int
		verify func(t *testing.T, segments [][]StyledRun)
	}{
		{
			name: "SingleRun",
			runs: []StyledRun{
				{Text: "abcdefghij", ANSI: "\033[31m"},
			},
			bp: []int{5},
			verify: func(t *testing.T, segments [][]StyledRun) {
				t.Helper()
				if len(segments) != 2 {
					t.Fatalf("expected 2 segments, got %d", len(segments))
				}
				if len(segments[0]) != 1 {
					t.Fatalf("expected 1 run in segment 0, got %d", len(segments[0]))
				}
				if segments[0][0].Text != "abcde" {
					t.Errorf("segment 0 text = %q, want 'abcde'", segments[0][0].Text)
				}
				if segments[0][0].ANSI != "\033[31m" {
					t.Errorf("segment 0 ANSI = %q, want '\\033[31m'", segments[0][0].ANSI)
				}
				if len(segments[1]) != 1 {
					t.Fatalf("expected 1 run in segment 1, got %d", len(segments[1]))
				}
				if segments[1][0].Text != "fghij" {
					t.Errorf("segment 1 text = %q, want 'fghij'", segments[1][0].Text)
				}
			},
		},
		{
			name: "MultipleRuns",
			runs: []StyledRun{
				{Text: "abc", ANSI: "\033[31m"},
				{Text: "defgh", ANSI: "\033[32m"},
			},
			bp: []int{5},
			verify: func(t *testing.T, segments [][]StyledRun) {
				t.Helper()
				if len(segments) != 2 {
					t.Fatalf("expected 2 segments, got %d", len(segments))
				}
				seg0Text := ""
				for _, r := range segments[0] {
					seg0Text += r.Text
				}
				if seg0Text != "abcde" {
					t.Errorf("segment 0 combined text = %q, want 'abcde'", seg0Text)
				}
				seg1Text := ""
				for _, r := range segments[1] {
					seg1Text += r.Text
				}
				if seg1Text != "fgh" {
					t.Errorf("segment 1 combined text = %q, want 'fgh'", seg1Text)
				}
			},
		},
		{
			name: "NilBreakpoints",
			runs: []StyledRun{
				{Text: "hello", ANSI: ""},
			},
			bp: nil,
			verify: func(t *testing.T, segments [][]StyledRun) {
				t.Helper()
				if len(segments) != 1 {
					t.Fatalf("expected 1 segment with nil breakpoints, got %d", len(segments))
				}
				if segments[0][0].Text != "hello" {
					t.Errorf("expected 'hello', got %q", segments[0][0].Text)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			segments := SplitRunsAtBreakpoints(tt.runs, tt.bp)
			tt.verify(t, segments)
		})
	}
}

func TestPadBetween(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		left  string
		right string
		width int
		want  string
	}{
		{
			name:  "Normal",
			left:  "ABC",
			right: "XY",
			width: 10,
			want:  "ABC     XY",
		},
		{
			name:  "ExactFit",
			left:  "ABC",
			right: "XY",
			width: 6,
			want:  "ABC XY",
		},
		{
			name:  "EmptyRight",
			left:  "hello",
			right: "",
			width: 10,
			want:  "hello     ",
		},
		{
			name:  "EmptyLeft",
			left:  "",
			right: "end",
			width: 10,
			want:  "       end",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := PadBetween(tt.left, tt.right, tt.width)
			if got != tt.want {
				t.Errorf("PadBetween(%q, %q, %d) = %q, want %q",
					tt.left, tt.right, tt.width, got, tt.want)
			}
		})
	}
}

func TestPadBetween_Overflow(t *testing.T) {
	t.Parallel()
	// Left is too long: should be truncated to make room for right.
	got := PadBetween("very long left text", "R", 10)
	if !strings.Contains(got, "R") {
		t.Errorf("expected right part 'R' preserved, got %q", got)
	}
	if len(got) < 10 {
		t.Errorf("expected at least width 10, got %d: %q", len(got), got)
	}
}

func TestClampHighlightsToSegment(t *testing.T) {
	t.Parallel()

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
			t.Parallel()
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
