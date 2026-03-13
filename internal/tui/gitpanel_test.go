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

	m.Update(gitChangedFilesMsg{mode: gitModeUncommitted, entries: entries})

	gs := m.gitState()
	if !gs.loaded {
		t.Fatal("expected loaded=true")
	}
	if len(gs.entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(gs.entries))
	}
	if gs.entries[0].name != "file1.go" {
		t.Errorf("expected file1.go, got %s", gs.entries[0].name)
	}
}

func TestGitChangedFilesMsg_Error(t *testing.T) {
	m := newTestModel(t)

	m.Update(gitChangedFilesMsg{mode: gitModeUncommitted, err: errTest})

	gs := m.gitState()
	if !gs.loaded {
		t.Fatal("expected loaded=true even on error")
	}
	if m.statusMsg == "" {
		t.Fatal("expected statusMsg to be set on error")
	}
}

var errTest = &testError{}

type testError struct{}

func (e *testError) Error() string { return "test error" }

func setGitEntries(m *Model, entries []changedFileEntry) {
	gs := m.gitState()
	gs.entries = entries
	gs.cursor = 0
	gs.loaded = true
}

func TestOpenGitDiffEntry_CreatesDiffTab(t *testing.T) {
	m := newTestModel(t)
	m.focusPane = paneTree
	m.activePanel = panelGitDiff

	setGitEntries(m, []changedFileEntry{
		{
			name:       "main.go",
			status:     "M",
			absPath:    "/tmp/main.go",
			oldContent: []string{"old line"},
			newContent: []string{"new line"},
		},
	})

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
	if !tab.hasGitDiffModeTag || tab.gitDiffModeTag != gitModeUncommitted {
		t.Errorf("expected gitDiffModeTag=gitModeUncommitted, got %d", tab.gitDiffModeTag)
	}
	if m.focusPane != paneEditor {
		t.Errorf("expected focusPane=paneEditor, got %d", m.focusPane)
	}
}

func TestOpenGitDiffEntry_Binary(t *testing.T) {
	m := newTestModel(t)
	m.activePanel = panelGitDiff

	setGitEntries(m, []changedFileEntry{
		{name: "image.png", status: "M", absPath: "/tmp/image.png", binary: true},
	})

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

	setGitEntries(m, []changedFileEntry{
		{
			name:       "removed.go",
			status:     "D",
			absPath:    "/tmp/removed.go",
			oldContent: []string{"old line1", "old line2"},
			newContent: nil,
		},
	})

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

	setGitEntries(m, []changedFileEntry{
		{
			name:       "new.go",
			status:     "A",
			absPath:    "/tmp/new.go",
			oldContent: nil,
			newContent: []string{"new line1"},
		},
	})

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

	setGitEntries(m, []changedFileEntry{
		{name: "a.go", status: "M"},
		{name: "b.go", status: "A"},
		{name: "c.go", status: "D"},
	})

	gs := m.gitState()

	// Down
	m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if gs.cursor != 1 {
		t.Errorf("expected cursor=1 after down, got %d", gs.cursor)
	}

	m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if gs.cursor != 2 {
		t.Errorf("expected cursor=2 after second down, got %d", gs.cursor)
	}

	// Don't go past the end
	m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if gs.cursor != 2 {
		t.Errorf("expected cursor=2 (clamped), got %d", gs.cursor)
	}

	// Up
	m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if gs.cursor != 1 {
		t.Errorf("expected cursor=1 after up, got %d", gs.cursor)
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

	setGitEntries(m, []changedFileEntry{
		{
			name:       "big.go",
			status:     "M",
			absPath:    "/tmp/big.go",
			oldContent: old,
			newContent: new_,
		},
	})
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

	setGitEntries(m, []changedFileEntry{
		{
			name:       "main.go",
			status:     "M",
			absPath:    "/tmp/main.go",
			oldContent: []string{"old"},
			newContent: []string{"new"},
		},
	})

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

func TestGitDiffModeSwitching(t *testing.T) {
	m := newTestModel(t)
	m.focusPane = paneTree
	m.activePanel = panelGitDiff
	m.gitState().loaded = true

	if m.gitDiffMode != gitModeUncommitted {
		t.Fatalf("expected initial mode=gitModeUncommitted, got %d", m.gitDiffMode)
	}

	// Right: Uncommitted -> Unstaged
	m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	if m.gitDiffMode != gitModeUnstaged {
		t.Errorf("expected gitModeUnstaged, got %d", m.gitDiffMode)
	}

	// Left: Unstaged -> Uncommitted
	m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	if m.gitDiffMode != gitModeUncommitted {
		t.Errorf("expected gitModeUncommitted, got %d", m.gitDiffMode)
	}

	// Left wraps: Uncommitted -> Branch
	m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	// Branch mode may fail to init merge-base (no git repo), so it stays on Branch
	// but may show an error. The mode should still have changed.
	if m.gitDiffMode != gitModeBranch {
		t.Errorf("expected gitModeBranch (wrap), got %d", m.gitDiffMode)
	}
}

func TestGitDiffModePerModeState(t *testing.T) {
	m := newTestModel(t)
	m.focusPane = paneTree
	m.activePanel = panelGitDiff

	// Set up state for uncommitted mode
	setGitEntries(m, []changedFileEntry{
		{name: "a.go", status: "M"},
		{name: "b.go", status: "A"},
	})
	gs := m.gitState()
	gs.cursor = 1

	// Switch to unstaged
	m.gitDiffMode = gitModeUnstaged
	setGitEntries(m, []changedFileEntry{
		{name: "c.go", status: "M"},
	})

	// Switch back to uncommitted
	m.gitDiffMode = gitModeUncommitted
	gs = m.gitState()
	if gs.cursor != 1 {
		t.Errorf("expected cursor=1 preserved, got %d", gs.cursor)
	}
	if len(gs.entries) != 2 {
		t.Errorf("expected 2 entries preserved, got %d", len(gs.entries))
	}
}

func TestGitDiffModeLabel(t *testing.T) {
	tests := []struct {
		mode gitDiffMode
		want string
	}{
		{gitModeUncommitted, "Uncommitted"},
		{gitModeUnstaged, "Unstaged"},
		{gitModeStaged, "Staged"},
		{gitModeBranch, "Branch"},
	}
	for _, tt := range tests {
		if got := tt.mode.label(); got != tt.want {
			t.Errorf("gitDiffMode(%d).label() = %q, want %q", tt.mode, got, tt.want)
		}
	}
}

func TestGitDiffModeTabPrefix(t *testing.T) {
	tests := []struct {
		mode gitDiffMode
		want string
	}{
		{gitModeUncommitted, "[uncommit]"},
		{gitModeUnstaged, "[unstaged]"},
		{gitModeStaged, "[staged]"},
		{gitModeBranch, "[branch]"},
	}
	for _, tt := range tests {
		if got := tt.mode.tabPrefix(); got != tt.want {
			t.Errorf("gitDiffMode(%d).tabPrefix() = %q, want %q", tt.mode, got, tt.want)
		}
	}
}
