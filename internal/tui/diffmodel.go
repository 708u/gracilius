package tui

import (
	"strings"

	diffmatchpatch "github.com/sergi/go-diff/diffmatchpatch"
)

// diffRowType classifies a row in the side-by-side diff view.
type diffRowType int

const (
	diffRowUnchanged diffRowType = iota
	diffRowModified              // delete+insert paired on the same row
	diffRowAdded
	diffRowDeleted
)

// diffRow represents a single row in the side-by-side diff.
type diffRow struct {
	oldLineNum int // 1-based; 0 means no line on this side
	newLineNum int
	oldText    string
	newText    string
	rowType    diffRowType
	oldSpans   []wordSpan // word-level diff spans (modified rows only)
	newSpans   []wordSpan
}

// diffHunk represents a contiguous range of changed rows with context.
type diffHunk struct {
	startIdx int // inclusive (index into rows)
	endIdx   int // exclusive
}

// diffStats holds summary counts for a diff.
type diffStats struct {
	additions int
	deletions int
	modified  int
}

// diffData holds the complete processed diff for rendering.
type diffData struct {
	rows       []diffRow
	hunks      []diffHunk
	stats      diffStats
	maxLineNum int // largest line number across all rows
}

// splitDiffLines splits a Diff's text into individual lines,
// trimming the trailing empty element from DiffCharsToLines output.
func splitDiffLines(text string) []string {
	lines := strings.Split(text, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// buildDiffData computes a side-by-side diff from old and new line slices.
func buildDiffData(oldLines, newLines []string) *diffData {
	dmp := diffmatchpatch.New()

	oldText := strings.Join(oldLines, "\n")
	newText := strings.Join(newLines, "\n")

	chars1, chars2, lineArray := dmp.DiffLinesToRunes(oldText, newText)
	diffs := dmp.DiffMainRunes(chars1, chars2, false)
	diffs = dmp.DiffCharsToLines(diffs, lineArray)

	var rows []diffRow
	var stats diffStats
	oldNum, newNum := 1, 1

	i := 0
	for i < len(diffs) {
		switch diffs[i].Type {
		case diffmatchpatch.DiffEqual:
			for _, line := range splitDiffLines(diffs[i].Text) {
				rows = append(rows, diffRow{
					oldLineNum: oldNum,
					newLineNum: newNum,
					oldText:    line,
					newText:    line,
					rowType:    diffRowUnchanged,
				})
				oldNum++
				newNum++
			}
			i++

		case diffmatchpatch.DiffDelete:
			delLines := splitDiffLines(diffs[i].Text)
			i++

			// Collect consecutive inserts that follow.
			var insLines []string
			if i < len(diffs) && diffs[i].Type == diffmatchpatch.DiffInsert {
				insLines = splitDiffLines(diffs[i].Text)
				i++
			}

			// Pair deletes with inserts.
			paired := min(len(delLines), len(insLines))
			for j := range paired {
				rows = append(rows, diffRow{
					oldLineNum: oldNum,
					newLineNum: newNum,
					oldText:    delLines[j],
					newText:    insLines[j],
					rowType:    diffRowModified,
				})
				oldNum++
				newNum++
				stats.modified++
			}
			// Remaining deletes.
			for j := paired; j < len(delLines); j++ {
				rows = append(rows, diffRow{
					oldLineNum: oldNum,
					newLineNum: 0,
					oldText:    delLines[j],
					rowType:    diffRowDeleted,
				})
				oldNum++
				stats.deletions++
			}
			// Remaining inserts.
			for j := paired; j < len(insLines); j++ {
				rows = append(rows, diffRow{
					oldLineNum: 0,
					newLineNum: newNum,
					newText:    insLines[j],
					rowType:    diffRowAdded,
				})
				newNum++
				stats.additions++
			}

		case diffmatchpatch.DiffInsert:
			// Standalone inserts (not preceded by deletes).
			for _, line := range splitDiffLines(diffs[i].Text) {
				rows = append(rows, diffRow{
					oldLineNum: 0,
					newLineNum: newNum,
					newText:    line,
					rowType:    diffRowAdded,
				})
				newNum++
				stats.additions++
			}
			i++
		}
	}

	// Pre-compute word-level diffs and max line number.
	maxLine := 0
	for i := range rows {
		if rows[i].oldLineNum > maxLine {
			maxLine = rows[i].oldLineNum
		}
		if rows[i].newLineNum > maxLine {
			maxLine = rows[i].newLineNum
		}
		if rows[i].rowType == diffRowModified {
			rows[i].oldSpans, rows[i].newSpans = computeWordDiff(rows[i].oldText, rows[i].newText)
		}
	}

	hunks := detectHunks(rows, 3)

	return &diffData{
		rows:       rows,
		hunks:      hunks,
		stats:      stats,
		maxLineNum: maxLine,
	}
}

// detectHunks groups changed rows into hunks, including contextLines
// of surrounding unchanged rows. Adjacent or overlapping hunks are merged
// in a single pass.
func detectHunks(rows []diffRow, contextLines int) []diffHunk {
	var hunks []diffHunk

	for i, r := range rows {
		if r.rowType == diffRowUnchanged {
			continue
		}
		start := max(i-contextLines, 0)
		end := min(i+contextLines+1, len(rows))

		if n := len(hunks); n > 0 && start <= hunks[n-1].endIdx {
			// Extend the last hunk.
			hunks[n-1].endIdx = max(hunks[n-1].endIdx, end)
		} else {
			hunks = append(hunks, diffHunk{startIdx: start, endIdx: end})
		}
	}
	return hunks
}
