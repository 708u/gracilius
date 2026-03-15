package render

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

// AnsiReset is the ANSI SGR reset sequence.
var AnsiReset = termenv.CSI + termenv.ResetSeq + "m"

// AnsiFaint is the ANSI SGR faint sequence.
var AnsiFaint = termenv.CSI + termenv.FaintSeq + "m"

// WriteStyledText writes text with optional ANSI prefix and reset suffix.
func WriteStyledText(sb *strings.Builder, ansiCode, text string) {
	if ansiCode != "" {
		sb.WriteString(ansiCode)
		sb.WriteString(text)
		sb.WriteString(AnsiReset)
	} else {
		sb.WriteString(text)
	}
}

// WriteColoredChunk writes text with optional foreground and background ANSI codes.
func WriteColoredChunk(sb *strings.Builder, fg, bg, text string) {
	if fg != "" || bg != "" {
		sb.WriteString(fg)
		sb.WriteString(bg)
		sb.WriteString(text)
		sb.WriteString(AnsiReset)
	} else {
		sb.WriteString(text)
	}
}

// WritePaddedText writes truncated text to sb, padding to targetWidth
// with optional background color.
func WritePaddedText(sb *strings.Builder, truncated string, targetWidth int, bg string) {
	if bg != "" {
		sb.WriteString(bg)
	}
	sb.WriteString(truncated)
	if visW := ansi.StringWidth(truncated); visW < targetWidth {
		sb.WriteString(strings.Repeat(" ", targetWidth-visW))
	}
	if bg != "" {
		sb.WriteString(AnsiReset)
	}
}
