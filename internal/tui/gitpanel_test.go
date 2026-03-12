package tui

import (
	"fmt"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestGitChangedFilesMsg_Populates(t *testing.T) {
	m := newTestModel(t)

	entries := []changedFileEntry{
		{name: "file1.go", status: "M", absPath: "/tmp/file1.go",
			oldContent: []string{"old"}, newContent: []string{"new"}},
		{name: "file2.go", status: "A", absPath: "/tmp/file2.go",
			newContent: []string{"added"}},
	}

	m.Update(gitChangedFilesMsg{entries: entries})

	if !m.gitLoaded {
		t.Fatal("expected gitLoaded=true")
	}
	if len(m.gitChangedFiles) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(m.gitChangedFiles))
	}
	if m.gitChangedFiles[0].name != "file1.go" {
		t.Errorf("expected file1.go, got %s", m.gitChangedFiles[0].name)
	}
}

func TestGitChangedFilesMsg_Error(t *testing.T) {
	m := newTestModel(t)

	m.Update(gitChangedFilesMsg{err: errTest})

	if !m.gitLoaded {
		t.Fatal("expected gitLoaded=true even on error")
	}
	if m.statusMsg == "" {
		t.Fatal("expected statusMsg to be set on error")
	}
}

var errTest = &testError{}

type testError struct{}

func (e *testError) Error() string { return "test error" }

func TestOpenGitDiffEntry_CreatesDiffTab(t *testing.T) {
	m := newTestModel(t)
	m.focusPane = paneTree
	m.activePanel = panelGitDiff

	m.gitChangedFiles = []changedFileEntry{
		{
			name:       "main.go",
			status:     "M",
			absPath:    "/tmp/main.go",
			oldContent: []string{"old line"},
			newContent: []string{"new line"},
		},
	}
	m.gitCursor = 0

	m.openGitDiffEntry()

	if len(m.tabs) != 1 {
		t.Fatalf("expected 1 tab, got %d", len(m.tabs))
	}
	tab := m.tabs[0]
	if tab.kind != diffTab {
		t.Errorf("expected diffTab, got %d", tab.kind)
	}
	if tab.diff != nil {
		t.Error("expected diff=nil (no accept/reject)")
	}
	if tab.diffViewData == nil {
		t.Error("expected diffViewData!=nil")
	}
	if m.focusPane != paneEditor {
		t.Errorf("expected focusPane=paneEditor, got %d", m.focusPane)
	}
}

func TestOpenGitDiffEntry_Binary(t *testing.T) {
	m := newTestModel(t)
	m.activePanel = panelGitDiff

	m.gitChangedFiles = []changedFileEntry{
		{name: "image.png", status: "M", absPath: "/tmp/image.png", binary: true},
	}
	m.gitCursor = 0

	m.openGitDiffEntry()

	if len(m.tabs) != 0 {
		t.Fatalf("expected 0 tabs for binary file, got %d", len(m.tabs))
	}
	if m.statusMsg == "" {
		t.Error("expected statusMsg for binary file")
	}
}

func TestOpenGitDiffEntry_DeletedFile(t *testing.T) {
	m := newTestModel(t)
	m.activePanel = panelGitDiff

	m.gitChangedFiles = []changedFileEntry{
		{
			name:       "removed.go",
			status:     "D",
			absPath:    "/tmp/removed.go",
			oldContent: []string{"old line1", "old line2"},
			newContent: nil,
		},
	}
	m.gitCursor = 0

	m.openGitDiffEntry()

	if len(m.tabs) != 1 {
		t.Fatalf("expected 1 tab, got %d", len(m.tabs))
	}
	if m.tabs[0].diffViewData == nil {
		t.Error("expected diffViewData!=nil for deleted file")
	}
}

func TestOpenGitDiffEntry_NewFile(t *testing.T) {
	m := newTestModel(t)
	m.activePanel = panelGitDiff

	m.gitChangedFiles = []changedFileEntry{
		{
			name:       "new.go",
			status:     "A",
			absPath:    "/tmp/new.go",
			oldContent: nil,
			newContent: []string{"new line1"},
		},
	}
	m.gitCursor = 0

	m.openGitDiffEntry()

	if len(m.tabs) != 1 {
		t.Fatalf("expected 1 tab, got %d", len(m.tabs))
	}
	if m.tabs[0].diffViewData == nil {
		t.Error("expected diffViewData!=nil for new file")
	}
}

func TestGitPanelNavigation(t *testing.T) {
	m := newTestModel(t)
	m.focusPane = paneTree
	m.activePanel = panelGitDiff
	m.gitLoaded = true
	m.gitChangedFiles = []changedFileEntry{
		{name: "a.go", status: "M"},
		{name: "b.go", status: "A"},
		{name: "c.go", status: "D"},
	}
	m.gitCursor = 0

	// Down
	m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.gitCursor != 1 {
		t.Errorf("expected gitCursor=1 after down, got %d", m.gitCursor)
	}

	m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.gitCursor != 2 {
		t.Errorf("expected gitCursor=2 after second down, got %d", m.gitCursor)
	}

	// Don't go past the end
	m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.gitCursor != 2 {
		t.Errorf("expected gitCursor=2 (clamped), got %d", m.gitCursor)
	}

	// Up
	m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if m.gitCursor != 1 {
		t.Errorf("expected gitCursor=1 after up, got %d", m.gitCursor)
	}
}

func TestPanelSwitchTriggersLoad(t *testing.T) {
	m := newTestModel(t)
	m.focusPane = paneTree
	m.activePanel = panelFiles

	// Switch to panelGitDiff (Shift+Tab cycles)
	var cmd tea.Cmd
	for m.activePanel != panelGitDiff {
		_, cmd = m.Update(tea.KeyPressMsg{
			Code: tea.KeyTab,
			Mod:  tea.ModShift,
		})
	}

	if cmd == nil {
		t.Error("expected cmd to be returned for lazy loading")
	}
	if m.activePanel != panelGitDiff {
		t.Errorf("expected panelGitDiff, got %d", m.activePanel)
	}
}

func TestGitDiffView_ScrollWithKeys(t *testing.T) {
	m := newTestModel(t)
	m.activePanel = panelGitDiff

	// Create a large diff (more lines than screen height).
	old := make([]string, 100)
	new_ := make([]string, 100)
	for i := range 100 {
		old[i] = fmt.Sprintf("old line %d", i)
		new_[i] = fmt.Sprintf("new line %d", i)
	}

	m.gitChangedFiles = []changedFileEntry{
		{
			name:       "big.go",
			status:     "M",
			absPath:    "/tmp/big.go",
			oldContent: old,
			newContent: new_,
		},
	}
	m.gitCursor = 0
	m.openGitDiffEntry()

	if len(m.tabs) != 1 {
		t.Fatalf("expected 1 tab, got %d", len(m.tabs))
	}
	tab := m.tabs[0]
	if tab.diffViewData == nil {
		t.Fatal("expected diffViewData to be set")
	}
	if m.focusPane != paneEditor {
		t.Fatalf("expected paneEditor, got %d", m.focusPane)
	}

	// Viewport should have content lines (pre-rendered diff).
	if tab.vp.TotalLineCount() == 0 {
		t.Fatal("expected viewport content lines for diff")
	}

	// Set offset to 0 to test scrolling down.
	tab.vp.SetYOffset(0)
	initialOffset := tab.vp.YOffset()

	// Press 'j' to scroll down.
	m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})

	if tab.vp.YOffset() != initialOffset+1 {
		t.Errorf("expected offset=%d after j, got %d",
			initialOffset+1, tab.vp.YOffset())
	}

	// Press 'k' to scroll up.
	m.Update(tea.KeyPressMsg{Code: 'k', Text: "k"})

	if tab.vp.YOffset() != initialOffset {
		t.Errorf("expected offset=%d after k, got %d",
			initialOffset, tab.vp.YOffset())
	}
}

func TestOpenGitDiffEntry_DuplicateTab(t *testing.T) {
	m := newTestModel(t)
	m.activePanel = panelGitDiff

	entry := changedFileEntry{
		name:       "main.go",
		status:     "M",
		absPath:    "/tmp/main.go",
		oldContent: []string{"old"},
		newContent: []string{"new"},
	}
	m.gitChangedFiles = []changedFileEntry{entry}
	m.gitCursor = 0

	// Open once
	m.openGitDiffEntry()
	if len(m.tabs) != 1 {
		t.Fatalf("expected 1 tab, got %d", len(m.tabs))
	}

	// Open again - should reuse existing tab
	m.openGitDiffEntry()
	if len(m.tabs) != 1 {
		t.Fatalf("expected 1 tab (reused), got %d", len(m.tabs))
	}
}
