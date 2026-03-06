package tui

import (
	"testing"
)

func TestBuildDiffData_Identical(t *testing.T) {
	lines := []string{"aaa", "bbb", "ccc"}
	d := buildDiffData(lines, lines)

	if len(d.rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(d.rows))
	}
	for i, r := range d.rows {
		if r.rowType != diffRowUnchanged {
			t.Errorf("row %d: expected unchanged, got %d", i, r.rowType)
		}
	}
	if len(d.hunks) != 0 {
		t.Fatalf("expected 0 hunks, got %d", len(d.hunks))
	}
	if d.stats.additions != 0 || d.stats.deletions != 0 || d.stats.modified != 0 {
		t.Errorf("expected zero stats, got %+v", d.stats)
	}
}

func TestBuildDiffData_AllAdded(t *testing.T) {
	d := buildDiffData(nil, []string{"aaa", "bbb"})

	if len(d.rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(d.rows))
	}
	for i, r := range d.rows {
		if r.rowType != diffRowAdded {
			t.Errorf("row %d: expected added, got %d", i, r.rowType)
		}
		if r.oldLineNum != 0 {
			t.Errorf("row %d: expected oldLineNum 0, got %d", i, r.oldLineNum)
		}
	}
	if d.stats.additions != 2 {
		t.Errorf("expected 2 additions, got %d", d.stats.additions)
	}
}

func TestBuildDiffData_AllDeleted(t *testing.T) {
	d := buildDiffData([]string{"aaa", "bbb"}, nil)

	if len(d.rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(d.rows))
	}
	for i, r := range d.rows {
		if r.rowType != diffRowDeleted {
			t.Errorf("row %d: expected deleted, got %d", i, r.rowType)
		}
		if r.newLineNum != 0 {
			t.Errorf("row %d: expected newLineNum 0, got %d", i, r.newLineNum)
		}
	}
	if d.stats.deletions != 2 {
		t.Errorf("expected 2 deletions, got %d", d.stats.deletions)
	}
}

func TestBuildDiffData_SingleModification(t *testing.T) {
	old := []string{"aaa", "bbb", "ccc"}
	new := []string{"aaa", "BBB", "ccc"}
	d := buildDiffData(old, new)

	var modCount int
	for _, r := range d.rows {
		if r.rowType == diffRowModified {
			modCount++
			if r.oldText != "bbb" {
				t.Errorf("expected oldText 'bbb', got %q", r.oldText)
			}
			if r.newText != "BBB" {
				t.Errorf("expected newText 'BBB', got %q", r.newText)
			}
		}
	}
	if modCount != 1 {
		t.Fatalf("expected 1 modified row, got %d", modCount)
	}
}

func TestBuildDiffData_UnevenPairing(t *testing.T) {
	old := []string{"aaa", "bbb", "ccc", "ddd"}
	new := []string{"aaa", "BBB", "CCC", "ddd"}
	// bbb→BBB, ccc→CCC: delete 2 + insert 2 → modified 2
	d := buildDiffData(old, new)

	if d.stats.modified != 2 {
		t.Errorf("expected 2 modified, got %d", d.stats.modified)
	}
	if d.stats.additions != 0 {
		t.Errorf("expected 0 additions, got %d", d.stats.additions)
	}
	if d.stats.deletions != 0 {
		t.Errorf("expected 0 deletions, got %d", d.stats.deletions)
	}

	// Now test uneven: delete 3, insert 2.
	old2 := []string{"aaa", "bbb", "ccc", "ddd", "eee"}
	new2 := []string{"aaa", "BBB", "CCC", "eee"}
	d2 := buildDiffData(old2, new2)

	if d2.stats.modified != 2 {
		t.Errorf("expected 2 modified, got %d", d2.stats.modified)
	}
	if d2.stats.deletions != 1 {
		t.Errorf("expected 1 deletion, got %d", d2.stats.deletions)
	}
}

func TestBuildDiffData_LineNumbers(t *testing.T) {
	old := []string{"aaa", "bbb", "ccc"}
	new := []string{"aaa", "BBB", "ccc"}
	d := buildDiffData(old, new)

	tests := []struct {
		idx    int
		oldNum int
		newNum int
	}{
		{0, 1, 1}, // unchanged "aaa"
		{1, 2, 2}, // modified "bbb"→"BBB"
		{2, 3, 3}, // unchanged "ccc"
	}
	for _, tt := range tests {
		if tt.idx >= len(d.rows) {
			t.Fatalf("row %d out of range (len=%d)", tt.idx, len(d.rows))
		}
		r := d.rows[tt.idx]
		if r.oldLineNum != tt.oldNum {
			t.Errorf("row %d: expected oldLineNum %d, got %d", tt.idx, tt.oldNum, r.oldLineNum)
		}
		if r.newLineNum != tt.newNum {
			t.Errorf("row %d: expected newLineNum %d, got %d", tt.idx, tt.newNum, r.newLineNum)
		}
	}
}

func TestBuildDiffData_Stats(t *testing.T) {
	old := []string{"aaa", "bbb", "ccc", "ddd", "eee"}
	new := []string{"aaa", "BBB", "ddd", "fff", "eee"}
	// bbb→BBB (modified), ccc deleted, fff added
	d := buildDiffData(old, new)

	if d.stats.modified != 1 {
		t.Errorf("expected 1 modified, got %d", d.stats.modified)
	}
	if d.stats.deletions != 1 {
		t.Errorf("expected 1 deletion, got %d", d.stats.deletions)
	}
	if d.stats.additions != 1 {
		t.Errorf("expected 1 addition, got %d", d.stats.additions)
	}
}

func TestDetectHunks_SingleChange(t *testing.T) {
	old := []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10"}
	new := []string{"1", "2", "3", "4", "X", "6", "7", "8", "9", "10"}
	d := buildDiffData(old, new)

	if len(d.hunks) != 1 {
		t.Fatalf("expected 1 hunk, got %d", len(d.hunks))
	}
	h := d.hunks[0]
	// Changed row is at index 4, context=3 → start=1, end=8
	if h.startIdx != 1 {
		t.Errorf("expected startIdx 1, got %d", h.startIdx)
	}
	if h.endIdx != 8 {
		t.Errorf("expected endIdx 8, got %d", h.endIdx)
	}
}

func TestDetectHunks_MergedHunks(t *testing.T) {
	// Two changes 5 rows apart → context overlap → single merged hunk.
	old := []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10"}
	new := []string{"1", "X", "3", "4", "5", "6", "Y", "8", "9", "10"}
	d := buildDiffData(old, new)

	if len(d.hunks) != 1 {
		t.Fatalf("expected 1 merged hunk, got %d", len(d.hunks))
	}
}

func TestDetectHunks_SeparateHunks(t *testing.T) {
	// Two changes far apart → two separate hunks.
	lines := make([]string, 20)
	for i := range lines {
		lines[i] = "line"
	}
	old := make([]string, 20)
	copy(old, lines)

	new := make([]string, 20)
	copy(new, lines)
	new[1] = "X"
	new[18] = "Y"

	d := buildDiffData(old, new)

	if len(d.hunks) != 2 {
		t.Fatalf("expected 2 separate hunks, got %d", len(d.hunks))
	}
	if d.hunks[0].endIdx > d.hunks[1].startIdx {
		t.Error("hunks should not overlap")
	}
}

func TestDetectHunks_NoChanges(t *testing.T) {
	lines := []string{"aaa", "bbb", "ccc"}
	d := buildDiffData(lines, lines)

	if len(d.hunks) != 0 {
		t.Fatalf("expected 0 hunks, got %d", len(d.hunks))
	}
}
