package tui

import (
	"strings"

	diffmatchpatch "github.com/sergi/go-diff/diffmatchpatch"
)

type diffOp int

const (
	diffOpEqual diffOp = iota
	diffOpInsert
	diffOpDelete
)

type wordSpan struct {
	text string
	op   diffOp
}

// tokenize splits a string into tokens at whitespace/non-whitespace boundaries.
// Concatenating all returned tokens reproduces the original string.
func tokenize(s string) []string {
	if len(s) == 0 {
		return nil
	}

	var tokens []string
	runes := []rune(s)
	start := 0
	inSpace := isSpaceRune(runes[0])

	for i := 1; i < len(runes); i++ {
		sp := isSpaceRune(runes[i])
		if sp != inSpace {
			tokens = append(tokens, string(runes[start:i]))
			start = i
			inSpace = sp
		}
	}
	tokens = append(tokens, string(runes[start:]))
	return tokens
}

func isSpaceRune(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r'
}

// computeWordDiff computes word-level diffs between oldLine and newLine.
// It returns spans for both the old and new sides, where each span is
// tagged with its diff operation (equal, insert, or delete).
func computeWordDiff(oldLine, newLine string) (oldSpans []wordSpan, newSpans []wordSpan) {
	oldTokens := tokenize(oldLine)
	newTokens := tokenize(newLine)

	dmp := diffmatchpatch.New()

	oldText := strings.Join(oldTokens, "\n")
	newText := strings.Join(newTokens, "\n")

	chars1, chars2, lines := dmp.DiffLinesToRunes(oldText, newText)
	diffs := dmp.DiffMainRunes(chars1, chars2, false)
	diffs = dmp.DiffCleanupSemantic(diffs)
	diffs = dmp.DiffCharsToLines(diffs, lines)

	for _, d := range diffs {
		// DiffCharsToLines restores newlines that were used as delimiters;
		// remove them to get the original token text back.
		text := strings.ReplaceAll(d.Text, "\n", "")
		if text == "" {
			continue
		}
		switch d.Type {
		case diffmatchpatch.DiffEqual:
			oldSpans = append(oldSpans, wordSpan{text: text, op: diffOpEqual})
			newSpans = append(newSpans, wordSpan{text: text, op: diffOpEqual})
		case diffmatchpatch.DiffDelete:
			oldSpans = append(oldSpans, wordSpan{text: text, op: diffOpDelete})
		case diffmatchpatch.DiffInsert:
			newSpans = append(newSpans, wordSpan{text: text, op: diffOpInsert})
		}
	}
	return oldSpans, newSpans
}
