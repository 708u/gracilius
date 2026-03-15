package tui

// splitRunsAtBreakpoints divides styledRuns at the given rune-index
// breakpoints, returning one []styledRun per visual wrap segment.
// bp must be sorted in ascending order (as returned by render.WrapBreakpoints).
func splitRunsAtBreakpoints(runs []styledRun, bp []int) [][]styledRun {
	segments := make([][]styledRun, 0, len(bp)+1)
	var current []styledRun
	pos := 0
	bpIdx := 0

	for _, run := range runs {
		runes := []rune(run.Text)
		runEnd := pos + len(runes)
		consumed := 0

		for bpIdx < len(bp) && bp[bpIdx] >= pos && bp[bpIdx] < runEnd {
			splitAt := bp[bpIdx] - pos
			if splitAt > consumed {
				current = append(current, styledRun{
					Text: string(runes[consumed:splitAt]),
					ANSI: run.ANSI,
				})
			}
			segments = append(segments, current)
			current = nil
			consumed = splitAt
			bpIdx++
		}

		if consumed < len(runes) {
			current = append(current, styledRun{
				Text: string(runes[consumed:]),
				ANSI: run.ANSI,
			})
		}

		pos = runEnd
	}

	segments = append(segments, current)
	return segments
}
