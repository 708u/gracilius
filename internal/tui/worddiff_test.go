package tui

import (
	"strings"
	"testing"
)

func TestComputeWordDiff_Identical(t *testing.T) {
	old, new := computeWordDiff("hello world", "hello world")
	for i, s := range old {
		if s.op != diffOpEqual {
			t.Errorf("oldSpans[%d]: expected diffOpEqual, got %d", i, s.op)
		}
	}
	for i, s := range new {
		if s.op != diffOpEqual {
			t.Errorf("newSpans[%d]: expected diffOpEqual, got %d", i, s.op)
		}
	}
}

func TestComputeWordDiff_VariableRename(t *testing.T) {
	oldSpans, newSpans := computeWordDiff("x := foo + bar", "x := baz + bar")

	if !containsSpan(oldSpans, "foo", diffOpDelete) {
		t.Fatal("expected oldSpans to contain 'foo' as delete")
	}
	if !containsSpan(newSpans, "baz", diffOpInsert) {
		t.Fatal("expected newSpans to contain 'baz' as insert")
	}
}

func TestComputeWordDiff_InsertedWord(t *testing.T) {
	_, newSpans := computeWordDiff("return value", "return new value")

	if !containsSpan(newSpans, "new", diffOpInsert) {
		t.Fatal("expected newSpans to contain 'new' as insert")
	}
}

func TestComputeWordDiff_DeletedWord(t *testing.T) {
	oldSpans, _ := computeWordDiff("return old value", "return value")

	if !containsSpan(oldSpans, "old", diffOpDelete) {
		t.Fatal("expected oldSpans to contain 'old' as delete")
	}
}

func TestComputeWordDiff_WhitespaceChange(t *testing.T) {
	oldSpans, newSpans := computeWordDiff("a  b", "a b")

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
		if s.op == diffOpDelete {
			hasChange = true
		}
	}
	for _, s := range newSpans {
		if s.op == diffOpInsert {
			hasChange = true
		}
	}
	if !hasChange {
		t.Fatal("expected whitespace change to produce non-equal spans")
	}
}

func TestComputeWordDiff_EmptyOld(t *testing.T) {
	oldSpans, newSpans := computeWordDiff("", "hello world")

	if len(oldSpans) != 0 {
		t.Fatalf("expected 0 oldSpans, got %d", len(oldSpans))
	}
	if len(newSpans) == 0 {
		t.Fatal("expected non-empty newSpans")
	}
	for _, s := range newSpans {
		if s.op != diffOpInsert {
			t.Errorf("expected all newSpans to be insert, got %d", s.op)
		}
	}
}

func TestComputeWordDiff_EmptyNew(t *testing.T) {
	oldSpans, newSpans := computeWordDiff("hello world", "")

	if len(newSpans) != 0 {
		t.Fatalf("expected 0 newSpans, got %d", len(newSpans))
	}
	if len(oldSpans) == 0 {
		t.Fatal("expected non-empty oldSpans")
	}
	for _, s := range oldSpans {
		if s.op != diffOpDelete {
			t.Errorf("expected all oldSpans to be delete, got %d", s.op)
		}
	}
}

func TestComputeWordDiff_IndentChange(t *testing.T) {
	oldSpans, newSpans := computeWordDiff("\tfoo", "\t\tfoo")

	oldJoined := joinSpans(oldSpans)
	newJoined := joinSpans(newSpans)
	if oldJoined != "\tfoo" {
		t.Fatalf("oldSpans round-trip: expected %q, got %q", "\tfoo", oldJoined)
	}
	if newJoined != "\t\tfoo" {
		t.Fatalf("newSpans round-trip: expected %q, got %q", "\t\tfoo", newJoined)
	}
}

func TestComputeWordDiff_RoundTrip(t *testing.T) {
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
			oldSpans, newSpans := computeWordDiff(tt.oldLine, tt.newLine)

			oldReconstructed := joinSpansFiltered(oldSpans, diffOpInsert)
			if oldReconstructed != tt.oldLine {
				t.Errorf("old round-trip: expected %q, got %q", tt.oldLine, oldReconstructed)
			}

			newReconstructed := joinSpansFiltered(newSpans, diffOpDelete)
			if newReconstructed != tt.newLine {
				t.Errorf("new round-trip: expected %q, got %q", tt.newLine, newReconstructed)
			}
		})
	}
}

func TestTokenize_Basic(t *testing.T) {
	got := tokenize("hello world")
	want := []string{"hello", " ", "world"}
	assertTokens(t, got, want)
}

func TestTokenize_MultipleSpaces(t *testing.T) {
	got := tokenize("a  b")
	want := []string{"a", "  ", "b"}
	assertTokens(t, got, want)
}

func TestTokenize_LeadingWhitespace(t *testing.T) {
	got := tokenize("  hello")
	want := []string{"  ", "hello"}
	assertTokens(t, got, want)
}

func TestTokenize_Empty(t *testing.T) {
	got := tokenize("")
	if len(got) != 0 {
		t.Fatalf("expected empty slice, got %v", got)
	}
}

func TestTokenize_TabsAndSpaces(t *testing.T) {
	got := tokenize("\thello world")
	want := []string{"\t", "hello", " ", "world"}
	assertTokens(t, got, want)
}

// --- helpers ---

func containsSpan(spans []wordSpan, text string, op diffOp) bool {
	for _, s := range spans {
		if strings.Contains(s.text, text) && s.op == op {
			return true
		}
	}
	return false
}

func joinSpans(spans []wordSpan) string {
	var b strings.Builder
	for _, s := range spans {
		b.WriteString(s.text)
	}
	return b.String()
}

func joinSpansFiltered(spans []wordSpan, excludeOp diffOp) string {
	var b strings.Builder
	for _, s := range spans {
		if s.op != excludeOp {
			b.WriteString(s.text)
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
