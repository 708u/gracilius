package render

import (
	"strings"
	"testing"
)

func TestWriteStyledText_WithANSI(t *testing.T) {
	var sb strings.Builder
	ansi := "\033[38;5;148m"
	WriteStyledText(&sb, ansi, "hello")
	got := sb.String()

	if !strings.HasPrefix(got, ansi) {
		t.Errorf("expected prefix %q, got %q", ansi, got)
	}
	if !strings.Contains(got, "hello") {
		t.Error("expected 'hello' in output")
	}
	if !strings.HasSuffix(got, AnsiReset) {
		t.Errorf("expected suffix %q, got %q", AnsiReset, got)
	}

	want := ansi + "hello" + AnsiReset
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestWriteStyledText_NoANSI(t *testing.T) {
	var sb strings.Builder
	WriteStyledText(&sb, "", "plain")
	got := sb.String()

	if got != "plain" {
		t.Errorf("expected 'plain', got %q", got)
	}
	if strings.Contains(got, "\033[") {
		t.Error("expected no ANSI codes for empty ansiCode")
	}
}

func TestWriteColoredChunk_WithFgAndBg(t *testing.T) {
	var sb strings.Builder
	fg := "\033[31m"
	bg := "\033[44m"
	WriteColoredChunk(&sb, fg, bg, "text")
	got := sb.String()

	if !strings.HasPrefix(got, fg) {
		t.Errorf("expected fg prefix %q, got %q", fg, got)
	}
	if !strings.Contains(got, bg) {
		t.Errorf("expected bg %q in output", bg)
	}
	if !strings.Contains(got, "text") {
		t.Error("expected 'text' in output")
	}
	if !strings.HasSuffix(got, AnsiReset) {
		t.Error("expected reset suffix")
	}
}

func TestWriteColoredChunk_FgOnly(t *testing.T) {
	var sb strings.Builder
	fg := "\033[31m"
	WriteColoredChunk(&sb, fg, "", "text")
	got := sb.String()

	if !strings.HasPrefix(got, fg) {
		t.Errorf("expected fg prefix, got %q", got)
	}
	if !strings.HasSuffix(got, AnsiReset) {
		t.Error("expected reset suffix")
	}
}

func TestWriteColoredChunk_NoStyle(t *testing.T) {
	var sb strings.Builder
	WriteColoredChunk(&sb, "", "", "text")
	got := sb.String()

	if got != "text" {
		t.Errorf("expected 'text', got %q", got)
	}
	if strings.Contains(got, "\033[") {
		t.Error("expected no ANSI codes")
	}
}

func TestWritePaddedText_WithBg(t *testing.T) {
	var sb strings.Builder
	bg := "\033[44m"
	WritePaddedText(&sb, "hi", 10, bg)
	got := sb.String()

	if !strings.HasPrefix(got, bg) {
		t.Errorf("expected bg prefix %q, got %q", bg, got)
	}
	if !strings.Contains(got, "hi") {
		t.Error("expected 'hi' in output")
	}
	if !strings.HasSuffix(got, AnsiReset) {
		t.Error("expected reset suffix")
	}
	// "hi" is 2 columns, so 8 spaces of padding
	if !strings.Contains(got, "        ") {
		t.Error("expected 8 spaces padding")
	}
}

func TestWritePaddedText_NoBg(t *testing.T) {
	var sb strings.Builder
	WritePaddedText(&sb, "hi", 10, "")
	got := sb.String()

	if !strings.Contains(got, "hi") {
		t.Error("expected 'hi' in output")
	}
	// No reset when no bg
	if strings.Contains(got, AnsiReset) {
		t.Error("expected no reset without bg")
	}
	// Still should be padded to width 10
	if !strings.Contains(got, "        ") {
		t.Error("expected 8 spaces padding")
	}
}

func TestWritePaddedText_ExactWidth(t *testing.T) {
	var sb strings.Builder
	WritePaddedText(&sb, "abcde", 5, "")
	got := sb.String()

	// No padding needed, text is exactly the target width
	if got != "abcde" {
		t.Errorf("expected 'abcde' with no padding, got %q", got)
	}
}

func TestWritePaddedText_OverWidth(t *testing.T) {
	var sb strings.Builder
	WritePaddedText(&sb, "abcdefgh", 5, "")
	got := sb.String()

	// Text wider than target: no padding added
	if got != "abcdefgh" {
		t.Errorf("expected 'abcdefgh' unchanged, got %q", got)
	}
}
