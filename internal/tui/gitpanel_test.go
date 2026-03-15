package tui

import (
	"fmt"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/708u/gracilius/internal/git"
)

// fillPathFields populates baseName and dirName for test entries.
func fillPathFields(entries []changedFileEntry) {
	for i := range entries {
		entries[i].baseName = filepath.Base(entries[i].name)
		entries[i].dirName = filepath.Dir(entries[i].name)
	}
}

// setGitEntries sets git entries on the model and builds visual rows.
func setGitEntries(m *Model, entries []changedFileEntry) {
	fillPathFields(entries)
	m.gitChangedFiles = entries
	m.gitVisualRows, m.gitEntryToVisualIdx = buildGitVisualRows(entries)
}

func TestGitChangedFilesMsg_Populates(t *testing.T) {
	m := newTestModel(t)

	entries := []changedFileEntry{
		{name: "file1.go", status: git.StatusModified, absPath: "/tmp/file1.go",
			oldContent: []string{"old"}, newContent: []string{"new"},
			category: categoryUnstaged},
		{name: "file2.go", status: git.StatusAdded, absPath: "/tmp/file2.go",
			newContent: []string{"added"}, category: categoryUnstaged},
	}
	fillPathFields(entries)

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
	// Visual rows: 1 category header + 1 dir header (./) + 2 files = 4
	if len(m.gitVisualRows) != 4 {
		t.Errorf("expected 4 visual rows, got %d", len(m.gitVisualRows))
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
			status:     git.StatusModified,
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
		{name: "image.png", status: git.StatusModified, absPath: "/tmp/image.png", binary: true},
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
			status:     git.StatusDeleted,
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
			status:     git.StatusAdded,
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
	entries := []changedFileEntry{
		{name: "a.go", status: git.StatusModified, category: categoryUnstaged},
		{name: "b.go", status: git.StatusAdded, category: categoryUnstaged},
		{name: "c.go", status: git.StatusDeleted, category: categoryUnstaged},
	}
	setGitEntries(m, entries)
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
			status:     git.StatusModified,
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
	entries := []changedFileEntry{
		{name: "staged.go", status: git.StatusModified, category: categoryStaged},
		{name: "unstaged.go", status: git.StatusModified, category: categoryUnstaged},
		{name: "untracked.go", status: git.StatusUntracked, category: categoryUntracked},
	}
	fillPathFields(entries)
	rows, reverseMap := buildGitVisualRows(entries)

	// 3 category headers + 3 dir headers (./) + 3 files = 9 rows
	if len(rows) != 9 {
		t.Fatalf("expected 9 rows, got %d", len(rows))
	}
	if !rows[0].isHeader {
		t.Error("expected first row to be category header")
	}
	if !rows[1].isDirHeader {
		t.Error("expected second row to be dir header")
	}
	if !rows[2].isFileRow() || rows[2].entryIdx != 0 {
		t.Error("expected third row to be staged entry")
	}
	if !rows[3].isHeader {
		t.Error("expected fourth row to be category header")
	}
	if !rows[4].isDirHeader {
		t.Error("expected fifth row to be dir header")
	}
	if !rows[5].isFileRow() || rows[5].entryIdx != 1 {
		t.Error("expected sixth row to be unstaged entry")
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
	entries := []changedFileEntry{
		{name: "a.go", status: git.StatusModified, category: categoryUnstaged},
	}
	fillPathFields(entries)
	rows, _ := buildGitVisualRows(entries)
	// Only unstaged: 1 category header + 1 dir header (./) + 1 file = 3
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
	fillPathFields(entries)

	m.Update(gitChangedFilesMsg{entries: entries})

	// 3 category headers + 3 dir headers + 3 files = 9
	if len(m.gitVisualRows) != 9 {
		t.Fatalf("expected 9 visual rows (3 cat headers + 3 dir headers + 3 files), got %d",
			len(m.gitVisualRows))
	}
}

func TestOpenGitDiffEntry_DuplicateTab(t *testing.T) {
	m := newTestModel(t)
	m.activePanel = panelGitDiff

	entry := changedFileEntry{
		name:       "main.go",
		status:     git.StatusModified,
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

func TestBuildGitVisualRows_DirGrouping(t *testing.T) {
	entries := []changedFileEntry{
		{name: "internal/git/status.go", status: git.StatusModified, category: categoryUnstaged},
		{name: "internal/tui/model.go", status: git.StatusModified, category: categoryUnstaged},
		{name: "internal/tui/view.go", status: git.StatusAdded, category: categoryUnstaged},
	}
	fillPathFields(entries)
	rows, reverseMap := buildGitVisualRows(entries)

	// 1 category header + 2 dir headers (internal/git/, internal/tui/) + 3 files = 6
	if len(rows) != 6 {
		t.Fatalf("expected 6 rows, got %d", len(rows))
	}

	// [0] category header
	if !rows[0].isHeader {
		t.Error("expected row 0 to be category header")
	}
	// [1] dir header: internal/git/
	if !rows[1].isDirHeader || rows[1].label != "    internal/git/" {
		t.Errorf("expected dir header 'internal/git/', got %q isDirHeader=%v",
			rows[1].label, rows[1].isDirHeader)
	}
	// [2] file: status.go
	if !rows[2].isFileRow() || rows[2].entryIdx != 0 {
		t.Errorf("expected file row entryIdx=0, got %v", rows[2])
	}
	// [3] dir header: internal/tui/
	if !rows[3].isDirHeader || rows[3].label != "    internal/tui/" {
		t.Errorf("expected dir header 'internal/tui/', got %q isDirHeader=%v",
			rows[3].label, rows[3].isDirHeader)
	}
	// [4] file: model.go
	if !rows[4].isFileRow() || rows[4].entryIdx != 1 {
		t.Errorf("expected file row entryIdx=1, got %v", rows[4])
	}
	// [5] file: view.go
	if !rows[5].isFileRow() || rows[5].entryIdx != 2 {
		t.Errorf("expected file row entryIdx=2, got %v", rows[5])
	}

	// Verify reverse map
	if reverseMap[0] != 2 {
		t.Errorf("expected reverseMap[0]=2, got %d", reverseMap[0])
	}
	if reverseMap[1] != 4 {
		t.Errorf("expected reverseMap[1]=4, got %d", reverseMap[1])
	}
	if reverseMap[2] != 5 {
		t.Errorf("expected reverseMap[2]=5, got %d", reverseMap[2])
	}
}

func TestBuildGitVisualRows_MixedRootAndDir(t *testing.T) {
	entries := []changedFileEntry{
		{name: "go.mod", status: git.StatusModified, category: categoryUnstaged},
		{name: "internal/tui/model.go", status: git.StatusModified, category: categoryUnstaged},
	}
	fillPathFields(entries)
	rows, _ := buildGitVisualRows(entries)

	// 1 category header + 2 dir headers (./, internal/tui/) + 2 files = 5
	if len(rows) != 5 {
		t.Fatalf("expected 5 rows, got %d", len(rows))
	}

	// [1] dir header for root: ./
	if !rows[1].isDirHeader || rows[1].label != "    ./" {
		t.Errorf("expected dir header './', got %q isDirHeader=%v",
			rows[1].label, rows[1].isDirHeader)
	}
	// [3] dir header: internal/tui/
	if !rows[3].isDirHeader || rows[3].label != "    internal/tui/" {
		t.Errorf("expected dir header 'internal/tui/', got %q isDirHeader=%v",
			rows[3].label, rows[3].isDirHeader)
	}
}

func TestBuildGitVisualRows_DirGroupingMultiCategory(t *testing.T) {
	entries := []changedFileEntry{
		{name: "internal/tui/model.go", status: git.StatusModified, category: categoryStaged},
		{name: "internal/tui/view.go", status: git.StatusModified, category: categoryUnstaged},
	}
	fillPathFields(entries)
	rows, _ := buildGitVisualRows(entries)

	// Staged: 1 cat header + 1 dir header + 1 file = 3
	// Unstaged: 1 cat header + 1 dir header + 1 file = 3
	// Total: 6
	if len(rows) != 6 {
		t.Fatalf("expected 6 rows, got %d", len(rows))
	}

	// Both categories should have their own dir headers
	if !rows[0].isHeader {
		t.Error("expected row 0 to be Staged category header")
	}
	if !rows[1].isDirHeader {
		t.Error("expected row 1 to be dir header under Staged")
	}
	if !rows[3].isHeader {
		t.Error("expected row 3 to be Changes category header")
	}
	if !rows[4].isDirHeader {
		t.Error("expected row 4 to be dir header under Changes")
	}
}

func TestGitCursorHelpers_WithDirHeaders(t *testing.T) {
	rows := []gitVisualRow{
		{isHeader: true, label: "Staged"},
		{isDirHeader: true, label: "    internal/"},
		{entryIdx: 0},
		{isHeader: true, label: "Unstaged"},
		{isDirHeader: true, label: "    ./"},
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
		{
			name: "file row",
			row:  gitVisualRow{entryIdx: 0},
			want: true,
		},
		{
			name: "category header",
			row:  gitVisualRow{isHeader: true, label: "Staged"},
			want: false,
		},
		{
			name: "dir header",
			row:  gitVisualRow{isDirHeader: true, label: "    internal/"},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.row.isFileRow(); got != tt.want {
				t.Errorf("isFileRow() = %v, want %v", got, tt.want)
			}
		})
	}
}
