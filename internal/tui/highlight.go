package tui

import (
	"fmt"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/muesli/termenv"
)

type themeConfig struct {
	name                string // Chroma style name
	selectionBg         string // Editor selection background hex color
	listSelectionBg     string // List/tree active selection hex color
	activeFileBg        string // File tree active-tab file background hex color
	tabActiveFg         string // Active tab foreground hex color
	tabActiveBorder     string // Active tab underline hex color
	tabInactiveFg       string // Inactive tab foreground hex color
	openFileSelectionBg string // Open-file overlay selection bg
	openFileMatchFg     string // Fuzzy match highlight fg
	logoLeaf            string // Welcome logo top color (green/leaf)
	logoTrunk           string // Welcome logo bottom color (brown/trunk)
	searchMatchBg       string // Search match background hex color
	searchCurrentBg     string // Current search match background hex color
}

func (t themeConfig) selectionBgSeq() string {
	return termenv.CSI + termenv.RGBColor(t.selectionBg).Sequence(true) + "m"
}

func (t themeConfig) searchMatchBgSeq() string {
	return termenv.CSI + termenv.RGBColor(t.searchMatchBg).Sequence(true) + "m"
}

func (t themeConfig) searchCurrentBgSeq() string {
	return termenv.CSI + termenv.RGBColor(t.searchCurrentBg).Sequence(true) + "m"
}

var (
	darkTheme = themeConfig{
		name:                "github-dark",
		selectionBg:         "#264F78",
		listSelectionBg:     "#37373D",
		activeFileBg:        "#2A2D2E",
		tabActiveFg:         "#FFFFFF",
		tabActiveBorder:     "#E8AB53",
		tabInactiveFg:       "#969696",
		openFileSelectionBg: "#04395E",
		openFileMatchFg:     "#FFCC66",
		logoLeaf:            "#73C991",
		logoTrunk:           "#CE9178",
		searchMatchBg:       "#613214",
		searchCurrentBg:     "#9E6A03",
	}
	lightTheme = themeConfig{
		name:                "github",
		selectionBg:         "#ADD6FF",
		listSelectionBg:     "#B8D8F8",
		activeFileBg:        "#E4E6F1",
		tabActiveFg:         "#333333",
		tabActiveBorder:     "#005FB8",
		tabInactiveFg:       "#6E6E6E",
		openFileSelectionBg: "#C4E0F9",
		openFileMatchFg:     "#0066CC",
		logoLeaf:            "#1B7F37",
		logoTrunk:           "#795E26",
		searchMatchBg:       "#FFF2CC",
		searchCurrentBg:     "#FFD700",
	}
)

var (
	ansiReset = termenv.CSI + termenv.ResetSeq + "m"
	ansiFaint = termenv.CSI + termenv.FaintSeq + "m"
)

type styledRun struct {
	Text string // Raw text (tabs unexpanded)
	ANSI string // SGR prefix, empty = no styling
}

type highlightedLine struct {
	rendered string      // Pre-rendered ANSI string for normal display
	runs     []styledRun // For cursor/selection splitting
}

func highlightFile(filePath, source string, theme themeConfig) []highlightedLine {
	lexer := lexers.Match(filePath)
	if lexer == nil {
		lexer = lexers.Fallback
	}
	lexer = chroma.Coalesce(lexer)

	style := styles.Get(theme.name)

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

func renderStyledLineWithSelection(sb *strings.Builder, runs []styledRun, selStart, selEnd int, selBgSeq string) {
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
			sb.WriteString(selBgSeq)
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

// highlightRange represents a range to highlight with a specific background.
type highlightRange struct {
	start int    // rune offset (inclusive)
	end   int    // rune offset (exclusive)
	bgSeq string // ANSI SGR background sequence
}

// renderStyledLineWithHighlights renders runs with multiple highlight ranges.
// Highlights are applied in order; later ranges override earlier ones for
// overlapping positions.
func renderStyledLineWithHighlights(sb *strings.Builder, runs []styledRun, highlights []highlightRange) {
	if len(highlights) == 0 {
		for _, run := range runs {
			writeStyledText(sb, run.ANSI, expandTabs(run.Text))
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
		for i := h.start; i < h.end && i < totalRunes; i++ {
			if i >= 0 {
				bgMap[i] = h.bgSeq
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
			chunk := expandTabs(string(runes[i:j]))
			if bg != "" {
				if run.ANSI != "" {
					sb.WriteString(run.ANSI)
				}
				sb.WriteString(bg)
				sb.WriteString(chunk)
				sb.WriteString(ansiReset)
			} else {
				writeStyledText(sb, run.ANSI, chunk)
			}
			i = j
		}
		pos += runLen
	}
}
