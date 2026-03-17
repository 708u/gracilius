package render

import (
	"reflect"
	"strings"
	"testing"
)

func TestTheme_AllFieldsNonEmpty(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		theme Theme
	}{
		{"Dark", Dark},
		{"Light", Light},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			v := reflect.ValueOf(tt.theme)
			typ := v.Type()
			for i := 0; i < v.NumField(); i++ {
				field := typ.Field(i)
				val := v.Field(i).String()
				if val == "" {
					t.Errorf("%s.%s is empty", tt.name, field.Name)
				}
			}
		})
	}
}

func TestTheme_SelectionBgSeq(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name  string
		theme Theme
	}{
		{"Dark", Dark},
		{"Light", Light},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			seq := tc.theme.SelectionBgSeq()
			if seq == "" {
				t.Error("SelectionBgSeq() returned empty string")
			}
			if !strings.Contains(seq, "\033[") {
				t.Errorf("expected ANSI CSI prefix, got %q", seq)
			}
			if !strings.HasSuffix(seq, "m") {
				t.Errorf("expected 'm' suffix, got %q", seq)
			}
		})
	}
}

func TestTheme_SearchMatchBgSeq(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name  string
		theme Theme
	}{
		{"Dark", Dark},
		{"Light", Light},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			seq := tc.theme.SearchMatchBgSeq()
			if seq == "" {
				t.Error("SearchMatchBgSeq() returned empty string")
			}
			if !strings.Contains(seq, "\033[") {
				t.Errorf("expected ANSI CSI prefix, got %q", seq)
			}
			if !strings.HasSuffix(seq, "m") {
				t.Errorf("expected 'm' suffix, got %q", seq)
			}
		})
	}
}

func TestTheme_SearchCurrentBgSeq(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name  string
		theme Theme
	}{
		{"Dark", Dark},
		{"Light", Light},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			seq := tc.theme.SearchCurrentBgSeq()
			if seq == "" {
				t.Error("SearchCurrentBgSeq() returned empty string")
			}
			if !strings.Contains(seq, "\033[") {
				t.Errorf("expected ANSI CSI prefix, got %q", seq)
			}
			if !strings.HasSuffix(seq, "m") {
				t.Errorf("expected 'm' suffix, got %q", seq)
			}
		})
	}
}
