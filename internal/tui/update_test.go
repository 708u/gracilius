package tui

import "testing"

// mockServer implements MCPServer for testing.
type mockServer struct{}

func (mockServer) Port() int { return 0 }
func (mockServer) NotifySelectionChanged(string, string, int, int, int, int) {
}

// helper to set up a minimal Model with lines and cursor position.
func newTestModel(lines []string, cursorLine, cursorChar int) *Model {
	t := &tab{
		lines:      lines,
		cursorLine: cursorLine,
		cursorChar: cursorChar,
	}
	return &Model{
		tabs:      []*tab{t},
		activeTab: 0,
		focusPane: paneEditor,
		keys:      newKeyMap(),
		server:    mockServer{},
	}
}

func TestWordLeft(t *testing.T) {
	tests := []struct {
		name     string
		lines    []string
		line     int
		char     int
		wantLine int
		wantChar int
	}{
		{
			name:     "middle of word",
			lines:    []string{"hello world"},
			line:     0,
			char:     8,
			wantLine: 0,
			wantChar: 6,
		},
		{
			name:     "at word boundary",
			lines:    []string{"hello world"},
			line:     0,
			char:     6,
			wantLine: 0,
			wantChar: 0,
		},
		{
			name:     "separator boundary",
			lines:    []string{"foo.bar(baz)"},
			line:     0,
			char:     4,
			wantLine: 0,
			wantChar: 3,
		},
		{
			name:     "separator to word",
			lines:    []string{"foo.bar(baz)"},
			line:     0,
			char:     3,
			wantLine: 0,
			wantChar: 0,
		},
		{
			name:     "paren to word",
			lines:    []string{"foo.bar(baz)"},
			line:     0,
			char:     12,
			wantLine: 0,
			wantChar: 11,
		},
		{
			name:     "CJK treated as word",
			lines:    []string{"hello世界test"},
			line:     0,
			char:     10,
			wantLine: 0,
			wantChar: 0,
		},
		{
			name:     "line start wraps to previous line end",
			lines:    []string{"hello", "world"},
			line:     1,
			char:     0,
			wantLine: 0,
			wantChar: 5,
		},
		{
			name:     "file start stays at 0",
			lines:    []string{"hello"},
			line:     0,
			char:     0,
			wantLine: 0,
			wantChar: 0,
		},
		{
			name:     "skip whitespace then stop at word",
			lines:    []string{"  hello"},
			line:     0,
			char:     7,
			wantLine: 0,
			wantChar: 2,
		},
		{
			name:     "all whitespace wraps to previous line",
			lines:    []string{"hello", "   "},
			line:     1,
			char:     3,
			wantLine: 0,
			wantChar: 5,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestModel(tc.lines, tc.line, tc.char)
			m.moveWordLeft()
			tab := m.tabs[0]
			if tab.cursorLine != tc.wantLine || tab.cursorChar != tc.wantChar {
				t.Errorf("got (%d, %d), want (%d, %d)",
					tab.cursorLine, tab.cursorChar,
					tc.wantLine, tc.wantChar)
			}
		})
	}
}

func TestWordRight(t *testing.T) {
	tests := []struct {
		name     string
		lines    []string
		line     int
		char     int
		wantLine int
		wantChar int
	}{
		{
			name:     "move to next word",
			lines:    []string{"hello world"},
			line:     0,
			char:     0,
			wantLine: 0,
			wantChar: 6,
		},
		{
			name:     "from space to next word",
			lines:    []string{"hello world"},
			line:     0,
			char:     5,
			wantLine: 0,
			wantChar: 6,
		},
		{
			name:     "separator stops",
			lines:    []string{"foo.bar(baz)"},
			line:     0,
			char:     0,
			wantLine: 0,
			wantChar: 3,
		},
		{
			name:     "dot to next word",
			lines:    []string{"foo.bar(baz)"},
			line:     0,
			char:     3,
			wantLine: 0,
			wantChar: 4,
		},
		{
			name:     "paren to word",
			lines:    []string{"foo.bar(baz)"},
			line:     0,
			char:     7,
			wantLine: 0,
			wantChar: 8,
		},
		{
			name:     "CJK treated as word",
			lines:    []string{"hello世界test"},
			line:     0,
			char:     0,
			wantLine: 0,
			wantChar: 11,
		},
		{
			name:     "line end wraps to next line start",
			lines:    []string{"hello", "world"},
			line:     0,
			char:     5,
			wantLine: 1,
			wantChar: 0,
		},
		{
			name:     "file end stays",
			lines:    []string{"hello"},
			line:     0,
			char:     5,
			wantLine: 0,
			wantChar: 5,
		},
		{
			name:     "empty line wraps to next",
			lines:    []string{"", "world"},
			line:     0,
			char:     0,
			wantLine: 1,
			wantChar: 0,
		},
		{
			name:     "trailing space wraps to next line",
			lines:    []string{"hello   ", "world"},
			line:     0,
			char:     5,
			wantLine: 1,
			wantChar: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestModel(tc.lines, tc.line, tc.char)
			m.moveWordRight()
			tab := m.tabs[0]
			if tab.cursorLine != tc.wantLine || tab.cursorChar != tc.wantChar {
				t.Errorf("got (%d, %d), want (%d, %d)",
					tab.cursorLine, tab.cursorChar,
					tc.wantLine, tc.wantChar)
			}
		})
	}
}

func TestRuneClass(t *testing.T) {
	tests := []struct {
		r    rune
		want charClass
	}{
		{' ', classSpace},
		{'\t', classSpace},
		{'a', classWord},
		{'Z', classWord},
		{'0', classWord},
		{'_', classWord},
		{'世', classWord},
		{'.', classSep},
		{'(', classSep},
		{'!', classSep},
	}

	for _, tc := range tests {
		got := runeClass(tc.r)
		if got != tc.want {
			t.Errorf("runeClass(%q) = %d, want %d", tc.r, got, tc.want)
		}
	}
}
