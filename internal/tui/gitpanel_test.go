package tui

import (
	"fmt"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/708u/gracilius/internal/git"
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
	// Visual rows: 1 header + 2 files
	if len(gs.visualRows) != 3 {
		t.Errorf("expected 3 visual rows, got %d", len(gs.visualRows))
	}
}

func TestGitChangedFilesMsg_Error(t *testing.T) {
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
	entries := fillPathFields([]changedFileEntry{
		{name: "staged.go", status: git.StatusModified, category: categoryStaged},
		{name: "unstaged.go", status: git.StatusModified, category: categoryUnstaged},
		{name: "untracked.go", status: git.StatusUntracked, category: categoryUntracked},
	})
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
	entries := fillPathFields([]changedFileEntry{
		{name: "a.go", status: git.StatusModified, category: categoryUnstaged},
	})
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

	entries := fillPathFields([]changedFileEntry{
		{name: "staged.go", status: git.StatusModified, category: categoryStaged},
		{name: "unstaged.go", status: git.StatusModified, category: categoryUnstaged},
		{name: "new.txt", status: git.StatusUntracked, category: categoryUntracked},
	})

	m.Update(gitChangedFilesMsg{mode: gitModeWorking, entries: entries})

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
	if !rows[1].isDirHeader || rows[1].label != "    internal/git/" {
		t.Errorf("row 1: expected dir header 'internal/git/', got %q (isDirHeader=%v)",
			rows[1].label, rows[1].isDirHeader)
	}
	if !rows[2].isFileRow() || rows[2].entryIdx != 0 {
		t.Errorf("row 2: expected file entry 0, got entryIdx=%d", rows[2].entryIdx)
	}
	if !rows[3].isFileRow() || rows[3].entryIdx != 1 {
		t.Errorf("row 3: expected file entry 1, got entryIdx=%d", rows[3].entryIdx)
	}
	if !rows[4].isDirHeader || rows[4].label != "    internal/tui/" {
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
	entries := fillPathFields([]changedFileEntry{
		{name: "go.mod", status: git.StatusModified, category: categoryUnstaged},
		{name: "internal/tui/model.go", status: git.StatusModified, category: categoryUnstaged},
	})
	rows, _ := buildGitVisualRows(entries)

	// 1 category header + 0 dir header (root) + 1 file + 1 dir header + 1 file = 4 rows
	if len(rows) != 4 {
		t.Fatalf("expected 4 rows, got %d", len(rows))
	}
	// Row 0: category header
	if !rows[0].isHeader {
		t.Error("row 0: expected category header")
	}
	// Row 1: root file (no dir header)
	if !rows[1].isFileRow() || rows[1].entryIdx != 0 {
		t.Errorf("row 1: expected root file entry 0, got entryIdx=%d", rows[1].entryIdx)
	}
	// Row 2: dir header for internal/tui
	if !rows[2].isDirHeader {
		t.Errorf("row 2: expected dir header, got isHeader=%v isDirHeader=%v",
			rows[2].isHeader, rows[2].isDirHeader)
	}
	// Row 3: file under internal/tui
	if !rows[3].isFileRow() || rows[3].entryIdx != 1 {
		t.Errorf("row 3: expected file entry 1, got entryIdx=%d", rows[3].entryIdx)
	}
}

func TestBuildGitVisualRows_DirGroupingMultiCategory(t *testing.T) {
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
	if !rows[1].isDirHeader || rows[1].label != "    internal/git/" {
		t.Errorf("row 1: expected staged dir header, got %q", rows[1].label)
	}
	if !rows[4].isDirHeader || rows[4].label != "    internal/git/" {
		t.Errorf("row 4: expected unstaged dir header, got %q", rows[4].label)
	}
}

func TestGitCursorHelpers_WithDirHeaders(t *testing.T) {
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

func TestGitDiffModeTabPrefix(t *testing.T) {
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
