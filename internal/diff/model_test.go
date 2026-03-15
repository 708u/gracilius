package diff

import (
	"testing"
)

func TestBuildDiffData_Identical(t *testing.T) {
	lines := []string{"aaa", "bbb", "ccc"}
	d := Build(lines, lines)

	if len(d.Rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(d.Rows))
	}
	for i, r := range d.Rows {
		if r.RowType != RowUnchanged {
			t.Errorf("row %d: expected unchanged, got %d", i, r.RowType)
		}
	}
	if len(d.Hunks) != 0 {
		t.Fatalf("expected 0 hunks, got %d", len(d.Hunks))
	}
	if d.Stats.Additions != 0 || d.Stats.Deletions != 0 || d.Stats.Modified != 0 {
		t.Errorf("expected zero stats, got %+v", d.Stats)
	}
}

func TestBuildDiffData_AllAdded(t *testing.T) {
	d := Build(nil, []string{"aaa", "bbb"})

	if len(d.Rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(d.Rows))
	}
	for i, r := range d.Rows {
		if r.RowType != RowAdded {
			t.Errorf("row %d: expected added, got %d", i, r.RowType)
		}
		if r.OldLineNum != 0 {
			t.Errorf("row %d: expected OldLineNum 0, got %d", i, r.OldLineNum)
		}
	}
	if d.Stats.Additions != 2 {
		t.Errorf("expected 2 additions, got %d", d.Stats.Additions)
	}
}

func TestBuildDiffData_AllDeleted(t *testing.T) {
	d := Build([]string{"aaa", "bbb"}, nil)

	if len(d.Rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(d.Rows))
	}
	for i, r := range d.Rows {
		if r.RowType != RowDeleted {
			t.Errorf("row %d: expected deleted, got %d", i, r.RowType)
		}
		if r.NewLineNum != 0 {
			t.Errorf("row %d: expected NewLineNum 0, got %d", i, r.NewLineNum)
		}
	}
	if d.Stats.Deletions != 2 {
		t.Errorf("expected 2 deletions, got %d", d.Stats.Deletions)
	}
}

func TestBuildDiffData_SingleModification(t *testing.T) {
	old := []string{"aaa", "bbb", "ccc"}
	new := []string{"aaa", "BBB", "ccc"}
	d := Build(old, new)

	var modCount int
	for _, r := range d.Rows {
		if r.RowType == RowModified {
			modCount++
			if r.OldText != "bbb" {
				t.Errorf("expected OldText 'bbb', got %q", r.OldText)
			}
			if r.NewText != "BBB" {
				t.Errorf("expected NewText 'BBB', got %q", r.NewText)
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
	// bbb->BBB, ccc->CCC: delete 2 + insert 2 -> modified 2
	d := Build(old, new)

	if d.Stats.Modified != 2 {
		t.Errorf("expected 2 modified, got %d", d.Stats.Modified)
	}
	if d.Stats.Additions != 0 {
		t.Errorf("expected 0 additions, got %d", d.Stats.Additions)
	}
	if d.Stats.Deletions != 0 {
		t.Errorf("expected 0 deletions, got %d", d.Stats.Deletions)
	}

	// Now test uneven: delete 3, insert 2.
	old2 := []string{"aaa", "bbb", "ccc", "ddd", "eee"}
	new2 := []string{"aaa", "BBB", "CCC", "eee"}
	d2 := Build(old2, new2)

	if d2.Stats.Modified != 2 {
		t.Errorf("expected 2 modified, got %d", d2.Stats.Modified)
	}
	if d2.Stats.Deletions != 1 {
		t.Errorf("expected 1 deletion, got %d", d2.Stats.Deletions)
	}
}

func TestBuildDiffData_LineNumbers(t *testing.T) {
	old := []string{"aaa", "bbb", "ccc"}
	new := []string{"aaa", "BBB", "ccc"}
	d := Build(old, new)

	tests := []struct {
		idx    int
		oldNum int
		newNum int
	}{
		{0, 1, 1}, // unchanged "aaa"
		{1, 2, 2}, // modified "bbb"->"BBB"
		{2, 3, 3}, // unchanged "ccc"
	}
	for _, tt := range tests {
		if tt.idx >= len(d.Rows) {
			t.Fatalf("row %d out of range (len=%d)", tt.idx, len(d.Rows))
		}
		r := d.Rows[tt.idx]
		if r.OldLineNum != tt.oldNum {
			t.Errorf("row %d: expected OldLineNum %d, got %d", tt.idx, tt.oldNum, r.OldLineNum)
		}
		if r.NewLineNum != tt.newNum {
			t.Errorf("row %d: expected NewLineNum %d, got %d", tt.idx, tt.newNum, r.NewLineNum)
		}
	}
}

func TestBuildDiffData_Stats(t *testing.T) {
	old := []string{"aaa", "bbb", "ccc", "ddd", "eee"}
	new := []string{"aaa", "BBB", "ddd", "fff", "eee"}
	// bbb->BBB (modified), ccc deleted, fff added
	d := Build(old, new)

	if d.Stats.Modified != 1 {
		t.Errorf("expected 1 modified, got %d", d.Stats.Modified)
	}
	if d.Stats.Deletions != 1 {
		t.Errorf("expected 1 deletion, got %d", d.Stats.Deletions)
	}
	if d.Stats.Additions != 1 {
		t.Errorf("expected 1 addition, got %d", d.Stats.Additions)
	}
}

func TestDetectHunks_SingleChange(t *testing.T) {
	old := []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10"}
	new := []string{"1", "2", "3", "4", "X", "6", "7", "8", "9", "10"}
	d := Build(old, new)

	if len(d.Hunks) != 1 {
		t.Fatalf("expected 1 hunk, got %d", len(d.Hunks))
	}
	h := d.Hunks[0]
	// Changed row is at index 4, context=3 -> start=1, end=8
	if h.StartIdx != 1 {
		t.Errorf("expected StartIdx 1, got %d", h.StartIdx)
	}
	if h.EndIdx != 8 {
		t.Errorf("expected EndIdx 8, got %d", h.EndIdx)
	}
}

func TestDetectHunks_MergedHunks(t *testing.T) {
	// Two changes 5 rows apart -> context overlap -> single merged hunk.
	old := []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10"}
	new := []string{"1", "X", "3", "4", "5", "6", "Y", "8", "9", "10"}
	d := Build(old, new)

	if len(d.Hunks) != 1 {
		t.Fatalf("expected 1 merged hunk, got %d", len(d.Hunks))
	}
}

func TestDetectHunks_SeparateHunks(t *testing.T) {
	// Two changes far apart -> two separate hunks.
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

	d := Build(old, new)

	if len(d.Hunks) != 2 {
		t.Fatalf("expected 2 separate hunks, got %d", len(d.Hunks))
	}
	if d.Hunks[0].EndIdx > d.Hunks[1].StartIdx {
		t.Error("hunks should not overlap")
	}
}

func TestDetectHunks_NoChanges(t *testing.T) {
	lines := []string{"aaa", "bbb", "ccc"}
	d := Build(lines, lines)

	if len(d.Hunks) != 0 {
		t.Fatalf("expected 0 hunks, got %d", len(d.Hunks))
	}
}
