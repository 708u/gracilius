package render

import (
	"strings"
	"testing"
)

func TestWriteStyledText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		ansiCode string
		text     string
		verify   func(t *testing.T, got string)
	}{
		{
			name:     "WithANSI",
			ansiCode: "\033[38;5;148m",
			text:     "hello",
			verify: func(t *testing.T, got string) {
				t.Helper()
				ansi := "\033[38;5;148m"
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
			},
		},
		{
			name:     "NoANSI",
			ansiCode: "",
			text:     "plain",
			verify: func(t *testing.T, got string) {
				t.Helper()
				if got != "plain" {
					t.Errorf("expected 'plain', got %q", got)
				}
				if strings.Contains(got, "\033[") {
					t.Error("expected no ANSI codes for empty ansiCode")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var sb strings.Builder
			WriteStyledText(&sb, tt.ansiCode, tt.text)
			tt.verify(t, sb.String())
		})
	}
}

func TestWriteColoredChunk(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		fg     string
		bg     string
		text   string
		verify func(t *testing.T, got string)
	}{
		{
			name: "WithFgAndBg",
			fg:   "\033[31m",
			bg:   "\033[44m",
			text: "text",
			verify: func(t *testing.T, got string) {
				t.Helper()
				if !strings.HasPrefix(got, "\033[31m") {
					t.Errorf("expected fg prefix, got %q", got)
				}
				if !strings.Contains(got, "\033[44m") {
					t.Errorf("expected bg in output")
				}
				if !strings.Contains(got, "text") {
					t.Error("expected 'text' in output")
				}
				if !strings.HasSuffix(got, AnsiReset) {
					t.Error("expected reset suffix")
				}
			},
		},
		{
			name: "FgOnly",
			fg:   "\033[31m",
			bg:   "",
			text: "text",
			verify: func(t *testing.T, got string) {
				t.Helper()
				if !strings.HasPrefix(got, "\033[31m") {
					t.Errorf("expected fg prefix, got %q", got)
				}
				if !strings.HasSuffix(got, AnsiReset) {
					t.Error("expected reset suffix")
				}
			},
		},
		{
			name: "NoStyle",
			fg:   "",
			bg:   "",
			text: "text",
			verify: func(t *testing.T, got string) {
				t.Helper()
				if got != "text" {
					t.Errorf("expected 'text', got %q", got)
				}
				if strings.Contains(got, "\033[") {
					t.Error("expected no ANSI codes")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var sb strings.Builder
			WriteColoredChunk(&sb, tt.fg, tt.bg, tt.text)
			tt.verify(t, sb.String())
		})
	}
}

func TestWritePaddedText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		text   string
		width  int
		bg     string
		verify func(t *testing.T, got string)
	}{
		{
			name:  "WithBg",
			text:  "hi",
			width: 10,
			bg:    "\033[44m",
			verify: func(t *testing.T, got string) {
				t.Helper()
				if !strings.HasPrefix(got, "\033[44m") {
					t.Errorf("expected bg prefix, got %q", got)
				}
				if !strings.Contains(got, "hi") {
					t.Error("expected 'hi' in output")
				}
				if !strings.HasSuffix(got, AnsiReset) {
					t.Error("expected reset suffix")
				}
				if !strings.Contains(got, "        ") {
					t.Error("expected 8 spaces padding")
				}
			},
		},
		{
			name:  "NoBg",
			text:  "hi",
			width: 10,
			bg:    "",
			verify: func(t *testing.T, got string) {
				t.Helper()
				if !strings.Contains(got, "hi") {
					t.Error("expected 'hi' in output")
				}
				if strings.Contains(got, AnsiReset) {
					t.Error("expected no reset without bg")
				}
				if !strings.Contains(got, "        ") {
					t.Error("expected 8 spaces padding")
				}
			},
		},
		{
			name:  "ExactWidth",
			text:  "abcde",
			width: 5,
			bg:    "",
			verify: func(t *testing.T, got string) {
				t.Helper()
				if got != "abcde" {
					t.Errorf("expected 'abcde' with no padding, got %q", got)
				}
			},
		},
		{
			name:  "OverWidth",
			text:  "abcdefgh",
			width: 5,
			bg:    "",
			verify: func(t *testing.T, got string) {
				t.Helper()
				if got != "abcdefgh" {
					t.Errorf("expected 'abcdefgh' unchanged, got %q", got)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var sb strings.Builder
			WritePaddedText(&sb, tt.text, tt.width, tt.bg)
			tt.verify(t, sb.String())
		})
	}
}
