package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestHandleOpenDiff(t *testing.T) {
	m := newTestModel(t)

	msg := OpenDiffMsg{
		FilePath: "diff.go",
		Contents: "line1\nline2\nline3",
	}
	m.Update(msg)

	if len(m.tabs) != 1 {
		t.Fatalf("expected 1 tab, got %d", len(m.tabs))
	}
	if m.tabs[0].kind != diffTab {
		t.Errorf("expected diffTab, got %d", m.tabs[0].kind)
	}
	if m.activeTab != 0 {
		t.Errorf("expected activeTab=0, got %d", m.activeTab)
	}
	if m.focusPane != paneEditor {
		t.Errorf("expected focusPane=paneEditor, got %d", m.focusPane)
	}
	if len(m.tabs[0].lines) != 3 {
		t.Errorf("expected 3 lines, got %d", len(m.tabs[0].lines))
	}
}

func TestHandleCloseDiff(t *testing.T) {
	m := newTestModel(t)

	// Add a file tab and a diff tab.
	ft := newFileTab()
	ft.filePath = "file.go"
	ft.lines = []string{"hello"}
	m.tabs = append(m.tabs, ft)

	dt := newDiffTab("diff.go", []string{"diff1", "diff2"})
	m.tabs = append(m.tabs, dt)
	m.activeTab = 1

	m.Update(CloseDiffMsg{})

	if len(m.tabs) != 1 {
		t.Fatalf("expected 1 tab after close diff, got %d", len(m.tabs))
	}
	if m.tabs[0].kind != fileTab {
		t.Errorf("expected remaining tab to be fileTab, got %d", m.tabs[0].kind)
	}
}

func TestHandleFileChanged(t *testing.T) {
	content := "line1\nline2\nline3"
	m := newTestModelWithFile(t, content)

	// Move cursor to end.
	tab := m.tabs[0]
	tab.cursorLine = 2
	tab.cursorChar = 5

	// Simulate file change with fewer lines.
	m.Update(fileChangedMsg{lines: []string{"only one line"}})

	if tab.cursorLine != 0 {
		t.Errorf("expected cursorLine clipped to 0, got %d", tab.cursorLine)
	}
}

func TestHandleWindowSize(t *testing.T) {
	m := newTestModel(t)
	m.treeWidth = 80

	m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})

	if m.width != 100 {
		t.Errorf("expected width=100, got %d", m.width)
	}
	if m.height != 50 {
		t.Errorf("expected height=50, got %d", m.height)
	}

	maxWidth := 100 * maxTreeWidthPercent / 100
	if m.treeWidth > maxWidth {
		t.Errorf("expected treeWidth <= %d, got %d", maxWidth, m.treeWidth)
	}
}

func TestKeyNavigation_UpDown(t *testing.T) {
	content := "line1\nline2\nline3\nline4\nline5"
	m := newTestModelWithFile(t, content)
	tab := m.tabs[0]
	tab.cursorLine = 0

	// Move down.
	m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if tab.cursorLine != 1 {
		t.Errorf("expected cursorLine=1 after down, got %d", tab.cursorLine)
	}

	// Move up.
	m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if tab.cursorLine != 0 {
		t.Errorf("expected cursorLine=0 after up, got %d", tab.cursorLine)
	}
}

func TestMouseClick_TreeEntry(t *testing.T) {
	m := newTestModel(t)
	m.fileTree = []fileEntry{
		{path: "dir1", name: "dir1", isDir: true, depth: 0},
		{path: "file1.go", name: "file1.go", isDir: false, depth: 0},
	}

	// Click on second tree entry (y = contentStartY + 1).
	m.Update(tea.MouseClickMsg{
		X:      5,
		Y:      contentStartY + 1,
		Button: tea.MouseLeft,
	})

	if m.treeCursor != 1 {
		t.Errorf("expected treeCursor=1, got %d", m.treeCursor)
	}
}
