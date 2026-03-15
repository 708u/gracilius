package diff

import (
	"strings"
	"testing"
)

func TestComputeWordDiff_Identical(t *testing.T) {
	old, new := ComputeWordDiff("hello world", "hello world")
	for i, s := range old {
		if s.Op != OpEqual {
			t.Errorf("oldSpans[%d]: expected OpEqual, got %d", i, s.Op)
		}
	}
	for i, s := range new {
		if s.Op != OpEqual {
			t.Errorf("newSpans[%d]: expected OpEqual, got %d", i, s.Op)
		}
	}
}

func TestComputeWordDiff_VariableRename(t *testing.T) {
	oldSpans, newSpans := ComputeWordDiff("x := foo + bar", "x := baz + bar")

	if !containsSpan(oldSpans, "foo", OpDelete) {
		t.Fatal("expected oldSpans to contain 'foo' as delete")
	}
	if !containsSpan(newSpans, "baz", OpInsert) {
		t.Fatal("expected newSpans to contain 'baz' as insert")
	}
}

func TestComputeWordDiff_InsertedWord(t *testing.T) {
	_, newSpans := ComputeWordDiff("return value", "return new value")

	if !containsSpan(newSpans, "new", OpInsert) {
		t.Fatal("expected newSpans to contain 'new' as insert")
	}
}

func TestComputeWordDiff_DeletedWord(t *testing.T) {
	oldSpans, _ := ComputeWordDiff("return old value", "return value")

	if !containsSpan(oldSpans, "old", OpDelete) {
		t.Fatal("expected oldSpans to contain 'old' as delete")
	}
}

func TestComputeWordDiff_WhitespaceChange(t *testing.T) {
	oldSpans, newSpans := ComputeWordDiff("a  b", "a b")

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
}

func TestComputeWordDiff_EmptyOld(t *testing.T) {
	oldSpans, newSpans := ComputeWordDiff("", "hello world")

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
}

func TestComputeWordDiff_EmptyNew(t *testing.T) {
	oldSpans, newSpans := ComputeWordDiff("hello world", "")

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
}

func TestComputeWordDiff_IndentChange(t *testing.T) {
	oldSpans, newSpans := ComputeWordDiff("\tfoo", "\t\tfoo")

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

func TestTokenize_Basic(t *testing.T) {
	got := Tokenize("hello world")
	want := []string{"hello", " ", "world"}
	assertTokens(t, got, want)
}

func TestTokenize_MultipleSpaces(t *testing.T) {
	got := Tokenize("a  b")
	want := []string{"a", "  ", "b"}
	assertTokens(t, got, want)
}

func TestTokenize_LeadingWhitespace(t *testing.T) {
	got := Tokenize("  hello")
	want := []string{"  ", "hello"}
	assertTokens(t, got, want)
}

func TestTokenize_Empty(t *testing.T) {
	got := Tokenize("")
	if len(got) != 0 {
		t.Fatalf("expected empty slice, got %v", got)
	}
}

func TestTokenize_TabsAndSpaces(t *testing.T) {
	got := Tokenize("\thello world")
	want := []string{"\t", "hello", " ", "world"}
	assertTokens(t, got, want)
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
