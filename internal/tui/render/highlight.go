package render

import (
	"fmt"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/muesli/termenv"
)

// StyledRun represents a styled text segment.
type StyledRun struct {
	Text string // Raw text (tabs unexpanded)
	ANSI string // SGR prefix, empty = no styling
}

// HighlightedLine holds a pre-rendered line and its constituent runs.
type HighlightedLine struct {
	Rendered string      // Pre-rendered ANSI string for normal display
	Runs     []StyledRun // For cursor/selection splitting
}

// HighlightFile tokenises source and returns syntax-highlighted lines.
func HighlightFile(filePath, source string, theme Theme) []HighlightedLine {
	lexer := lexers.Match(filePath)
	if lexer == nil {
		lexer = lexers.Fallback
	}
	lexer = chroma.Coalesce(lexer)

	style := styles.Get(theme.Name)

	iterator, err := lexer.Tokenise(nil, source)
	if err != nil {
		return nil
	}

	tokens := iterator.Tokens()

	var result []HighlightedLine
	var currentRuns []StyledRun

	for _, token := range tokens {
		ansiCode := ResolveANSI(style, token.Type)
		parts := strings.Split(token.Value, "\n")
		for i, part := range parts {
			if part != "" {
				currentRuns = append(currentRuns, StyledRun{
					Text: part, ANSI: ansiCode,
				})
			}
			if i < len(parts)-1 {
				result = append(result, NewHighlightedLine(currentRuns))
				currentRuns = nil
			}
		}
	}
	result = append(result, NewHighlightedLine(currentRuns))

	return result
}

// NewHighlightedLine creates a HighlightedLine from runs.
func NewHighlightedLine(runs []StyledRun) HighlightedLine {
	var sb strings.Builder
	for _, run := range runs {
		WriteStyledText(&sb, run.ANSI, ExpandTabs(run.Text))
	}
	return HighlightedLine{Rendered: sb.String(), Runs: runs}
}

// ResolveANSI returns the ANSI SGR sequence for a Chroma token type.
func ResolveANSI(style *chroma.Style, tokenType chroma.TokenType) string {
	entry := style.Get(tokenType)
	if entry.IsZero() {
		return ""
	}

	var params []string
	if entry.Colour.IsSet() { //nolint:misspell // Colour is the chroma library's field name
		r, g, b := entry.Colour.Red(), entry.Colour.Green(), entry.Colour.Blue() //nolint:misspell // Colour is the chroma library's field name
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

// RenderStyledLineWithSelection renders runs with a single selection range.
func RenderStyledLineWithSelection(sb *strings.Builder, runs []StyledRun, selStart, selEnd int, selBgSeq string) {
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
			WriteStyledText(sb, run.ANSI, ExpandTabs(run.Text))
		} else {
			// Before selection
			if overlapStart > runStart {
				beforeEnd := overlapStart - runStart
				WriteStyledText(sb, run.ANSI, ExpandTabs(string(runes[:beforeEnd])))
			}

			// Within selection
			selLocalStart := overlapStart - runStart
			selLocalEnd := overlapEnd - runStart
			if run.ANSI != "" {
				sb.WriteString(run.ANSI)
			}
			sb.WriteString(selBgSeq)
			sb.WriteString(ExpandTabs(string(runes[selLocalStart:selLocalEnd])))
			sb.WriteString(AnsiReset)

			// After selection
			if overlapEnd < runEnd {
				afterStart := overlapEnd - runStart
				WriteStyledText(sb, run.ANSI, ExpandTabs(string(runes[afterStart:])))
			}
		}

		pos = runEnd
	}
}

// RenderStyledLineWithHighlights renders runs with multiple highlight ranges.
// Highlights are applied in order; later ranges override earlier ones for
// overlapping positions.
func RenderStyledLineWithHighlights(sb *strings.Builder, runs []StyledRun, highlights []HighlightRange) {
	if len(highlights) == 0 {
		for _, run := range runs {
			WriteStyledText(sb, run.ANSI, ExpandTabs(run.Text))
		}
		return
	}

	// Build a per-rune background map from highlights (later wins).
	totalRunes := 0
	for _, r := range runs {
		totalRunes += len([]rune(r.Text))
	}
	bgMap := make([]string, totalRunes)
	for _, h := range highlights {
		for i := h.Start; i < h.End && i < totalRunes; i++ {
			if i >= 0 {
				bgMap[i] = h.BgSeq
			}
		}
	}

	pos := 0
	for _, run := range runs {
		runes := []rune(run.Text)
		runLen := len(runes)
		// Split run into chunks with the same background.
		i := 0
		for i < runLen {
			bg := bgMap[pos+i]
			j := i + 1
			for j < runLen && bgMap[pos+j] == bg {
				j++
			}
			chunk := ExpandTabs(string(runes[i:j]))
			if bg != "" {
				if run.ANSI != "" {
					sb.WriteString(run.ANSI)
				}
				sb.WriteString(bg)
				sb.WriteString(chunk)
				sb.WriteString(AnsiReset)
			} else {
				WriteStyledText(sb, run.ANSI, chunk)
			}
			i = j
		}
		pos += runLen
	}
}
