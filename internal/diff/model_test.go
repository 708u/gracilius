package diff

import (
	"testing"
)

func TestBuild(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		old    []string
		new    []string
		verify func(t *testing.T, d *Data)
	}{
		{
			name: "Identical",
			old:  []string{"aaa", "bbb", "ccc"},
			new:  []string{"aaa", "bbb", "ccc"},
			verify: func(t *testing.T, d *Data) {
				t.Helper()
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
			},
		},
		{
			name: "AllAdded",
			old:  nil,
			new:  []string{"aaa", "bbb"},
			verify: func(t *testing.T, d *Data) {
				t.Helper()
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
			},
		},
		{
			name: "AllDeleted",
			old:  []string{"aaa", "bbb"},
			new:  nil,
			verify: func(t *testing.T, d *Data) {
				t.Helper()
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
			},
		},
		{
			name: "SingleModification",
			old:  []string{"aaa", "bbb", "ccc"},
			new:  []string{"aaa", "BBB", "ccc"},
			verify: func(t *testing.T, d *Data) {
				t.Helper()
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
			},
		},
		{
			name: "EvenPairing",
			old:  []string{"aaa", "bbb", "ccc", "ddd"},
			new:  []string{"aaa", "BBB", "CCC", "ddd"},
			verify: func(t *testing.T, d *Data) {
				t.Helper()
				if d.Summary.Modified != 2 {
					t.Errorf("expected 2 modified, got %d", d.Summary.Modified)
				}
				if d.Summary.Additions != 0 {
					t.Errorf("expected 0 additions, got %d", d.Summary.Additions)
				}
				if d.Summary.Deletions != 0 {
					t.Errorf("expected 0 deletions, got %d", d.Summary.Deletions)
				}
			},
		},
		{
			name: "UnevenPairing",
			old:  []string{"aaa", "bbb", "ccc", "ddd", "eee"},
			new:  []string{"aaa", "BBB", "CCC", "eee"},
			verify: func(t *testing.T, d *Data) {
				t.Helper()
				if d.Summary.Modified != 2 {
					t.Errorf("expected 2 modified, got %d", d.Summary.Modified)
				}
				if d.Summary.Deletions != 1 {
					t.Errorf("expected 1 deletion, got %d", d.Summary.Deletions)
				}
			},
		},
		{
			name: "Stats",
			old:  []string{"aaa", "bbb", "ccc", "ddd", "eee"},
			new:  []string{"aaa", "BBB", "ddd", "fff", "eee"},
			verify: func(t *testing.T, d *Data) {
				t.Helper()
				if d.Summary.Modified != 1 {
					t.Errorf("expected 1 modified, got %d", d.Summary.Modified)
				}
				if d.Summary.Deletions != 1 {
					t.Errorf("expected 1 deletion, got %d", d.Summary.Deletions)
				}
				if d.Summary.Additions != 1 {
					t.Errorf("expected 1 addition, got %d", d.Summary.Additions)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			d := Build(tt.old, tt.new)
			tt.verify(t, d)
		})
	}
}

func TestBuildDiffData_LineNumbers(t *testing.T) {
	t.Parallel()

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

func verifyEmptyBuild(t *testing.T, d *Data) {
	t.Helper()
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

func TestBuild_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		old    []string
		new    []string
		verify func(t *testing.T, d *Data)
	}{
		{
			name:   "BothNil",
			old:    nil,
			new:    nil,
			verify: verifyEmptyBuild,
		},
		{
			name:   "BothEmpty",
			old:    []string{},
			new:    []string{},
			verify: verifyEmptyBuild,
		},
		{
			name: "WordDiffComputed",
			old:  []string{"hello world"},
			new:  []string{"hello earth"},
			verify: func(t *testing.T, d *Data) {
				t.Helper()
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
			},
		},
		{
			name: "UnchangedRowsHaveNilSpans",
			old:  []string{"same", "changed"},
			new:  []string{"same", "CHANGED"},
			verify: func(t *testing.T, d *Data) {
				t.Helper()
				for _, r := range d.Rows {
					if r.Type == RowUnchanged {
						if r.OldSpans != nil || r.NewSpans != nil {
							t.Error("unchanged row should have nil spans")
						}
					}
				}
			},
		},
		{
			name: "MaxLineNum/equal",
			old:  []string{"a", "b", "c"},
			new:  []string{"a", "b", "c"},
			verify: func(t *testing.T, d *Data) {
				t.Helper()
				if d.MaxLineNum != 3 {
					t.Errorf("expected MaxLineNum 3, got %d", d.MaxLineNum)
				}
			},
		},
		{
			name: "MaxLineNum/new_longer",
			old:  []string{"a"},
			new:  []string{"a", "b", "c", "d", "e"},
			verify: func(t *testing.T, d *Data) {
				t.Helper()
				if d.MaxLineNum != 5 {
					t.Errorf("expected MaxLineNum 5, got %d", d.MaxLineNum)
				}
			},
		},
		{
			name: "MaxLineNum/old_longer",
			old:  []string{"a", "b", "c", "d"},
			new:  []string{"a"},
			verify: func(t *testing.T, d *Data) {
				t.Helper()
				if d.MaxLineNum != 4 {
					t.Errorf("expected MaxLineNum 4, got %d", d.MaxLineNum)
				}
			},
		},
		{
			name: "StandaloneInsert",
			old:  []string{"a", "b"},
			new:  []string{"x", "a", "b"},
			verify: func(t *testing.T, d *Data) {
				t.Helper()
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
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			d := Build(tt.old, tt.new)
			tt.verify(t, d)
		})
	}
}

func TestDetectHunks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		setup  func() []Hunk
		verify func(t *testing.T, hunks []Hunk)
	}{
		{
			name: "SingleChange",
			setup: func() []Hunk {
				old := []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10"}
				new := []string{"1", "2", "3", "4", "X", "6", "7", "8", "9", "10"}
				d := Build(old, new)
				return d.Hunks
			},
			verify: func(t *testing.T, hunks []Hunk) {
				t.Helper()
				if len(hunks) != 1 {
					t.Fatalf("expected 1 hunk, got %d", len(hunks))
				}
				h := hunks[0]
				if h.StartIdx != 1 {
					t.Errorf("expected StartIdx 1, got %d", h.StartIdx)
				}
				if h.EndIdx != 8 {
					t.Errorf("expected EndIdx 8, got %d", h.EndIdx)
				}
			},
		},
		{
			name: "MergedHunks",
			setup: func() []Hunk {
				old := []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10"}
				new := []string{"1", "X", "3", "4", "5", "6", "Y", "8", "9", "10"}
				d := Build(old, new)
				return d.Hunks
			},
			verify: func(t *testing.T, hunks []Hunk) {
				t.Helper()
				if len(hunks) != 1 {
					t.Fatalf("expected 1 merged hunk, got %d", len(hunks))
				}
			},
		},
		{
			name: "SeparateHunks",
			setup: func() []Hunk {
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
				return d.Hunks
			},
			verify: func(t *testing.T, hunks []Hunk) {
				t.Helper()
				if len(hunks) != 2 {
					t.Fatalf("expected 2 separate hunks, got %d", len(hunks))
				}
				if hunks[0].EndIdx > hunks[1].StartIdx {
					t.Error("hunks should not overlap")
				}
			},
		},
		{
			name: "NoChanges",
			setup: func() []Hunk {
				lines := []string{"aaa", "bbb", "ccc"}
				d := Build(lines, lines)
				return d.Hunks
			},
			verify: func(t *testing.T, hunks []Hunk) {
				t.Helper()
				if len(hunks) != 0 {
					t.Fatalf("expected 0 hunks, got %d", len(hunks))
				}
			},
		},
		{
			name: "AllChanged",
			setup: func() []Hunk {
				rows := []Row{
					{Type: RowModified},
					{Type: RowAdded},
					{Type: RowDeleted},
					{Type: RowModified},
				}
				return DetectHunks(rows, 3)
			},
			verify: func(t *testing.T, hunks []Hunk) {
				t.Helper()
				if len(hunks) != 1 {
					t.Fatalf("expected 1 hunk, got %d", len(hunks))
				}
				if hunks[0].StartIdx != 0 {
					t.Errorf("expected StartIdx 0, got %d", hunks[0].StartIdx)
				}
				if hunks[0].EndIdx != 4 {
					t.Errorf("expected EndIdx 4, got %d", hunks[0].EndIdx)
				}
			},
		},
		{
			name: "ContextZero",
			setup: func() []Hunk {
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
				return DetectHunks(rows, 0)
			},
			verify: func(t *testing.T, hunks []Hunk) {
				t.Helper()
				if len(hunks) != 2 {
					t.Fatalf("expected 2 hunks with zero context, got %d", len(hunks))
				}
				if hunks[0].StartIdx != 2 || hunks[0].EndIdx != 3 {
					t.Errorf("hunk[0]: expected [2,3), got [%d,%d)", hunks[0].StartIdx, hunks[0].EndIdx)
				}
				if hunks[1].StartIdx != 7 || hunks[1].EndIdx != 8 {
					t.Errorf("hunk[1]: expected [7,8), got [%d,%d)", hunks[1].StartIdx, hunks[1].EndIdx)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			hunks := tt.setup()
			tt.verify(t, hunks)
		})
	}
}

func TestDetectHunks_EmptyRows(t *testing.T) {
	t.Parallel()

	hunks := DetectHunks(nil, 3)
	if hunks != nil {
		t.Errorf("expected nil hunks, got %v", hunks)
	}

	hunks2 := DetectHunks([]Row{}, 3)
	if hunks2 != nil {
		t.Errorf("expected nil hunks for empty slice, got %v", hunks2)
	}
}
