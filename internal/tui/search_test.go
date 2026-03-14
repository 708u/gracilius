package tui

import "testing"

func TestComputeSearchMatches(t *testing.T) {
	lines := []string{
		"Hello world",
		"hello World",
		"HELLO WORLD",
		"foobar foo",
	}

	t.Run("basic case insensitive", func(t *testing.T) {
		matches := computeSearchMatches(lines, "hello")
		// "hello" is all lowercase → case insensitive → matches lines 0,1,2
		if len(matches) != 3 {
			t.Fatalf("expected 3 matches, got %d", len(matches))
		}
		for i, m := range matches {
			if m.line != i {
				t.Errorf("match %d: expected line %d, got %d", i, i, m.line)
			}
			if m.startChar != 0 || m.endChar != 5 {
				t.Errorf("match %d: expected [0,5), got [%d,%d)", i, m.startChar, m.endChar)
			}
		}
	})

	t.Run("smartcase sensitive", func(t *testing.T) {
		matches := computeSearchMatches(lines, "Hello")
		// "Hello" has uppercase → case sensitive → only line 0
		if len(matches) != 1 {
			t.Fatalf("expected 1 match, got %d", len(matches))
		}
		if matches[0].line != 0 {
			t.Errorf("expected line 0, got %d", matches[0].line)
		}
	})

	t.Run("multiple matches per line", func(t *testing.T) {
		matches := computeSearchMatches(lines, "foo")
		// line 3 "foobar foo" → matches at 0 and 7
		if len(matches) != 2 {
			t.Fatalf("expected 2 matches, got %d", len(matches))
		}
		if matches[0].startChar != 0 || matches[0].endChar != 3 {
			t.Errorf("first match: expected [0,3), got [%d,%d)", matches[0].startChar, matches[0].endChar)
		}
		if matches[1].startChar != 7 || matches[1].endChar != 10 {
			t.Errorf("second match: expected [7,10), got [%d,%d)", matches[1].startChar, matches[1].endChar)
		}
	})

	t.Run("empty query", func(t *testing.T) {
		matches := computeSearchMatches(lines, "")
		if matches != nil {
			t.Fatalf("expected nil, got %v", matches)
		}
	})

	t.Run("no matches", func(t *testing.T) {
		matches := computeSearchMatches(lines, "zzz")
		if len(matches) != 0 {
			t.Fatalf("expected 0 matches, got %d", len(matches))
		}
	})
}

func TestComputeDiffSearchMatches(t *testing.T) {
	data := &diffData{
		rows: []diffRow{
			{oldLineNum: 1, newLineNum: 1, oldText: "hello world", newText: "hello world", rowType: diffRowUnchanged},
			{oldLineNum: 2, newLineNum: 0, oldText: "old line", rowType: diffRowDeleted},
			{oldLineNum: 0, newLineNum: 2, newText: "new line hello", rowType: diffRowAdded},
		},
	}

	matches := computeDiffSearchMatches(data, "hello")
	// row 0: old "hello" at 0, new "hello" at 0
	// row 1: old "old line" → no match
	// row 2: new "new line hello" → "hello" at 9
	if len(matches) != 3 {
		t.Fatalf("expected 3 matches, got %d", len(matches))
	}

	// row 0, old side
	if matches[0].rowIdx != 0 || !matches[0].isOld || matches[0].startChar != 0 {
		t.Errorf("match 0: got row=%d isOld=%v start=%d", matches[0].rowIdx, matches[0].isOld, matches[0].startChar)
	}
	// row 0, new side
	if matches[1].rowIdx != 0 || matches[1].isOld || matches[1].startChar != 0 {
		t.Errorf("match 1: got row=%d isOld=%v start=%d", matches[1].rowIdx, matches[1].isOld, matches[1].startChar)
	}
	// row 2, new side
	if matches[2].rowIdx != 2 || matches[2].isOld || matches[2].startChar != 9 {
		t.Errorf("match 2: got row=%d isOld=%v start=%d", matches[2].rowIdx, matches[2].isOld, matches[2].startChar)
	}
}

func TestComputeDiffSearchMatches_nil(t *testing.T) {
	matches := computeDiffSearchMatches(nil, "hello")
	if matches != nil {
		t.Fatalf("expected nil, got %v", matches)
	}
}

func TestIsSmartCaseSensitive(t *testing.T) {
	tests := []struct {
		query string
		want  bool
	}{
		{"hello", false},
		{"Hello", true},
		{"HELLO", true},
		{"hello world", false},
		{"Hello World", true},
		{"", false},
		{"123", false},
		{"heLLo", true},
	}

	for _, tt := range tests {
		got := isSmartCaseSensitive(tt.query)
		if got != tt.want {
			t.Errorf("isSmartCaseSensitive(%q) = %v, want %v", tt.query, got, tt.want)
		}
	}
}
