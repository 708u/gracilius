package tui

import (
	"os"
	"path/filepath"
	"testing"

	"charm.land/bubbles/v2/list"
	"github.com/708u/gracilius/internal/tui/render"
)

func TestScanAllFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create directory structure:
	//   dir/
	//     a.go
	//     sub/
	//       b.txt
	os.MkdirAll(filepath.Join(dir, "sub"), 0o755)
	os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a"), 0o644)
	os.WriteFile(filepath.Join(dir, "sub", "b.txt"), []byte("hello"), 0o644)

	items := scanAllFiles(dir, nil)

	paths := make(map[string]bool)
	for _, fi := range items {
		paths[fi.path] = true
	}

	if !paths["a.go"] {
		t.Error("expected a.go in results")
	}
	expected := filepath.Join("sub", "b.txt")
	if !paths[expected] {
		t.Errorf("expected %s in results", expected)
	}
	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}
}

func TestScanAllFiles_ExcludesHidden(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create directory structure:
	//   dir/
	//     visible.go
	//     .git/
	//       config
	//     node_modules/
	//       pkg.js
	//     .hidden_file
	os.MkdirAll(filepath.Join(dir, ".git"), 0o755)
	os.MkdirAll(filepath.Join(dir, "node_modules"), 0o755)
	os.WriteFile(filepath.Join(dir, "visible.go"), []byte("package v"), 0o644)
	os.WriteFile(filepath.Join(dir, ".git", "config"), []byte("[core]"), 0o644)
	os.WriteFile(filepath.Join(dir, "node_modules", "pkg.js"), []byte("module.exports"), 0o644)
	os.WriteFile(filepath.Join(dir, ".hidden_file"), []byte("secret"), 0o644)

	items := scanAllFiles(dir, nil)

	paths := make(map[string]bool)
	for _, fi := range items {
		paths[fi.path] = true
	}

	if !paths["visible.go"] {
		t.Error("expected visible.go in results")
	}
	if paths[filepath.Join(".git", "config")] {
		t.Error(".git/config should be excluded")
	}
	if paths[filepath.Join("node_modules", "pkg.js")] {
		t.Error("node_modules/pkg.js should be excluded")
	}
	if paths[".hidden_file"] {
		t.Error(".hidden_file should be excluded")
	}
	if len(items) != 1 {
		t.Errorf("expected 1 item, got %d", len(items))
	}
}

// newTestOverlay creates an openFileOverlay with the given file items
// pre-populated (bypassing filesystem scanning).
func newTestOverlay(items []fileItem) openFileOverlay {
	s := newOpenFileOverlay(iconSymbol, render.Dark)
	s.allItems = items
	s.targets = make([]string, len(items))
	for i, fi := range items {
		s.targets[i] = fi.path
	}
	s.active = true

	listItems := make([]list.Item, len(items))
	for i := range items {
		listItems[i] = items[i]
	}
	s.list.SetItems(listItems)
	return s
}

func TestApplyFilter_EmptyTerm(t *testing.T) {
	t.Parallel()
	items := []fileItem{
		{path: "main.go"},
		{path: "util.go"},
		{path: "readme.md"},
	}
	s := newTestOverlay(items)
	s.input.SetValue("")

	s.applyFilter()

	got := s.list.Items()
	if len(got) != 3 {
		t.Fatalf("expected 3 items, got %d", len(got))
	}
}

func TestApplyFilter_FuzzyMatch(t *testing.T) {
	t.Parallel()
	items := []fileItem{
		{path: "main.go"},
		{path: "model.go"},
		{path: "readme.md"},
	}
	s := newTestOverlay(items)
	s.input.SetValue("mod")

	s.applyFilter()

	got := s.list.Items()
	if len(got) != 1 {
		t.Fatalf("expected 1 match for 'mod', got %d", len(got))
	}
	fi := got[0].(fileItem)
	if fi.path != "model.go" {
		t.Errorf("expected model.go, got %s", fi.path)
	}
	if len(fi.matchedRunes) == 0 {
		t.Error("expected matchedRunes to be populated")
	}
}

func TestApplyFilter_NoMatch(t *testing.T) {
	t.Parallel()
	items := []fileItem{
		{path: "main.go"},
		{path: "util.go"},
	}
	s := newTestOverlay(items)
	s.input.SetValue("xyz")

	s.applyFilter()

	got := s.list.Items()
	if len(got) != 0 {
		t.Errorf("expected 0 matches for 'xyz', got %d", len(got))
	}
}

func TestComputeLayout_WideTerminal(t *testing.T) {
	t.Parallel()
	items := []fileItem{
		{path: "a.go"},
		{path: "b.go"},
		{path: "c.go"},
	}
	s := newTestOverlay(items)

	// 120x40 terminal: overlay should be capped at overlayMaxW (80)
	g := s.computeLayout(120, 40)

	if g.overlayW != overlayMaxW {
		t.Errorf("overlayW: expected %d, got %d", overlayMaxW, g.overlayW)
	}
	if g.innerW != overlayMaxW-overlayBorderW-overlayPaddingW {
		t.Errorf("innerW: expected %d, got %d",
			overlayMaxW-overlayBorderW-overlayPaddingW, g.innerW)
	}
	// Horizontally centered
	expectedX := (120 - overlayMaxW) / 2
	if g.startX != expectedX {
		t.Errorf("startX: expected %d, got %d", expectedX, g.startX)
	}
	if g.startY != paneHeaderRows {
		t.Errorf("startY: expected %d, got %d", paneHeaderRows, g.startY)
	}
}

func TestComputeLayout_NarrowTerminal(t *testing.T) {
	t.Parallel()
	items := []fileItem{{path: "a.go"}}
	s := newTestOverlay(items)

	// 40 columns: overlay = 40*3/4 = 30
	g := s.computeLayout(40, 30)

	expectedW := 40 * overlayWidthRatio / 4
	if g.overlayW != expectedW {
		t.Errorf("overlayW: expected %d, got %d", expectedW, g.overlayW)
	}
}

func TestComputeLayout_ShortTerminal(t *testing.T) {
	t.Parallel()
	items := []fileItem{
		{path: "a.go"},
		{path: "b.go"},
		{path: "c.go"},
		{path: "d.go"},
		{path: "e.go"},
	}
	s := newTestOverlay(items)

	// Very short terminal: height = 12
	// paneHeaderRows=2, footerHeight=4, borderH=2 => available=4
	g := s.computeLayout(100, 12)

	if g.listH < overlayMinItems {
		t.Errorf("listH should be at least %d, got %d", overlayMinItems, g.listH)
	}
	// Box should not extend past footer
	maxBottom := 12 - footerHeight
	if g.startY+g.boxH > maxBottom {
		t.Errorf("box extends past footer: startY=%d boxH=%d maxBottom=%d",
			g.startY, g.boxH, maxBottom)
	}
}

func TestHandleClick_OutsideOverlay(t *testing.T) {
	t.Parallel()
	items := []fileItem{{path: "a.go"}, {path: "b.go"}}
	s := newTestOverlay(items)

	width, height := 100, 40
	g := s.computeLayout(width, height)

	// Click above the overlay
	_, close := s.handleClick(g.startX+1, g.startY-1, width, height)
	if !close {
		t.Error("click above overlay should close it")
	}

	// Click to the left of the overlay
	_, close = s.handleClick(g.startX-1, g.startY+1, width, height)
	if !close {
		t.Error("click left of overlay should close it")
	}

	// Click below the overlay
	_, close = s.handleClick(g.startX+1, g.startY+g.boxH, width, height)
	if !close {
		t.Error("click below overlay should close it")
	}

	// Click to the right of the overlay
	_, close = s.handleClick(g.startX+g.overlayW, g.startY+1, width, height)
	if !close {
		t.Error("click right of overlay should close it")
	}
}

func TestHandleClick_InsideNonListArea(t *testing.T) {
	t.Parallel()
	items := []fileItem{{path: "a.go"}}
	s := newTestOverlay(items)

	width, height := 100, 40
	g := s.computeLayout(width, height)

	// Click on the input area (inside overlay but above list)
	path, close := s.handleClick(g.startX+2, g.startY+1, width, height)
	if close {
		t.Error("click on input area should not close overlay")
	}
	if path != "" {
		t.Errorf("click on input area should not return a path, got %q", path)
	}
}

func TestHandleClick_OnListItem(t *testing.T) {
	t.Parallel()
	items := []fileItem{{path: "first.go"}, {path: "second.go"}}
	s := newTestOverlay(items)

	width, height := 100, 40
	g := s.computeLayout(width, height)
	s.list.SetSize(g.innerW, g.listH)

	// Click on first list item
	listStartY := g.startY + overlayListOffset
	path, close := s.handleClick(g.startX+2, listStartY, width, height)
	if close {
		t.Error("click on list item should not close overlay")
	}
	if path != "first.go" {
		t.Errorf("expected first.go, got %q", path)
	}
}

func TestSelectedPath_Empty(t *testing.T) {
	t.Parallel()
	s := newOpenFileOverlay(iconSymbol, render.Dark)
	if p := s.selectedPath(); p != "" {
		t.Errorf("expected empty, got %q", p)
	}
}

func TestSelectedPath_WithItems(t *testing.T) {
	t.Parallel()
	items := []fileItem{{path: "hello.go"}, {path: "world.go"}}
	s := newTestOverlay(items)
	s.list.SetSize(80, 10)

	s.list.Select(1)
	if p := s.selectedPath(); p != "world.go" {
		t.Errorf("expected world.go, got %q", p)
	}
}
