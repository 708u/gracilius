package tui

import (
	"testing"
)

const testBlock = "\u2588"

func TestRenderScrollbar_AllVisible(t *testing.T) {
	col := renderScrollbar(10, 5, 0, testBlock)
	for i, c := range col {
		if c != " " {
			t.Errorf("row %d: expected space, got %q", i, c)
		}
	}
}

func TestRenderScrollbar_OffsetZero(t *testing.T) {
	col := renderScrollbar(10, 100, 0, testBlock)
	if col[0] == " " {
		t.Error("expected thumb at row 0 when offset=0")
	}
}

func TestRenderScrollbar_OffsetMax(t *testing.T) {
	height := 10
	total := 100
	maxOffset := total - height
	col := renderScrollbar(height, total, maxOffset, testBlock)
	if col[height-1] == " " {
		t.Errorf("expected thumb at last row when offset=max")
	}
}

func TestRenderScrollbar_ZeroTotal(t *testing.T) {
	col := renderScrollbar(10, 0, 0, testBlock)
	if len(col) != 10 {
		t.Errorf("expected 10 rows, got %d", len(col))
	}
}

func TestRenderScrollbar_ThumbMinSize(t *testing.T) {
	col := renderScrollbar(5, 10000, 0, testBlock)
	thumbCount := 0
	for _, c := range col {
		if c != " " {
			thumbCount++
		}
	}
	if thumbCount < 1 {
		t.Error("thumb size should be at least 1")
	}
}
