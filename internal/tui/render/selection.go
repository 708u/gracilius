package render

// HighlightRange represents a range to highlight with a specific background.
type HighlightRange struct {
	Start int    // rune offset (inclusive)
	End   int    // rune offset (exclusive)
	BgSeq string // ANSI SGR background sequence
}

// ClampHighlightsToSegment adjusts highlight ranges to be relative to a
// wrapped segment starting at wrapOff with segLen runes.
func ClampHighlightsToSegment(highlights []HighlightRange, wrapOff, segLen int) []HighlightRange {
	var clamped []HighlightRange
	for _, h := range highlights {
		s := max(0, h.Start-wrapOff)
		e := min(segLen, h.End-wrapOff)
		if s < e {
			clamped = append(clamped, HighlightRange{Start: s, End: e, BgSeq: h.BgSeq})
		}
	}
	return clamped
}
