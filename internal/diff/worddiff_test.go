package diff

import (
	"strings"
	"testing"
)

func TestComputeWordDiff(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		old    string
		new    string
		verify func(t *testing.T, oldSpans, newSpans []WordSpan)
	}{
		{
			name: "Identical",
			old:  "hello world",
			new:  "hello world",
			verify: func(t *testing.T, oldSpans, newSpans []WordSpan) {
				t.Helper()
				for i, s := range oldSpans {
					if s.Op != OpEqual {
						t.Errorf("oldSpans[%d]: expected OpEqual, got %d", i, s.Op)
					}
				}
				for i, s := range newSpans {
					if s.Op != OpEqual {
						t.Errorf("newSpans[%d]: expected OpEqual, got %d", i, s.Op)
					}
				}
			},
		},
		{
			name: "VariableRename",
			old:  "x := foo + bar",
			new:  "x := baz + bar",
			verify: func(t *testing.T, oldSpans, newSpans []WordSpan) {
				t.Helper()
				if !containsSpan(oldSpans, "foo", OpDelete) {
					t.Fatal("expected oldSpans to contain 'foo' as delete")
				}
				if !containsSpan(newSpans, "baz", OpInsert) {
					t.Fatal("expected newSpans to contain 'baz' as insert")
				}
			},
		},
		{
			name: "InsertedWord",
			old:  "return value",
			new:  "return new value",
			verify: func(t *testing.T, _, newSpans []WordSpan) {
				t.Helper()
				if !containsSpan(newSpans, "new", OpInsert) {
					t.Fatal("expected newSpans to contain 'new' as insert")
				}
			},
		},
		{
			name: "DeletedWord",
			old:  "return old value",
			new:  "return value",
			verify: func(t *testing.T, oldSpans, _ []WordSpan) {
				t.Helper()
				if !containsSpan(oldSpans, "old", OpDelete) {
					t.Fatal("expected oldSpans to contain 'old' as delete")
				}
			},
		},
		{
			name: "EmptyOld",
			old:  "",
			new:  "hello world",
			verify: func(t *testing.T, oldSpans, newSpans []WordSpan) {
				t.Helper()
				if len(oldSpans) != 0 {
					t.Fatalf("expected 0 oldSpans, got %d", len(oldSpans))
				}
				if len(newSpans) == 0 {
					t.Fatal("expected non-empty newSpans")
				}
				for _, s := range newSpans {
					if s.Op != OpInsert {
						t.Errorf("expected all newSpans to be insert, got %d", s.Op)
					}
				}
			},
		},
		{
			name: "EmptyNew",
			old:  "hello world",
			new:  "",
			verify: func(t *testing.T, oldSpans, newSpans []WordSpan) {
				t.Helper()
				if len(newSpans) != 0 {
					t.Fatalf("expected 0 newSpans, got %d", len(newSpans))
				}
				if len(oldSpans) == 0 {
					t.Fatal("expected non-empty oldSpans")
				}
				for _, s := range oldSpans {
					if s.Op != OpDelete {
						t.Errorf("expected all oldSpans to be delete, got %d", s.Op)
					}
				}
			},
		},
		{
			name: "EmptyStrings",
			old:  "",
			new:  "",
			verify: func(t *testing.T, oldSpans, newSpans []WordSpan) {
				t.Helper()
				if len(oldSpans) != 0 {
					t.Errorf("expected 0 oldSpans, got %d", len(oldSpans))
				}
				if len(newSpans) != 0 {
					t.Errorf("expected 0 newSpans, got %d", len(newSpans))
				}
			},
		},
		{
			name: "WhitespaceOnly",
			old:  "  ",
			new:  "\t",
			verify: func(t *testing.T, oldSpans, newSpans []WordSpan) {
				t.Helper()
				oldJoined := joinSpans(oldSpans)
				newJoined := joinSpans(newSpans)
				if oldJoined != "  " {
					t.Errorf("old round-trip: expected %q, got %q", "  ", oldJoined)
				}
				if newJoined != "\t" {
					t.Errorf("new round-trip: expected %q, got %q", "\t", newJoined)
				}
				hasDelete := false
				for _, s := range oldSpans {
					if s.Op == OpDelete {
						hasDelete = true
					}
				}
				hasInsert := false
				for _, s := range newSpans {
					if s.Op == OpInsert {
						hasInsert = true
					}
				}
				if !hasDelete {
					t.Error("expected old side to have a delete span")
				}
				if !hasInsert {
					t.Error("expected new side to have an insert span")
				}
			},
		},
		{
			name: "WhitespaceChange",
			old:  "a  b",
			new:  "a b",
			verify: func(t *testing.T, oldSpans, newSpans []WordSpan) {
				t.Helper()
				oldJoined := joinSpans(oldSpans)
				newJoined := joinSpans(newSpans)
				if oldJoined != "a  b" {
					t.Fatalf("oldSpans round-trip: expected %q, got %q", "a  b", oldJoined)
				}
				if newJoined != "a b" {
					t.Fatalf("newSpans round-trip: expected %q, got %q", "a b", newJoined)
				}
				hasChange := false
				for _, s := range oldSpans {
					if s.Op == OpDelete {
						hasChange = true
					}
				}
				for _, s := range newSpans {
					if s.Op == OpInsert {
						hasChange = true
					}
				}
				if !hasChange {
					t.Fatal("expected whitespace change to produce non-equal spans")
				}
			},
		},
		{
			name: "IndentChange",
			old:  "\tfoo",
			new:  "\t\tfoo",
			verify: func(t *testing.T, oldSpans, newSpans []WordSpan) {
				t.Helper()
				oldJoined := joinSpans(oldSpans)
				newJoined := joinSpans(newSpans)
				if oldJoined != "\tfoo" {
					t.Fatalf("oldSpans round-trip: expected %q, got %q", "\tfoo", oldJoined)
				}
				if newJoined != "\t\tfoo" {
					t.Fatalf("newSpans round-trip: expected %q, got %q", "\t\tfoo", newJoined)
				}
			},
		},
		{
			name: "CJK_identical",
			old:  "hello world",
			new:  "hello world",
			verify: func(t *testing.T, oldSpans, newSpans []WordSpan) {
				t.Helper()
				oldJoined := joinSpans(oldSpans)
				newJoined := joinSpans(newSpans)
				if oldJoined != "hello world" {
					t.Errorf("old round-trip: expected %q, got %q", "hello world", oldJoined)
				}
				if newJoined != "hello world" {
					t.Errorf("new round-trip: expected %q, got %q", "hello world", newJoined)
				}
			},
		},
		{
			name: "CJK_diff",
			old:  "func",
			new:  "func",
			verify: func(t *testing.T, oldSpans, newSpans []WordSpan) {
				t.Helper()
				oldJoined := joinSpansFiltered(oldSpans, OpInsert)
				newJoined := joinSpansFiltered(newSpans, OpDelete)
				if oldJoined != "func" {
					t.Errorf("CJK diff old round-trip: expected %q, got %q",
						"func", oldJoined)
				}
				if newJoined != "func" {
					t.Errorf("CJK diff new round-trip: expected %q, got %q",
						"func", newJoined)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			oldSpans, newSpans := ComputeWordDiff(tt.old, tt.new)
			tt.verify(t, oldSpans, newSpans)
		})
	}
}

func TestComputeWordDiff_RoundTrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		oldLine string
		newLine string
	}{
		{"simple rename", "x := foo + bar", "x := baz + bar"},
		{"insert word", "return value", "return new value"},
		{"delete word", "return old value", "return value"},
		{"indent change", "\tfoo", "\t\tfoo"},
		{"whitespace change", "a  b", "a b"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			oldSpans, newSpans := ComputeWordDiff(tt.oldLine, tt.newLine)

			oldReconstructed := joinSpansFiltered(oldSpans, OpInsert)
			if oldReconstructed != tt.oldLine {
				t.Errorf("old round-trip: expected %q, got %q", tt.oldLine, oldReconstructed)
			}

			newReconstructed := joinSpansFiltered(newSpans, OpDelete)
			if newReconstructed != tt.newLine {
				t.Errorf("new round-trip: expected %q, got %q", tt.newLine, newReconstructed)
			}
		})
	}
}

func TestComputeWordDiff_IdenticalStrings(t *testing.T) {
	t.Parallel()

	tests := []string{
		"hello world",
		"single",
		"  spaced  out  ",
		"\ttabbed",
	}
	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			t.Parallel()
			oldSpans, newSpans := ComputeWordDiff(input, input)

			for i, s := range oldSpans {
				if s.Op != OpEqual {
					t.Errorf("oldSpans[%d]: expected OpEqual, got %d", i, s.Op)
				}
			}
			for i, s := range newSpans {
				if s.Op != OpEqual {
					t.Errorf("newSpans[%d]: expected OpEqual, got %d", i, s.Op)
				}
			}

			oldJoined := joinSpans(oldSpans)
			newJoined := joinSpans(newSpans)
			if oldJoined != input {
				t.Errorf("old round-trip: expected %q, got %q", input, oldJoined)
			}
			if newJoined != input {
				t.Errorf("new round-trip: expected %q, got %q", input, newJoined)
			}
		})
	}
}

func TestTokenize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{"Basic", "hello world", []string{"hello", " ", "world"}},
		{"MultipleSpaces", "a  b", []string{"a", "  ", "b"}},
		{"LeadingWhitespace", "  hello", []string{"  ", "hello"}},
		{"Empty", "", nil},
		{"TabsAndSpaces", "\thello world", []string{"\t", "hello", " ", "world"}},
		{"SingleWord", "hello", []string{"hello"}},
		{"OnlySpaces", "   ", []string{"   "}},
		{"MixedWhitespace", "\t foo", []string{"\t ", "foo"}},
		{"MixedWhitespace_Separated", "\ta b", []string{"\t", "a", " ", "b"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := Tokenize(tt.input)
			assertTokens(t, got, tt.want)
		})
	}
}

// --- helpers ---

func containsSpan(spans []WordSpan, text string, op Op) bool {
	for _, s := range spans {
		if strings.Contains(s.Text, text) && s.Op == op {
			return true
		}
	}
	return false
}

func joinSpans(spans []WordSpan) string {
	var b strings.Builder
	for _, s := range spans {
		b.WriteString(s.Text)
	}
	return b.String()
}

func joinSpansFiltered(spans []WordSpan, excludeOp Op) string {
	var b strings.Builder
	for _, s := range spans {
		if s.Op != excludeOp {
			b.WriteString(s.Text)
		}
	}
	return b.String()
}

func assertTokens(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("expected %d tokens, got %d: %v", len(want), len(got), got)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("token[%d]: expected %q, got %q", i, want[i], got[i])
		}
	}
}
