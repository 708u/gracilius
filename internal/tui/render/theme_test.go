package render

import (
	"reflect"
	"strings"
	"testing"
)

func TestDarkTheme_AllFieldsNonEmpty(t *testing.T) {
	v := reflect.ValueOf(Dark)
	typ := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := typ.Field(i)
		val := v.Field(i).String()
		if val == "" {
			t.Errorf("Dark.%s is empty", field.Name)
		}
	}
}

func TestLightTheme_AllFieldsNonEmpty(t *testing.T) {
	v := reflect.ValueOf(Light)
	typ := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := typ.Field(i)
		val := v.Field(i).String()
		if val == "" {
			t.Errorf("Light.%s is empty", field.Name)
		}
	}
}

func TestTheme_SelectionBgSeq(t *testing.T) {
	for _, tc := range []struct {
		name  string
		theme Theme
	}{
		{"Dark", Dark},
		{"Light", Light},
	} {
		t.Run(tc.name, func(t *testing.T) {
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
	for _, tc := range []struct {
		name  string
		theme Theme
	}{
		{"Dark", Dark},
		{"Light", Light},
	} {
		t.Run(tc.name, func(t *testing.T) {
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
	for _, tc := range []struct {
		name  string
		theme Theme
	}{
		{"Dark", Dark},
		{"Light", Light},
	} {
		t.Run(tc.name, func(t *testing.T) {
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
