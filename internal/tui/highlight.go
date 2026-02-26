package tui

import (
	"fmt"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

type themeConfig struct {
	name            string // Chroma style name
	selectionBg     string // Editor selection background hex color
	listSelectionBg string // List/tree active selection hex color
}

func (t themeConfig) selectionBgSeq() string {
	return termenv.CSI + termenv.RGBColor(t.selectionBg).Sequence(true) + "m"
}

var (
	darkTheme = themeConfig{
		name:            "github-dark",
		selectionBg:     "#264F78",
		listSelectionBg: "#37373D",
	}
	lightTheme = themeConfig{
		name:            "github",
		selectionBg:     "#ADD6FF",
		listSelectionBg: "#B8D8F8",
	}
	activeTheme = darkTheme // default fallback
)

func init() {
	if lipgloss.HasDarkBackground() {
		activeTheme = darkTheme
	} else {
		activeTheme = lightTheme
	}
}

var (
	ansiInverse = termenv.CSI + termenv.ReverseSeq + "m"
	ansiReset   = termenv.CSI + termenv.ResetSeq + "m"
)

type styledRun struct {
	Text string // Raw text (tabs unexpanded)
	ANSI string // SGR prefix, empty = no styling
}

type highlightedLine struct {
	rendered string      // Pre-rendered ANSI string for normal display
	runs     []styledRun // For cursor/selection splitting
}

func highlightFile(filePath, source string) []highlightedLine {
	lexer := lexers.Match(filePath)
	if lexer == nil {
		lexer = lexers.Fallback
	}
	lexer = chroma.Coalesce(lexer)

	style := styles.Get(activeTheme.name)

	iterator, err := lexer.Tokenise(nil, source)
	if err != nil {
		return nil
	}

	tokens := iterator.Tokens()

	var result []highlightedLine
	var currentRuns []styledRun

	for _, token := range tokens {
		ansi := resolveANSI(style, token.Type)
		parts := strings.Split(token.Value, "\n")
		for i, part := range parts {
			if part != "" {
				currentRuns = append(currentRuns, styledRun{
					Text: part, ANSI: ansi,
				})
			}
			if i < len(parts)-1 {
				result = append(result, newHighlightedLine(currentRuns))
				currentRuns = nil
			}
		}
	}
	result = append(result, newHighlightedLine(currentRuns))

	return result
}

func newHighlightedLine(runs []styledRun) highlightedLine {
	var sb strings.Builder
	for _, run := range runs {
		writeStyledText(&sb, run.ANSI, expandTabs(run.Text))
	}
	return highlightedLine{rendered: sb.String(), runs: runs}
}

func resolveANSI(style *chroma.Style, tokenType chroma.TokenType) string {
	entry := style.Get(tokenType)
	if entry.IsZero() {
		return ""
	}

	var params []string
	if entry.Colour.IsSet() {
		r, g, b := entry.Colour.Red(), entry.Colour.Green(), entry.Colour.Blue()
		c := termenv.RGBColor(fmt.Sprintf("#%02x%02x%02x", r, g, b))
		params = append(params, c.Sequence(false))
	}
	if entry.Bold == chroma.Yes {
		params = append(params, termenv.BoldSeq)
	}
	if entry.Italic == chroma.Yes {
		params = append(params, termenv.ItalicSeq)
	}
	if entry.Underline == chroma.Yes {
		params = append(params, termenv.UnderlineSeq)
	}

	if len(params) == 0 {
		return ""
	}
	return termenv.CSI + strings.Join(params, ";") + "m"
}

func renderStyledLineWithCursor(sb *strings.Builder, runs []styledRun, cursorChar int) {
	pos := 0
	cursorRendered := false

	for _, run := range runs {
		runes := []rune(run.Text)
		runStart := pos
		runEnd := pos + len(runes)

		if !cursorRendered && cursorChar >= runStart && cursorChar < runEnd {
			// Cursor falls within this run
			offset := cursorChar - runStart

			// Text before cursor
			if offset > 0 {
				writeStyledText(sb, run.ANSI, expandTabs(string(runes[:offset])))
			}

			// Cursor character
			ch := runes[offset]
			sb.WriteString(ansiInverse)
			if ch == '\t' {
				sb.WriteString("    ")
			} else {
				sb.WriteString(string(ch))
			}
			sb.WriteString(ansiReset)

			// Text after cursor
			if offset+1 < len(runes) {
				writeStyledText(sb, run.ANSI, expandTabs(string(runes[offset+1:])))
			}

			cursorRendered = true
		} else {
			writeStyledText(sb, run.ANSI, expandTabs(run.Text))
		}

		pos = runEnd
	}

	if !cursorRendered {
		// Cursor is past end of line
		sb.WriteString(ansiInverse + " " + ansiReset)
	}
}

func renderStyledLineWithSelection(sb *strings.Builder, runs []styledRun, selStart, selEnd int) {
	pos := 0

	for _, run := range runs {
		runes := []rune(run.Text)
		runStart := pos
		runEnd := pos + len(runes)

		// Determine overlap between run [runStart, runEnd) and selection [selStart, selEnd)
		overlapStart := max(runStart, selStart)
		overlapEnd := min(runEnd, selEnd)

		if overlapStart >= overlapEnd {
			// No overlap with selection
			writeStyledText(sb, run.ANSI, expandTabs(run.Text))
		} else {
			// Before selection
			if overlapStart > runStart {
				beforeEnd := overlapStart - runStart
				writeStyledText(sb, run.ANSI, expandTabs(string(runes[:beforeEnd])))
			}

			// Within selection
			selLocalStart := overlapStart - runStart
			selLocalEnd := overlapEnd - runStart
			if run.ANSI != "" {
				sb.WriteString(run.ANSI)
			}
			sb.WriteString(activeTheme.selectionBgSeq())
			sb.WriteString(expandTabs(string(runes[selLocalStart:selLocalEnd])))
			sb.WriteString(ansiReset)

			// After selection
			if overlapEnd < runEnd {
				afterStart := overlapEnd - runStart
				writeStyledText(sb, run.ANSI, expandTabs(string(runes[afterStart:])))
			}
		}

		pos = runEnd
	}
}

func writeStyledText(sb *strings.Builder, ansi, text string) {
	if ansi != "" {
		sb.WriteString(ansi)
		sb.WriteString(text)
		sb.WriteString(ansiReset)
	} else {
		sb.WriteString(text)
	}
}
