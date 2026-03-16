package render

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

func TestPadRight_ASCII(t *testing.T) {
	result := PadRight("abc", 10)
	// lipgloss pads to display width 10 using spaces
	if len(result) < 10 {
		t.Errorf("expected padded string length >= 10, got %d: %q", len(result), result)
	}
	if !strings.HasPrefix(result, "abc") {
		t.Errorf("expected prefix 'abc', got %q", result)
	}
}

func TestPadRight_CJK(t *testing.T) {
	// Each CJK char is 2 columns wide; 3 chars = 6 columns
	result := PadRight("\u4e16\u754c\u4eba", 10)
	if !strings.Contains(result, "\u4e16\u754c\u4eba") {
		t.Errorf("expected CJK text preserved, got %q", result)
	}
	// Should have 4 padding spaces (10 - 6 = 4)
	if !strings.Contains(result, "    ") {
		t.Errorf("expected 4 spaces padding for CJK at width 10, got %q", result)
	}
}

func TestPadRight_AlreadyWide(t *testing.T) {
	// Exactly at width: should contain the original text with no extra padding
	result := PadRight("abcdefghij", 10)
	if !strings.Contains(result, "abcdefghij") {
		t.Errorf("expected original text preserved, got %q", result)
	}

	// Over width: lipgloss wraps to fit, so the text may be reformatted.
	// Just verify the function does not panic and produces output.
	result2 := PadRight("abcdefghijkl", 10)
	if result2 == "" {
		t.Error("expected non-empty result for over-width string")
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
		got := ExpandTabs(tc.input)
		if got != tc.want {
			t.Errorf("ExpandTabs(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestRuneWidth_ASCII(t *testing.T) {
	for _, r := range "abcABC123!@#" {
		w := RuneWidth(r)
		if w != 1 {
			t.Errorf("RuneWidth(%q) = %d, want 1", r, w)
		}
	}
}

func TestRuneWidth_CJK(t *testing.T) {
	cjk := "\u4e16\u754c\u4eba\u3042\uff21"
	for _, r := range cjk {
		w := RuneWidth(r)
		if w != 2 {
			t.Errorf("RuneWidth(%q) = %d, want 2", r, w)
		}
	}
}

func TestRuneWidth_Tab(t *testing.T) {
	w := RuneWidth('\t')
	if w != 4 {
		t.Errorf("RuneWidth(tab) = %d, want 4", w)
	}
}

func TestWrapBreakpoints_NoWrap(t *testing.T) {
	bp := WrapBreakpoints("short", 80)
	if bp != nil {
		t.Errorf("expected nil breakpoints for short line, got %v", bp)
	}
}

func TestWrapBreakpoints_BasicWrap(t *testing.T) {
	// 10 chars, width 5 => break at index 5
	line := "abcdefghij"
	bp := WrapBreakpoints(line, 5)
	if len(bp) != 1 {
		t.Fatalf("expected 1 breakpoint, got %d: %v", len(bp), bp)
	}
	if bp[0] != 5 {
		t.Errorf("expected breakpoint at 5, got %d", bp[0])
	}
}

func TestWrapBreakpoints_CJK(t *testing.T) {
	// Each CJK char is 2 columns; 5 CJK chars = 10 columns
	line := "\u4e16\u754c\u4eba\u985e\u5b9d"
	bp := WrapBreakpoints(line, 6)
	// Width 6 fits 3 CJK chars (6 cols), break at index 3
	if len(bp) < 1 {
		t.Fatalf("expected at least 1 breakpoint for CJK wrapping, got %v", bp)
	}
	if bp[0] != 3 {
		t.Errorf("expected first breakpoint at 3 (3 CJK chars = 6 cols), got %d", bp[0])
	}
}

func TestWrapBreakpoints_ZeroWidth(t *testing.T) {
	bp := WrapBreakpoints("anything", 0)
	if bp != nil {
		t.Errorf("expected nil breakpoints for zero width, got %v", bp)
	}
}

func TestWrapBreakpoints_EmptyLine(t *testing.T) {
	bp := WrapBreakpoints("", 10)
	if bp != nil {
		t.Errorf("expected nil breakpoints for empty line, got %v", bp)
	}
}

func TestDisplayWidthRange(t *testing.T) {
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
		got := DisplayWidthRange(tc.line, tc.from, tc.to)
		if got != tc.want {
			t.Errorf("DisplayWidthRange(%q, %d, %d) = %d, want %d",
				tc.line, tc.from, tc.to, got, tc.want)
		}
	}
}

func TestCountWraps_NoWrap(t *testing.T) {
	count := CountWraps("short", 80)
	if count != 1 {
		t.Errorf("expected 1 for short line, got %d", count)
	}
}

func TestCountWraps_WithWrap(t *testing.T) {
	// 10 chars at width 5 => 2 visual rows
	count := CountWraps("abcdefghij", 5)
	if count != 2 {
		t.Errorf("expected 2 for 10-char line at width 5, got %d", count)
	}

	// 15 chars at width 5 => 3 visual rows
	count = CountWraps("abcdefghijklmno", 5)
	if count != 3 {
		t.Errorf("expected 3 for 15-char line at width 5, got %d", count)
	}
}

func TestCountWraps_ZeroWidth(t *testing.T) {
	count := CountWraps("anything", 0)
	if count != 1 {
		t.Errorf("expected 1 for zero width, got %d", count)
	}
}

func TestCountWraps_CJK(t *testing.T) {
	// 5 CJK chars = 10 columns; width 6 => 2 visual rows
	count := CountWraps("\u4e16\u754c\u4eba\u985e\u5b9d", 6)
	if count != 2 {
		t.Errorf("expected 2 for CJK at width 6, got %d", count)
	}
}

func TestSplitRunsAtBreakpoints(t *testing.T) {
	runs := []StyledRun{
		{Text: "abcdefghij", ANSI: "\033[31m"},
	}
	bp := []int{5}
	segments := SplitRunsAtBreakpoints(runs, bp)

	if len(segments) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(segments))
	}

	// First segment: "abcde"
	if len(segments[0]) != 1 {
		t.Fatalf("expected 1 run in segment 0, got %d", len(segments[0]))
	}
	if segments[0][0].Text != "abcde" {
		t.Errorf("segment 0 text = %q, want 'abcde'", segments[0][0].Text)
	}
	if segments[0][0].ANSI != "\033[31m" {
		t.Errorf("segment 0 ANSI = %q, want '\\033[31m'", segments[0][0].ANSI)
	}

	// Second segment: "fghij"
	if len(segments[1]) != 1 {
		t.Fatalf("expected 1 run in segment 1, got %d", len(segments[1]))
	}
	if segments[1][0].Text != "fghij" {
		t.Errorf("segment 1 text = %q, want 'fghij'", segments[1][0].Text)
	}
}

func TestSplitRunsAtBreakpoints_MultipleRuns(t *testing.T) {
	runs := []StyledRun{
		{Text: "abc", ANSI: "\033[31m"},
		{Text: "defgh", ANSI: "\033[32m"},
	}
	// Break at position 5, which falls in the second run ("defgh" starts at pos 3)
	bp := []int{5}
	segments := SplitRunsAtBreakpoints(runs, bp)

	if len(segments) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(segments))
	}

	// First segment should have "abc" + "de"
	seg0Text := ""
	for _, r := range segments[0] {
		seg0Text += r.Text
	}
	if seg0Text != "abcde" {
		t.Errorf("segment 0 combined text = %q, want 'abcde'", seg0Text)
	}

	// Second segment should have "fgh"
	seg1Text := ""
	for _, r := range segments[1] {
		seg1Text += r.Text
	}
	if seg1Text != "fgh" {
		t.Errorf("segment 1 combined text = %q, want 'fgh'", seg1Text)
	}
}

func TestSplitRunsAtBreakpoints_NilBreakpoints(t *testing.T) {
	runs := []StyledRun{
		{Text: "hello", ANSI: ""},
	}
	segments := SplitRunsAtBreakpoints(runs, nil)
	if len(segments) != 1 {
		t.Fatalf("expected 1 segment with nil breakpoints, got %d", len(segments))
	}
	if segments[0][0].Text != "hello" {
		t.Errorf("expected 'hello', got %q", segments[0][0].Text)
	}
}

// TestDiag_LipglossResetFormat inspects the exact ANSI sequences
// lipgloss v2 emits, so we know what PadRightWithBg must handle.
func TestDiag_LipglossResetFormat(t *testing.T) {
	styled := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#D19A66")).Render("M")
	t.Logf("lipgloss output bytes: %q", styled)
	t.Logf("visual width: %d", ansi.StringWidth(styled))

	reset0m := termenv.CSI + "0m"
	resetM := termenv.CSI + "m"
	t.Logf("contains \\033[0m: %v", strings.Contains(styled, reset0m))
	t.Logf("contains \\033[m:  %v", strings.Contains(styled, resetM))

	// Dump each byte for inspection.
	var buf strings.Builder
	for i, b := range []byte(styled) {
		if i > 0 {
			buf.WriteByte(' ')
		}
		_, _ = fmt.Fprintf(&buf, "%02x", b)
	}
	t.Logf("hex: %s", buf.String())
}

func TestDiag_PadRightWithBg_VisualWidth(t *testing.T) {
	bgSeq := "\033[48;2;80;80;80m"
	width := 30

	// Build content exactly like the git panel does.
	styledM := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#D19A66")).Render("M")
	line := "      " + styledM + " " + "theme.go"

	truncated := ansi.Truncate(line, width, "...")

	padRight := PadRight(truncated, width)
	withBg := PadRightWithBg(truncated, width, bgSeq)

	padRightW := ansi.StringWidth(padRight)
	withBgW := ansi.StringWidth(withBg)

	t.Logf("line:      %q (vis=%d)", line, ansi.StringWidth(line))
	t.Logf("truncated: %q (vis=%d)", truncated, ansi.StringWidth(truncated))
	t.Logf("PadRight:      vis=%d, bytes=%q", padRightW, padRight)
	t.Logf("PadRightWithBg: vis=%d, bytes=%q", withBgW, withBg)

	if padRightW != width {
		t.Errorf("PadRight visual width = %d, want %d", padRightW, width)
	}
	if withBgW != width {
		t.Errorf("PadRightWithBg visual width = %d, want %d", withBgW, width)
	}
}
