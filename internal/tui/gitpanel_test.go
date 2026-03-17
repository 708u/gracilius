package tui

import (
	"fmt"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/708u/gracilius/internal/diff"
	"github.com/708u/gracilius/internal/git"
	"github.com/708u/gracilius/internal/tui/render"
)

// fillPathFields populates baseName/dirName for test entries.
func fillPathFields(entries []changedFileEntry) []changedFileEntry {
	for i := range entries {
		entries[i].baseName = filepath.Base(entries[i].name)
		entries[i].dirName = filepath.Dir(entries[i].name)
	}
	return entries
}

func TestGitChangedFilesMsg_Populates(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)

	entries := fillPathFields([]changedFileEntry{
		{name: "file1.go", status: git.StatusModified, absPath: "/tmp/file1.go",
			oldContent: []string{"old"}, newContent: []string{"new"},
			category: categoryUnstaged},
		{name: "file2.go", status: git.StatusAdded, absPath: "/tmp/file2.go",
			newContent: []string{"added"}, category: categoryUnstaged},
	})

	m.Update(gitChangedFilesMsg{mode: gitModeWorking, entries: entries})

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
	// Visual rows: 1 category header + 1 dir header (./) + 2 files
	if len(gs.visualRows) != 4 {
		t.Errorf("expected 4 visual rows, got %d", len(gs.visualRows))
	}
}

func TestGitChangedFilesMsg_Error(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)

	m.Update(gitChangedFilesMsg{mode: gitModeWorking, err: errTest})

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
	fillPathFields(entries)
	gs := m.gitState()
	gs.entries = entries
	gs.visualRows, gs.entryToVisualIdx = buildGitVisualRows(entries)
	gs.cursor = 0
	gs.loaded = true
}

func TestOpenGitDiffEntry_CreatesDiffTab(t *testing.T) {
	t.Parallel()
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
	if !tab.hasGitDiffModeTag || tab.gitDiffModeTag != gitModeWorking {
		t.Errorf("expected gitDiffModeTag=gitModeWorking, got %d", tab.gitDiffModeTag)
	}
	if m.focusPane != paneEditor {
		t.Errorf("expected focusPane=paneEditor, got %d", m.focusPane)
	}
}

func TestOpenGitDiffEntry_Binary(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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

func TestGitPanelNavigation_CrossDirectory(t *testing.T) {
	m := newTestModel(t)
	m.focusPane = paneTree
	m.activePanel = panelGitDiff

	// Entries where array order differs from visual order.
	// Visual grouping by directory puts entries 0,2 together
	// (internal/git/) and entry 1 separately (internal/tui/).
	// Visual order: entry0, entry2, entry1.
	setGitEntries(m, []changedFileEntry{
		{name: "internal/git/a.go", status: git.StatusModified, category: categoryUnstaged},
		{name: "internal/tui/b.go", status: git.StatusModified, category: categoryUnstaged},
		{name: "internal/git/c.go", status: git.StatusModified, category: categoryUnstaged},
	})

	gs := m.gitState()
	gs.cursor = firstGitEntryIdx(gs.visualRows) // entry 0

	// Down from entry 0 → entry 2 (next in visual order, same dir)
	m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if gs.cursor != 2 {
		t.Errorf("expected cursor=2 (next in visual order), got %d", gs.cursor)
	}

	// Down from entry 2 → entry 1 (crosses directory boundary)
	m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if gs.cursor != 1 {
		t.Errorf("expected cursor=1 (cross-dir), got %d", gs.cursor)
	}

	// Down from entry 1 → still 1 (last file)
	m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if gs.cursor != 1 {
		t.Errorf("expected cursor=1 (clamped at end), got %d", gs.cursor)
	}

	// Up from entry 1 → entry 2
	m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if gs.cursor != 2 {
		t.Errorf("expected cursor=2 after up, got %d", gs.cursor)
	}

	// Up from entry 2 → entry 0
	m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if gs.cursor != 0 {
		t.Errorf("expected cursor=0 after up, got %d", gs.cursor)
	}

	// Up from entry 0 → still 0 (first file)
	m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if gs.cursor != 0 {
		t.Errorf("expected cursor=0 (clamped at start), got %d", gs.cursor)
	}
}

func TestPanelSwitchTriggersLoad(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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

	// Set offset and cursor to 0 to test cursor movement.
	tab.vp.SetYOffset(0)
	tab.diffCursor = 0
	tab.diffAnchor = 0
	initialCursor := tab.diffCursor

	// Press 'j' to move diff cursor down.
	m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})

	if tab.diffCursor != initialCursor+1 {
		t.Errorf("expected diffCursor=%d after j, got %d",
			initialCursor+1, tab.diffCursor)
	}

	// Press 'k' to move diff cursor up.
	m.Update(tea.KeyPressMsg{Code: 'k', Text: "k"})

	if tab.diffCursor != initialCursor {
		t.Errorf("expected diffCursor=%d after k, got %d",
			initialCursor, tab.diffCursor)
	}
}

func TestBuildGitVisualRows(t *testing.T) {
	t.Parallel()
	entries := fillPathFields([]changedFileEntry{
		{name: "staged.go", status: git.StatusModified, category: categoryStaged},
		{name: "unstaged.go", status: git.StatusModified, category: categoryUnstaged},
		{name: "untracked.go", status: git.StatusUntracked, category: categoryUntracked},
	})
	rows, reverseMap := buildGitVisualRows(entries)

	// 3 category headers + 3 dir headers (./) + 3 files = 9 rows
	if len(rows) != 9 {
		t.Fatalf("expected 9 rows, got %d", len(rows))
	}
	if !rows[0].isHeader {
		t.Error("expected row 0 to be category header")
	}
	if !rows[1].isDirHeader {
		t.Error("expected row 1 to be dir header")
	}
	if !rows[2].isFileRow() || rows[2].entryIdx != 0 {
		t.Error("expected row 2 to be staged entry")
	}
	if !rows[3].isHeader {
		t.Error("expected row 3 to be category header")
	}
	if !rows[5].isFileRow() || rows[5].entryIdx != 1 {
		t.Error("expected row 5 to be unstaged entry")
	}

	// Verify reverse map
	if reverseMap[0] != 2 {
		t.Errorf("expected reverseMap[0]=2, got %d", reverseMap[0])
	}
	if reverseMap[1] != 5 {
		t.Errorf("expected reverseMap[1]=5, got %d", reverseMap[1])
	}
	if reverseMap[2] != 8 {
		t.Errorf("expected reverseMap[2]=8, got %d", reverseMap[2])
	}
}

func TestBuildGitVisualRows_EmptySection(t *testing.T) {
	t.Parallel()
	entries := fillPathFields([]changedFileEntry{
		{name: "a.go", status: git.StatusModified, category: categoryUnstaged},
	})
	rows, _ := buildGitVisualRows(entries)
	// Only unstaged: 1 category header + 1 dir header (./) + 1 file
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(rows))
	}
	if !rows[0].isHeader {
		t.Error("expected category header")
	}
	if !rows[1].isDirHeader {
		t.Error("expected dir header")
	}
	if rows[2].entryIdx != 0 {
		t.Error("expected entry index 0")
	}
}

func TestGitCursorHelpers(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	m := newTestModel(t)

	entries := fillPathFields([]changedFileEntry{
		{name: "staged.go", status: git.StatusModified, category: categoryStaged},
		{name: "unstaged.go", status: git.StatusModified, category: categoryUnstaged},
		{name: "new.txt", status: git.StatusUntracked, category: categoryUntracked},
	})

	m.Update(gitChangedFilesMsg{mode: gitModeWorking, entries: entries})

	gs := m.gitState()
	if len(gs.visualRows) != 9 {
		t.Fatalf("expected 9 visual rows (3 cat + 3 dir + 3 files), got %d",
			len(gs.visualRows))
	}
}

func TestOpenGitDiffEntry_DuplicateTab(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	m := newTestModel(t)
	m.focusPane = paneTree
	m.activePanel = panelGitDiff
	m.gitState().loaded = true

	if m.gitDiffMode != gitModeWorking {
		t.Fatalf("expected initial mode=gitModeWorking, got %d", m.gitDiffMode)
	}

	// Right: Working -> Branch
	m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	if m.gitDiffMode != gitModeBranch {
		t.Errorf("expected gitModeBranch, got %d", m.gitDiffMode)
	}

	// Right wraps: Branch -> Working
	m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	if m.gitDiffMode != gitModeWorking {
		t.Errorf("expected gitModeWorking (wrap), got %d", m.gitDiffMode)
	}

	// Left wraps: Working -> Branch
	m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	if m.gitDiffMode != gitModeBranch {
		t.Errorf("expected gitModeBranch (wrap left), got %d", m.gitDiffMode)
	}
}

func TestGitDiffModePerModeState(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.focusPane = paneTree
	m.activePanel = panelGitDiff

	// Set up state for Working mode
	setGitEntries(m, []changedFileEntry{
		{name: "a.go", status: git.StatusModified, category: categoryUnstaged},
		{name: "b.go", status: git.StatusAdded, category: categoryUnstaged},
	})
	gs := m.gitState()
	gs.cursor = 1

	// Switch to Branch
	m.gitDiffMode = gitModeBranch
	setGitEntries(m, []changedFileEntry{
		{name: "c.go", status: git.StatusModified, category: categoryUnstaged},
	})

	// Switch back to Working
	m.gitDiffMode = gitModeWorking
	gs = m.gitState()
	if gs.cursor != 1 {
		t.Errorf("expected cursor=1 preserved, got %d", gs.cursor)
	}
	if len(gs.entries) != 2 {
		t.Errorf("expected 2 entries preserved, got %d", len(gs.entries))
	}
}

func TestGitDiffModeLabel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		mode          gitDiffMode
		defaultBranch string
		want          string
	}{
		{gitModeWorking, "", "working"},
		{gitModeBranch, "main", "vs main"},
		{gitModeBranch, "master", "vs master"},
		{gitModeBranch, "", "vs main"},
	}
	for _, tt := range tests {
		if got := tt.mode.label(tt.defaultBranch); got != tt.want {
			t.Errorf("gitDiffMode(%d).label(%q) = %q, want %q", tt.mode, tt.defaultBranch, got, tt.want)
		}
	}
}

func TestBuildGitVisualRows_DirGrouping(t *testing.T) {
	t.Parallel()
	entries := fillPathFields([]changedFileEntry{
		{name: "internal/git/diff.go", status: git.StatusModified, category: categoryUnstaged},
		{name: "internal/git/status.go", status: git.StatusModified, category: categoryUnstaged},
		{name: "internal/tui/model.go", status: git.StatusModified, category: categoryUnstaged},
	})
	rows, reverseMap := buildGitVisualRows(entries)

	// 1 category header + 2 dir headers + 3 files = 6 rows
	if len(rows) != 6 {
		t.Fatalf("expected 6 rows, got %d", len(rows))
	}
	if !rows[0].isHeader {
		t.Error("row 0: expected category header")
	}
	if !rows[1].isDirHeader || rows[1].label != "  internal/git/" {
		t.Errorf("row 1: expected dir header 'internal/git/', got %q (isDirHeader=%v)",
			rows[1].label, rows[1].isDirHeader)
	}
	if !rows[2].isFileRow() || rows[2].entryIdx != 0 {
		t.Errorf("row 2: expected file entry 0, got entryIdx=%d", rows[2].entryIdx)
	}
	if !rows[3].isFileRow() || rows[3].entryIdx != 1 {
		t.Errorf("row 3: expected file entry 1, got entryIdx=%d", rows[3].entryIdx)
	}
	if !rows[4].isDirHeader || rows[4].label != "  internal/tui/" {
		t.Errorf("row 4: expected dir header 'internal/tui/', got %q", rows[4].label)
	}
	if !rows[5].isFileRow() || rows[5].entryIdx != 2 {
		t.Errorf("row 5: expected file entry 2, got entryIdx=%d", rows[5].entryIdx)
	}

	// Verify reverse map points to file rows
	if reverseMap[0] != 2 {
		t.Errorf("expected reverseMap[0]=2, got %d", reverseMap[0])
	}
	if reverseMap[1] != 3 {
		t.Errorf("expected reverseMap[1]=3, got %d", reverseMap[1])
	}
	if reverseMap[2] != 5 {
		t.Errorf("expected reverseMap[2]=5, got %d", reverseMap[2])
	}
}

func TestBuildGitVisualRows_MixedRootAndDir(t *testing.T) {
	t.Parallel()
	entries := fillPathFields([]changedFileEntry{
		{name: "go.mod", status: git.StatusModified, category: categoryUnstaged},
		{name: "internal/tui/model.go", status: git.StatusModified, category: categoryUnstaged},
	})
	rows, _ := buildGitVisualRows(entries)

	// 1 category header + 1 dir header (./) + 1 file + 1 dir header + 1 file = 5 rows
	if len(rows) != 5 {
		t.Fatalf("expected 5 rows, got %d", len(rows))
	}
	// Row 0: category header
	if !rows[0].isHeader {
		t.Error("row 0: expected category header")
	}
	// Row 1: root dir header
	if !rows[1].isDirHeader || rows[1].label != "  ./" {
		t.Errorf("row 1: expected dir header './', got %q", rows[1].label)
	}
	// Row 2: root file
	if !rows[2].isFileRow() || rows[2].entryIdx != 0 {
		t.Errorf("row 2: expected root file entry 0, got entryIdx=%d", rows[2].entryIdx)
	}
	// Row 3: dir header for internal/tui
	if !rows[3].isDirHeader {
		t.Errorf("row 3: expected dir header, got isHeader=%v isDirHeader=%v",
			rows[3].isHeader, rows[3].isDirHeader)
	}
	// Row 4: file under internal/tui
	if !rows[4].isFileRow() || rows[4].entryIdx != 1 {
		t.Errorf("row 4: expected file entry 1, got entryIdx=%d", rows[4].entryIdx)
	}
}

func TestBuildGitVisualRows_DirGroupingMultiCategory(t *testing.T) {
	t.Parallel()
	entries := fillPathFields([]changedFileEntry{
		{name: "internal/git/diff.go", status: git.StatusModified, category: categoryStaged},
		{name: "internal/git/status.go", status: git.StatusModified, category: categoryUnstaged},
	})
	rows, _ := buildGitVisualRows(entries)

	// Staged: 1 cat header + 1 dir header + 1 file = 3
	// Unstaged: 1 cat header + 1 dir header + 1 file = 3
	// Total: 6
	if len(rows) != 6 {
		t.Fatalf("expected 6 rows, got %d", len(rows))
	}
	// Same directory appears in both categories
	if !rows[1].isDirHeader || rows[1].label != "  internal/git/" {
		t.Errorf("row 1: expected staged dir header, got %q", rows[1].label)
	}
	if !rows[4].isDirHeader || rows[4].label != "  internal/git/" {
		t.Errorf("row 4: expected unstaged dir header, got %q", rows[4].label)
	}
}

func TestGitCursorHelpers_WithDirHeaders(t *testing.T) {
	t.Parallel()
	rows := []gitVisualRow{
		{isHeader: true, label: "Changes"},
		{isDirHeader: true, label: "internal/git/"},
		{entryIdx: 0},
		{isDirHeader: true, label: "internal/tui/"},
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

func TestGitVisualRow_IsFileRow(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		row  gitVisualRow
		want bool
	}{
		{"file row", gitVisualRow{entryIdx: 0}, true},
		{"category header", gitVisualRow{isHeader: true}, false},
		{"dir header", gitVisualRow{isDirHeader: true}, false},
	}
	for _, tt := range tests {
		if got := tt.row.isFileRow(); got != tt.want {
			t.Errorf("%s: isFileRow() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestAutoOpenFirstDiff(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.initialDiffAutoOpened = false
	m.activePanel = panelGitDiff
	m.gitDiffMode = gitModeBranch

	entries := fillPathFields([]changedFileEntry{
		{
			name:       "main.go",
			status:     git.StatusModified,
			absPath:    "/tmp/main.go",
			oldContent: []string{"old"},
			newContent: []string{"new"},
			category:   categoryUnstaged,
		},
	})

	m.Update(gitChangedFilesMsg{mode: gitModeBranch, entries: entries})

	if !m.initialDiffAutoOpened {
		t.Fatal("expected initialDiffAutoOpened=true")
	}
	if len(m.tabs) != 1 {
		t.Fatalf("expected 1 tab, got %d", len(m.tabs))
	}
	if m.tabs[0].kind != diffTab {
		t.Errorf("expected diffTab, got %d", m.tabs[0].kind)
	}
	if m.focusPane != paneEditor {
		t.Errorf("expected paneEditor, got %d", m.focusPane)
	}
}

func TestAutoOpenFirstDiff_SkipsBinary(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.initialDiffAutoOpened = false
	m.activePanel = panelGitDiff
	m.gitDiffMode = gitModeBranch

	entries := fillPathFields([]changedFileEntry{
		{
			name:    "image.png",
			status:  git.StatusAdded,
			absPath: "/tmp/image.png",
			binary:  true,
		},
		{
			name:       "util.go",
			status:     git.StatusModified,
			absPath:    "/tmp/util.go",
			oldContent: []string{"old"},
			newContent: []string{"new"},
			category:   categoryUnstaged,
		},
	})

	m.Update(gitChangedFilesMsg{mode: gitModeBranch, entries: entries})

	if len(m.tabs) != 1 {
		t.Fatalf("expected 1 tab (skipping binary), got %d", len(m.tabs))
	}
	if m.tabs[0].filePath != "/tmp/util.go" {
		t.Errorf("expected util.go tab, got %s", m.tabs[0].filePath)
	}
}

func TestAutoOpenFirstDiff_NoEntries(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.initialDiffAutoOpened = false
	m.activePanel = panelGitDiff
	m.gitDiffMode = gitModeBranch

	m.Update(gitChangedFilesMsg{mode: gitModeBranch, entries: nil})

	if !m.initialDiffAutoOpened {
		t.Fatal("expected initialDiffAutoOpened=true even with no entries")
	}
	if len(m.tabs) != 0 {
		t.Fatalf("expected 0 tabs for empty entries, got %d", len(m.tabs))
	}
}

func TestAutoOpenFirstDiff_OnlyOnce(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.initialDiffAutoOpened = false
	m.activePanel = panelGitDiff
	m.gitDiffMode = gitModeBranch

	entries := fillPathFields([]changedFileEntry{
		{
			name:       "a.go",
			status:     git.StatusModified,
			absPath:    "/tmp/a.go",
			oldContent: []string{"old"},
			newContent: []string{"new"},
			category:   categoryUnstaged,
		},
	})

	m.Update(gitChangedFilesMsg{mode: gitModeBranch, entries: entries})
	if len(m.tabs) != 1 {
		t.Fatalf("expected 1 tab after first load, got %d", len(m.tabs))
	}

	// Close the tab and send another branch result
	m.tabs = m.tabs[:0]
	m.activeTab = 0

	entries2 := fillPathFields([]changedFileEntry{
		{
			name:       "b.go",
			status:     git.StatusAdded,
			absPath:    "/tmp/b.go",
			newContent: []string{"new"},
			category:   categoryUnstaged,
		},
	})
	m.Update(gitChangedFilesMsg{mode: gitModeBranch, entries: entries2})

	if len(m.tabs) != 0 {
		t.Fatalf("expected 0 tabs (auto-open should not fire again), got %d", len(m.tabs))
	}
}

func TestGitDiffModeTabPrefix(t *testing.T) {
	t.Parallel()
	tests := []struct {
		mode          gitDiffMode
		defaultBranch string
		want          string
	}{
		{gitModeWorking, "", "[working]"},
		{gitModeBranch, "main", "[vs main]"},
		{gitModeBranch, "master", "[vs master]"},
		{gitModeBranch, "", "[vs main]"},
	}
	for _, tt := range tests {
		if got := tt.mode.tabPrefix(tt.defaultBranch); got != tt.want {
			t.Errorf("gitDiffMode(%d).tabPrefix(%q) = %q, want %q", tt.mode, tt.defaultBranch, got, tt.want)
		}
	}
}

func TestCategoryStats(t *testing.T) {
	t.Parallel()
	entries := []changedFileEntry{
		{category: categoryStaged, stats: diff.Stats{Additions: 5, Deletions: 2, Modified: 1}},
		{category: categoryStaged, stats: diff.Stats{Additions: 3}},
		{category: categoryUnstaged, stats: diff.Stats{Additions: 10, Deletions: 1, Modified: 2}},
	}

	s := categoryStats(entries, categoryStaged)
	if s.Additions != 8 || s.Deletions != 2 || s.Modified != 1 {
		t.Errorf("staged: got +%d -%d ~%d, want +8 -2 ~1", s.Additions, s.Deletions, s.Modified)
	}

	s = categoryStats(entries, categoryUnstaged)
	if s.Additions != 10 || s.Deletions != 1 || s.Modified != 2 {
		t.Errorf("unstaged: got +%d -%d ~%d, want +10 -1 ~2", s.Additions, s.Deletions, s.Modified)
	}

	s = categoryStats(entries, categoryUntracked)
	if s.Additions != 0 || s.Deletions != 0 || s.Modified != 0 {
		t.Errorf("untracked: got +%d -%d ~%d, want all zero", s.Additions, s.Deletions, s.Modified)
	}
}

func TestRenderModeSelector(t *testing.T) {
	t.Parallel()
	result := renderModeSelector(gitModeWorking, "main", render.Dark)
	if result == "" {
		t.Fatal("expected non-empty mode selector")
	}
}
