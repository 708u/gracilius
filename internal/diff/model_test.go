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
		if r.Type != RowUnchanged {
			t.Errorf("row %d: expected unchanged, got %d", i, r.Type)
		}
	}
	if len(d.Hunks) != 0 {
		t.Fatalf("expected 0 hunks, got %d", len(d.Hunks))
	}
	if d.Summary.Additions != 0 || d.Summary.Deletions != 0 || d.Summary.Modified != 0 {
		t.Errorf("expected zero stats, got %+v", d.Summary)
	}
}

func TestBuildDiffData_AllAdded(t *testing.T) {
	d := Build(nil, []string{"aaa", "bbb"})

	if len(d.Rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(d.Rows))
	}
	for i, r := range d.Rows {
		if r.Type != RowAdded {
			t.Errorf("row %d: expected added, got %d", i, r.Type)
		}
		if r.OldLineNum != 0 {
			t.Errorf("row %d: expected OldLineNum 0, got %d", i, r.OldLineNum)
		}
	}
	if d.Summary.Additions != 2 {
		t.Errorf("expected 2 additions, got %d", d.Summary.Additions)
	}
}

func TestBuildDiffData_AllDeleted(t *testing.T) {
	d := Build([]string{"aaa", "bbb"}, nil)

	if len(d.Rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(d.Rows))
	}
	for i, r := range d.Rows {
		if r.Type != RowDeleted {
			t.Errorf("row %d: expected deleted, got %d", i, r.Type)
		}
		if r.NewLineNum != 0 {
			t.Errorf("row %d: expected NewLineNum 0, got %d", i, r.NewLineNum)
		}
	}
	if d.Summary.Deletions != 2 {
		t.Errorf("expected 2 deletions, got %d", d.Summary.Deletions)
	}
}

func TestBuildDiffData_SingleModification(t *testing.T) {
	old := []string{"aaa", "bbb", "ccc"}
	new := []string{"aaa", "BBB", "ccc"}
	d := Build(old, new)

	var modCount int
	for _, r := range d.Rows {
		if r.Type == RowModified {
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

	if d.Summary.Modified != 2 {
		t.Errorf("expected 2 modified, got %d", d.Summary.Modified)
	}
	if d.Summary.Additions != 0 {
		t.Errorf("expected 0 additions, got %d", d.Summary.Additions)
	}
	if d.Summary.Deletions != 0 {
		t.Errorf("expected 0 deletions, got %d", d.Summary.Deletions)
	}

	// Now test uneven: delete 3, insert 2.
	old2 := []string{"aaa", "bbb", "ccc", "ddd", "eee"}
	new2 := []string{"aaa", "BBB", "CCC", "eee"}
	d2 := Build(old2, new2)

	if d2.Summary.Modified != 2 {
		t.Errorf("expected 2 modified, got %d", d2.Summary.Modified)
	}
	if d2.Summary.Deletions != 1 {
		t.Errorf("expected 1 deletion, got %d", d2.Summary.Deletions)
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

	if d.Summary.Modified != 1 {
		t.Errorf("expected 1 modified, got %d", d.Summary.Modified)
	}
	if d.Summary.Deletions != 1 {
		t.Errorf("expected 1 deletion, got %d", d.Summary.Deletions)
	}
	if d.Summary.Additions != 1 {
		t.Errorf("expected 1 addition, got %d", d.Summary.Additions)
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

func TestBuild_BothNil(t *testing.T) {
	d := Build(nil, nil)

	if len(d.Rows) != 0 {
		t.Errorf("expected 0 rows, got %d", len(d.Rows))
	}
	if len(d.Hunks) != 0 {
		t.Errorf("expected 0 hunks, got %d", len(d.Hunks))
	}
	if d.Summary.Additions != 0 || d.Summary.Deletions != 0 || d.Summary.Modified != 0 {
		t.Errorf("expected zero stats, got %+v", d.Summary)
	}
	if d.MaxLineNum != 0 {
		t.Errorf("expected MaxLineNum 0, got %d", d.MaxLineNum)
	}
}

func TestBuild_BothEmpty(t *testing.T) {
	d := Build([]string{}, []string{})

	if len(d.Rows) != 0 {
		t.Errorf("expected 0 rows, got %d", len(d.Rows))
	}
	if len(d.Hunks) != 0 {
		t.Errorf("expected 0 hunks, got %d", len(d.Hunks))
	}
	if d.Summary.Additions != 0 || d.Summary.Deletions != 0 || d.Summary.Modified != 0 {
		t.Errorf("expected zero stats, got %+v", d.Summary)
	}
	if d.MaxLineNum != 0 {
		t.Errorf("expected MaxLineNum 0, got %d", d.MaxLineNum)
	}
}

func TestBuild_WordDiffComputed(t *testing.T) {
	old := []string{"hello world"}
	new := []string{"hello earth"}
	d := Build(old, new)

	if len(d.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(d.Rows))
	}
	r := d.Rows[0]
	if r.Type != RowModified {
		t.Fatalf("expected RowModified, got %d", r.Type)
	}
	if r.OldSpans == nil {
		t.Error("expected non-nil OldSpans for modified row")
	}
	if r.NewSpans == nil {
		t.Error("expected non-nil NewSpans for modified row")
	}

	// Verify unchanged rows do NOT get word diff spans.
	old2 := []string{"same", "changed"}
	new2 := []string{"same", "CHANGED"}
	d2 := Build(old2, new2)

	for _, r := range d2.Rows {
		if r.Type == RowUnchanged {
			if r.OldSpans != nil || r.NewSpans != nil {
				t.Error("unchanged row should have nil spans")
			}
		}
	}
}

func TestBuild_MaxLineNum(t *testing.T) {
	old := []string{"a", "b", "c"}
	new := []string{"a", "b", "c"}
	d := Build(old, new)

	if d.MaxLineNum != 3 {
		t.Errorf("expected MaxLineNum 3, got %d", d.MaxLineNum)
	}

	// When new side is longer, MaxLineNum should reflect it.
	d2 := Build([]string{"a"}, []string{"a", "b", "c", "d", "e"})
	if d2.MaxLineNum != 5 {
		t.Errorf("expected MaxLineNum 5, got %d", d2.MaxLineNum)
	}

	// When old side is longer, MaxLineNum should reflect it.
	d3 := Build([]string{"a", "b", "c", "d"}, []string{"a"})
	if d3.MaxLineNum != 4 {
		t.Errorf("expected MaxLineNum 4, got %d", d3.MaxLineNum)
	}
}

func TestBuild_StandaloneInsert(t *testing.T) {
	// Insert at the beginning (not preceded by deletes).
	old := []string{"a", "b"}
	new := []string{"x", "a", "b"}
	d := Build(old, new)

	foundAdded := false
	for _, r := range d.Rows {
		if r.Type == RowAdded && r.NewText == "x" {
			foundAdded = true
			if r.OldLineNum != 0 {
				t.Errorf("standalone insert should have OldLineNum 0, got %d", r.OldLineNum)
			}
		}
	}
	if !foundAdded {
		t.Error("expected to find a RowAdded for standalone insert 'x'")
	}
	if d.Summary.Additions < 1 {
		t.Errorf("expected at least 1 addition, got %d", d.Summary.Additions)
	}
}

func TestDetectHunks_EmptyRows(t *testing.T) {
	hunks := DetectHunks(nil, 3)
	if hunks != nil {
		t.Errorf("expected nil hunks, got %v", hunks)
	}

	hunks2 := DetectHunks([]Row{}, 3)
	if hunks2 != nil {
		t.Errorf("expected nil hunks for empty slice, got %v", hunks2)
	}
}

func TestDetectHunks_AllChanged(t *testing.T) {
	rows := []Row{
		{Type: RowModified},
		{Type: RowAdded},
		{Type: RowDeleted},
		{Type: RowModified},
	}
	hunks := DetectHunks(rows, 3)

	if len(hunks) != 1 {
		t.Fatalf("expected 1 hunk, got %d", len(hunks))
	}
	if hunks[0].StartIdx != 0 {
		t.Errorf("expected StartIdx 0, got %d", hunks[0].StartIdx)
	}
	if hunks[0].EndIdx != 4 {
		t.Errorf("expected EndIdx 4, got %d", hunks[0].EndIdx)
	}
}

func TestDetectHunks_ContextZero(t *testing.T) {
	rows := []Row{
		{Type: RowUnchanged},
		{Type: RowUnchanged},
		{Type: RowModified},
		{Type: RowUnchanged},
		{Type: RowUnchanged},
		{Type: RowUnchanged},
		{Type: RowUnchanged},
		{Type: RowDeleted},
		{Type: RowUnchanged},
	}
	hunks := DetectHunks(rows, 0)

	if len(hunks) != 2 {
		t.Fatalf("expected 2 hunks with zero context, got %d", len(hunks))
	}
	// First hunk covers only the modified row.
	if hunks[0].StartIdx != 2 || hunks[0].EndIdx != 3 {
		t.Errorf("hunk[0]: expected [2,3), got [%d,%d)", hunks[0].StartIdx, hunks[0].EndIdx)
	}
	// Second hunk covers only the deleted row.
	if hunks[1].StartIdx != 7 || hunks[1].EndIdx != 8 {
		t.Errorf("hunk[1]: expected [7,8), got [%d,%d)", hunks[1].StartIdx, hunks[1].EndIdx)
	}
}
