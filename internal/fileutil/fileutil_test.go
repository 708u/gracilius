package fileutil

import (
	"strings"
	"testing"
)

func TestIsBinary(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{
			name: "null byte in content",
			data: []byte("hello\x00world"),
			want: true,
		},
		{
			name: "text content",
			data: []byte("hello world\n"),
			want: false,
		},
		{
			name: "empty",
			data: []byte{},
			want: false,
		},
		{
			name: "null at boundary 8192",
			data: append(make([]byte, 8191), 0),
			want: true,
		},
		{
			name: "null beyond boundary 8192",
			data: func() []byte {
				b := make([]byte, 8293)
				for i := range 8192 {
					b[i] = 'a'
				}
				b[8192] = 0
				return b
			}(),
			want: false,
		},
		{
			name: "null at first byte",
			data: []byte{0, 'a', 'b'},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsBinary(tt.data)
			if got != tt.want {
				t.Fatalf("IsBinary() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSplitLines(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want []string
	}{
		{
			name: "LF",
			data: []byte("a\nb\nc\n"),
			want: []string{"a", "b", "c"},
		},
		{
			name: "CRLF",
			data: []byte("a\r\nb\r\nc\r\n"),
			want: []string{"a", "b", "c"},
		},
		{
			name: "bare CR not split",
			data: []byte("a\rb\rc\r"),
			want: []string{"a\rb\rc"},
		},
		{
			name: "empty",
			data: []byte{},
			want: nil,
		},
		{
			name: "no trailing newline",
			data: []byte("a\nb\nc"),
			want: []string{"a", "b", "c"},
		},
		{
			name: "single line no newline",
			data: []byte("hello"),
			want: []string{"hello"},
		},
		{
			name: "single line with newline",
			data: []byte("hello\n"),
			want: []string{"hello"},
		},
		{
			name: "mixed LF and CRLF",
			data: []byte("a\nb\r\nc"),
			want: []string{"a", "b", "c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SplitLines(tt.data)
			if len(got) != len(tt.want) {
				t.Fatalf("SplitLines() returned %d lines, want %d\ngot:  %q\nwant: %q",
					len(got), len(tt.want), got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("SplitLines()[%d] = %q, want %q",
						i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestSplitLines_LargeInput(t *testing.T) {
	// Ensure it works with content exceeding default scanner buffer.
	line := strings.Repeat("x", 100)
	var buf []byte
	for range 1000 {
		buf = append(buf, line...)
		buf = append(buf, '\n')
	}
	got := SplitLines(buf)
	if len(got) != 1000 {
		t.Fatalf("expected 1000 lines, got %d", len(got))
	}
}
