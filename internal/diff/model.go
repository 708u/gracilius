package diff

import (
	"strings"

	diffmatchpatch "github.com/sergi/go-diff/diffmatchpatch"
)

// RowType classifies a row in the side-by-side diff view.
type RowType int

const (
	RowUnchanged RowType = iota
	RowModified          // delete+insert paired on the same row
	RowAdded
	RowDeleted
)

// Row represents a single row in the side-by-side diff.
type Row struct {
	OldLineNum int // 1-based; 0 means no line on this side
	NewLineNum int
	OldText    string
	NewText    string
	RowType    RowType
	OldSpans   []WordSpan // word-level diff spans (modified rows only)
	NewSpans   []WordSpan
}

// Hunk represents a contiguous range of changed rows with context.
type Hunk struct {
	StartIdx int // inclusive (index into rows)
	EndIdx   int // exclusive
}

// Stats holds summary counts for a diff.
type Stats struct {
	Additions int
	Deletions int
	Modified  int
}

// Data holds the complete processed diff for rendering.
type Data struct {
	Rows       []Row
	Hunks      []Hunk
	Stats      Stats
	MaxLineNum int // largest line number across all rows
}

// SplitDiffLines splits a Diff's text into individual lines,
// trimming the trailing empty element from DiffCharsToLines output.
func SplitDiffLines(text string) []string {
	lines := strings.Split(text, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// Build computes a side-by-side diff from old and new line slices.
func Build(oldLines, newLines []string) *Data {
	dmp := diffmatchpatch.New()

	oldText := strings.Join(oldLines, "\n")
	newText := strings.Join(newLines, "\n")

	chars1, chars2, lineArray := dmp.DiffLinesToRunes(oldText, newText)
	diffs := dmp.DiffMainRunes(chars1, chars2, false)
	diffs = dmp.DiffCharsToLines(diffs, lineArray)

	var rows []Row
	var stats Stats
	oldNum, newNum := 1, 1

	i := 0
	for i < len(diffs) {
		switch diffs[i].Type {
		case diffmatchpatch.DiffEqual:
			for _, line := range SplitDiffLines(diffs[i].Text) {
				rows = append(rows, Row{
					OldLineNum: oldNum,
					NewLineNum: newNum,
					OldText:    line,
					NewText:    line,
					RowType:    RowUnchanged,
				})
				oldNum++
				newNum++
			}
			i++

		case diffmatchpatch.DiffDelete:
			delLines := SplitDiffLines(diffs[i].Text)
			i++

			// Collect consecutive inserts that follow.
			var insLines []string
			if i < len(diffs) && diffs[i].Type == diffmatchpatch.DiffInsert {
				insLines = SplitDiffLines(diffs[i].Text)
				i++
			}

			// Pair deletes with inserts.
			paired := min(len(delLines), len(insLines))
			for j := range paired {
				rows = append(rows, Row{
					OldLineNum: oldNum,
					NewLineNum: newNum,
					OldText:    delLines[j],
					NewText:    insLines[j],
					RowType:    RowModified,
				})
				oldNum++
				newNum++
				stats.Modified++
			}
			// Remaining deletes.
			for j := paired; j < len(delLines); j++ {
				rows = append(rows, Row{
					OldLineNum: oldNum,
					NewLineNum: 0,
					OldText:    delLines[j],
					RowType:    RowDeleted,
				})
				oldNum++
				stats.Deletions++
			}
			// Remaining inserts.
			for j := paired; j < len(insLines); j++ {
				rows = append(rows, Row{
					OldLineNum: 0,
					NewLineNum: newNum,
					NewText:    insLines[j],
					RowType:    RowAdded,
				})
				newNum++
				stats.Additions++
			}

		case diffmatchpatch.DiffInsert:
			// Standalone inserts (not preceded by deletes).
			for _, line := range SplitDiffLines(diffs[i].Text) {
				rows = append(rows, Row{
					OldLineNum: 0,
					NewLineNum: newNum,
					NewText:    line,
					RowType:    RowAdded,
				})
				newNum++
				stats.Additions++
			}
			i++
		}
	}

	// Pre-compute word-level diffs and max line number.
	maxLine := 0
	for i := range rows {
		if rows[i].OldLineNum > maxLine {
			maxLine = rows[i].OldLineNum
		}
		if rows[i].NewLineNum > maxLine {
			maxLine = rows[i].NewLineNum
		}
		if rows[i].RowType == RowModified {
			rows[i].OldSpans, rows[i].NewSpans = ComputeWordDiff(rows[i].OldText, rows[i].NewText)
		}
	}

	hunks := DetectHunks(rows, 3)

	return &Data{
		Rows:       rows,
		Hunks:      hunks,
		Stats:      stats,
		MaxLineNum: maxLine,
	}
}

// DetectHunks groups changed rows into hunks, including contextLines
// of surrounding unchanged rows. Adjacent or overlapping hunks are merged
// in a single pass.
func DetectHunks(rows []Row, contextLines int) []Hunk {
	var hunks []Hunk

	for i, r := range rows {
		if r.RowType == RowUnchanged {
			continue
		}
		start := max(i-contextLines, 0)
		end := min(i+contextLines+1, len(rows))

		if n := len(hunks); n > 0 && start <= hunks[n-1].EndIdx {
			// Extend the last hunk.
			hunks[n-1].EndIdx = max(hunks[n-1].EndIdx, end)
		} else {
			hunks = append(hunks, Hunk{StartIdx: start, EndIdx: end})
		}
	}
	return hunks
}
