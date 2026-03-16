package tui

import (
	"testing"

	"github.com/708u/gracilius/internal/diff"
)

func TestComputeSearchMatches(t *testing.T) {
	t.Parallel()

	lines := []string{
		"Hello world",
		"hello World",
		"HELLO WORLD",
		"foobar foo",
	}

	tests := []struct {
		name       string
		query      string
		wantCount  int
		verifyEach func(t *testing.T, matches []searchMatch)
	}{
		{
			name:      "basic case insensitive",
			query:     "hello",
			wantCount: 3,
			verifyEach: func(t *testing.T, matches []searchMatch) {
				t.Helper()
				for i, m := range matches {
					if m.line != i {
						t.Errorf("match %d: expected line %d, got %d", i, i, m.line)
					}
					if m.startChar != 0 || m.endChar != 5 {
						t.Errorf("match %d: expected [0,5), got [%d,%d)", i, m.startChar, m.endChar)
					}
				}
			},
		},
		{
			name:      "smartcase sensitive",
			query:     "Hello",
			wantCount: 1,
			verifyEach: func(t *testing.T, matches []searchMatch) {
				t.Helper()
				if matches[0].line != 0 {
					t.Errorf("expected line 0, got %d", matches[0].line)
				}
			},
		},
		{
			name:      "multiple matches per line",
			query:     "foo",
			wantCount: 2,
			verifyEach: func(t *testing.T, matches []searchMatch) {
				t.Helper()
				if matches[0].startChar != 0 || matches[0].endChar != 3 {
					t.Errorf("first match: expected [0,3), got [%d,%d)", matches[0].startChar, matches[0].endChar)
				}
				if matches[1].startChar != 7 || matches[1].endChar != 10 {
					t.Errorf("second match: expected [7,10), got [%d,%d)", matches[1].startChar, matches[1].endChar)
				}
			},
		},
		{
			name:      "empty query",
			query:     "",
			wantCount: -1, // expect nil
		},
		{
			name:      "no matches",
			query:     "zzz",
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			matches := computeSearchMatches(lines, tt.query)
			if tt.wantCount == -1 {
				if matches != nil {
					t.Fatalf("expected nil, got %v", matches)
				}
				return
			}
			if len(matches) != tt.wantCount {
				t.Fatalf("expected %d matches, got %d", tt.wantCount, len(matches))
			}
			if tt.verifyEach != nil {
				tt.verifyEach(t, matches)
			}
		})
	}
}

func TestComputeDiffSearchMatches(t *testing.T) {
	t.Parallel()
	data := &diff.Data{
		Rows: []diff.Row{
			{OldLineNum: 1, NewLineNum: 1, OldText: "hello world", NewText: "hello world", Type: diff.RowUnchanged},
			{OldLineNum: 2, NewLineNum: 0, OldText: "old line", Type: diff.RowDeleted},
			{OldLineNum: 0, NewLineNum: 2, NewText: "new line hello", Type: diff.RowAdded},
		},
	}

	matches := computeDiffSearchMatches(data, "hello")
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
	t.Parallel()
	matches := computeDiffSearchMatches(nil, "hello")
	if matches != nil {
		t.Fatalf("expected nil, got %v", matches)
	}
}

func TestIsSmartCaseSensitive(t *testing.T) {
	t.Parallel()
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
		t.Run(tt.query, func(t *testing.T) {
			t.Parallel()
			got := isSmartCaseSensitive(tt.query)
			if got != tt.want {
				t.Errorf("isSmartCaseSensitive(%q) = %v, want %v", tt.query, got, tt.want)
			}
		})
	}
}
