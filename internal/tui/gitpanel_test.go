package tui

import (
	"fmt"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/708u/gracilius/internal/git"
)

func TestGitChangedFilesMsg_Populates(t *testing.T) {
	m := newTestModel(t)

	entries := []changedFileEntry{
		{name: "file1.go", status: git.StatusModified, absPath: "/tmp/file1.go",
			oldContent: []string{"old"}, newContent: []string{"new"},
			category: categoryUnstaged},
		{name: "file2.go", status: git.StatusAdded, absPath: "/tmp/file2.go",
			newContent: []string{"added"}, category: categoryUnstaged},
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
	// Visual rows: 1 header + 2 files
	if len(gs.visualRows) != 3 {
		t.Errorf("expected 3 visual rows, got %d", len(gs.visualRows))
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
	gs.visualRows, gs.entryToVisualIdx = buildGitVisualRows(entries)
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
			status:     git.StatusModified,
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
		{name: "image.png", status: git.StatusModified, absPath: "/tmp/image.png", binary: true},
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
			status:     git.StatusDeleted,
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
			status:     git.StatusAdded,
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
		{name: "a.go", status: git.StatusModified, category: categoryUnstaged},
		{name: "b.go", status: git.StatusAdded, category: categoryUnstaged},
		{name: "c.go", status: git.StatusDeleted, category: categoryUnstaged},
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
			status:     git.StatusModified,
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

func TestBuildGitVisualRows(t *testing.T) {
	entries := []changedFileEntry{
		{name: "staged.go", status: git.StatusModified, category: categoryStaged},
		{name: "unstaged.go", status: git.StatusModified, category: categoryUnstaged},
		{name: "untracked.go", status: git.StatusUntracked, category: categoryUntracked},
	}
	rows, reverseMap := buildGitVisualRows(entries)

	// 3 headers + 3 files = 6 rows
	if len(rows) != 6 {
		t.Fatalf("expected 6 rows, got %d", len(rows))
	}
	if !rows[0].isHeader {
		t.Error("expected first row to be header")
	}
	if rows[1].isHeader || rows[1].entryIdx != 0 {
		t.Error("expected second row to be staged entry")
	}
	if !rows[2].isHeader {
		t.Error("expected third row to be header")
	}
	if rows[3].isHeader || rows[3].entryIdx != 1 {
		t.Error("expected fourth row to be unstaged entry")
	}

	// Verify reverse map
	if reverseMap[0] != 1 {
		t.Errorf("expected reverseMap[0]=1, got %d", reverseMap[0])
	}
	if reverseMap[1] != 3 {
		t.Errorf("expected reverseMap[1]=3, got %d", reverseMap[1])
	}
	if reverseMap[2] != 5 {
		t.Errorf("expected reverseMap[2]=5, got %d", reverseMap[2])
	}
}

func TestBuildGitVisualRows_EmptySection(t *testing.T) {
	entries := []changedFileEntry{
		{name: "a.go", status: git.StatusModified, category: categoryUnstaged},
	}
	rows, _ := buildGitVisualRows(entries)
	// Only unstaged: 1 header + 1 file
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if !rows[0].isHeader {
		t.Error("expected header")
	}
	if rows[1].entryIdx != 0 {
		t.Error("expected entry index 0")
	}
}

func TestGitCursorHelpers(t *testing.T) {
	rows := []gitVisualRow{
		{isHeader: true, label: "Staged"},
		{entryIdx: 0},
		{isHeader: true, label: "Unstaged"},
		{entryIdx: 1},
		{entryIdx: 2},
	}

	if got := firstGitEntryIdx(rows); got != 0 {
		t.Errorf("expected firstGitEntryIdx=0, got %d", got)
	}
	if got := lastGitEntryIdx(rows); got != 2 {
		t.Errorf("expected lastGitEntryIdx=2, got %d", got)
	}
}

func TestGitChangedFilesMsg_Categories(t *testing.T) {
	m := newTestModel(t)

	entries := []changedFileEntry{
		{name: "staged.go", status: git.StatusModified, category: categoryStaged},
		{name: "unstaged.go", status: git.StatusModified, category: categoryUnstaged},
		{name: "new.txt", status: git.StatusUntracked, category: categoryUntracked},
	}

	m.Update(gitChangedFilesMsg{mode: gitModeUncommitted, entries: entries})

	gs := m.gitState()
	if len(gs.visualRows) != 6 {
		t.Fatalf("expected 6 visual rows (3 headers + 3 files), got %d",
			len(gs.visualRows))
	}
}

func TestOpenGitDiffEntry_DuplicateTab(t *testing.T) {
	m := newTestModel(t)
	m.activePanel = panelGitDiff

	setGitEntries(m, []changedFileEntry{
		{
			name:       "main.go",
			status:     git.StatusModified,
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
		{name: "a.go", status: git.StatusModified, category: categoryUnstaged},
		{name: "b.go", status: git.StatusAdded, category: categoryUnstaged},
	})
	gs := m.gitState()
	gs.cursor = 1

	// Switch to unstaged
	m.gitDiffMode = gitModeUnstaged
	setGitEntries(m, []changedFileEntry{
		{name: "c.go", status: git.StatusModified, category: categoryUnstaged},
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
