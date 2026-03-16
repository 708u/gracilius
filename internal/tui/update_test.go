package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/708u/gracilius/internal/diff"
	"github.com/charmbracelet/x/ansi"
)

func TestHandleOpenDiff(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)

	msg := OpenDiffMsg{
		FilePath: "diff.go",
		Contents: "line1\nline2\nline3",
		Accept:   func(string) {},
		Reject:   func() {},
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
	t.Parallel()
	m := newTestModel(t)

	// Add a file tab and a diff tab.
	ft := newFileTab()
	ft.filePath = "file.go"
	ft.lines = []string{"hello"}
	m.tabs = append(m.tabs, ft)

	dt := newDiffTab("diff.go", []string{"diff1", "diff2"}, func(string) {}, func() {})
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
	t.Parallel()
	content := "line1\nline2\nline3"
	m := newTestModelWithFile(t, content)

	// Move cursor to end.
	tab := m.tabs[0]
	tab.cursorLine = 2
	tab.cursorChar = 5

	// Simulate file change with fewer lines.
	m.Update(fileChangedMsg{path: tab.filePath, lines: []string{"only one line"}})

	if tab.cursorLine != 0 {
		t.Errorf("expected cursorLine clipped to 0, got %d", tab.cursorLine)
	}
}

func TestHandleFileChanged_UpdatesDiffTab(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)

	// Create a diff tab with known old/new content.
	oldLines := []string{"old1", "old2"}
	newLines := []string{"new1", "new2", "new3"}
	dt := newDiffTab("/workspace/file.go", newLines, func(string) {}, func() {})
	dt.diffViewData = diff.Build(oldLines, newLines)
	dt.diffOldSource = "old1\nold2"
	m.tabs = append(m.tabs, dt)
	m.activeTab = 0

	// Simulate the on-disk file changing.
	updatedOldLines := []string{"updated1", "updated2", "updated3"}
	m.Update(fileChangedMsg{
		path:  "/workspace/file.go",
		lines: updatedOldLines,
	})

	if dt.diffOldSource != "updated1\nupdated2\nupdated3" {
		t.Errorf("expected diffOldSource updated, got %q", dt.diffOldSource)
	}
	if dt.diffViewData == nil {
		t.Fatal("expected diffViewData to be rebuilt")
	}
}

func TestHandleFileChanged_IgnoresUnrelatedPath(t *testing.T) {
	t.Parallel()
	content := "line1\nline2\nline3"
	m := newTestModelWithFile(t, content)
	tab := m.tabs[0]

	// Send a change for a different file.
	m.Update(fileChangedMsg{
		path:  "/some/other/file.go",
		lines: []string{"changed"},
	})

	// The active tab should be untouched.
	if len(tab.lines) != 3 {
		t.Errorf("expected 3 lines unchanged, got %d", len(tab.lines))
	}
}

func TestHandleWindowSize(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	content := "line1\nline2\nline3\nline4\nline5"
	m := newTestModelWithFile(t, content)
	tab := m.tabs[0]
	tab.cursorLine = 0

	srv := m.server.(*mockServer)

	// Move down.
	m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if tab.cursorLine != 1 {
		t.Errorf("expected cursorLine=1 after down, got %d", tab.cursorLine)
	}

	n, ok := srv.lastNotification()
	if !ok {
		t.Fatal("expected notification after cursor move")
	}
	if n.startLine != 1 {
		t.Errorf("expected notification startLine=1, got %d", n.startLine)
	}

	// Move up.
	m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if tab.cursorLine != 0 {
		t.Errorf("expected cursorLine=0 after up, got %d", tab.cursorLine)
	}
}

func TestMouseClick_TreeEntry(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.fileTree = []fileEntry{
		{path: "dir1", name: "dir1", isDir: true, depth: 0},
		{path: "file1.go", name: "file1.go", isDir: false, depth: 0},
	}

	// Click on second tree entry: panelBodyY + 1
	// panelBodyY = contentStartY + 1 (panel header takes 1 row)
	panelBodyY := contentStartY + 1
	m.Update(tea.MouseClickMsg{
		X:      5,
		Y:      panelBodyY + 1,
		Button: tea.MouseLeft,
	})

	if m.treeCursor != 1 {
		t.Errorf("expected treeCursor=1, got %d", m.treeCursor)
	}
}

func TestAcceptDiff_CallsOnAccept(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)

	var accepted bool
	var acceptedContents string
	dt := newDiffTab("/workspace/file.go",
		[]string{"line1", "line2"},
		func(contents string) {
			accepted = true
			acceptedContents = contents
		},
		func() { t.Error("reject should not be called") },
	)
	m.tabs = append(m.tabs, dt)
	m.activeTab = 0
	m.focusPane = paneEditor

	msg := tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter})
	m.Update(msg)

	if !accepted {
		t.Fatal("onAccept should have been called")
	}
	if acceptedContents != "line1\nline2" {
		t.Fatalf("expected 'line1\\nline2', got %q", acceptedContents)
	}
	if len(m.tabs) != 0 {
		t.Fatalf("expected 0 tabs, got %d", len(m.tabs))
	}
}

func TestRejectDiff_CallsOnReject(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)

	var rejected bool
	dt := newDiffTab("/workspace/file.go",
		[]string{"line1"},
		func(string) { t.Error("accept should not be called") },
		func() { rejected = true },
	)
	m.tabs = append(m.tabs, dt)
	m.activeTab = 0
	m.focusPane = paneEditor

	msg := tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape})
	m.Update(msg)

	if !rejected {
		t.Fatal("onReject should have been called")
	}
	if len(m.tabs) != 0 {
		t.Fatalf("expected 0 tabs, got %d", len(m.tabs))
	}
}

func TestCloseTab_CallsOnReject(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)

	var rejected bool
	dt := newDiffTab("/workspace/file.go",
		[]string{"line1"},
		func(string) { t.Error("accept should not be called") },
		func() { rejected = true },
	)
	m.tabs = append(m.tabs, dt)
	m.activeTab = 0
	m.focusPane = paneEditor

	msg := tea.KeyPressMsg(tea.Key{Code: 'q', Text: "q"})
	m.Update(msg)

	if !rejected {
		t.Fatal("onReject should have been called when closing diff tab with q")
	}
	if len(m.tabs) != 0 {
		t.Fatalf("expected 0 tabs, got %d", len(m.tabs))
	}
}

func TestCloseDiffTabs_CallsOnReject(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)

	var rejectCount int
	for range 3 {
		dt := newDiffTab("/workspace/file.go",
			[]string{"line1"},
			func(string) {},
			func() { rejectCount++ },
		)
		m.tabs = append(m.tabs, dt)
	}
	m.activeTab = 0

	m.Update(CloseDiffMsg{})

	if rejectCount != 3 {
		t.Fatalf("expected 3 rejects, got %d", rejectCount)
	}
	if len(m.tabs) != 0 {
		t.Fatalf("expected 0 tabs, got %d", len(m.tabs))
	}
}

func TestCloseDiffTabs_PreservesLocalDiffTabs(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)

	// File tab.
	ft := newFileTab()
	ft.filePath = "file.go"
	ft.lines = []string{"hello"}
	m.tabs = append(m.tabs, ft)

	// MCP diff tab (has diff state).
	var rejected bool
	mcpDt := newDiffTab("/workspace/mcp.go",
		[]string{"mcp1"},
		func(string) {},
		func() { rejected = true },
	)
	m.tabs = append(m.tabs, mcpDt)

	// Git panel diff tab (no diff state, like openGitDiffEntry).
	gitDt := &tab{
		kind:         diffTab,
		filePath:     "/workspace/local.go",
		lines:        []string{"local1"},
		commentInput: newTextarea(),
		vp:           newViewport(),
	}
	m.tabs = append(m.tabs, gitDt)
	m.activeTab = 0

	m.Update(CloseDiffMsg{})

	if !rejected {
		t.Fatal("expected MCP diff tab onReject to be called")
	}
	if len(m.tabs) != 2 {
		t.Fatalf("expected 2 tabs (file + local diff), got %d", len(m.tabs))
	}
	if m.tabs[0] != ft {
		t.Error("expected first tab to be the file tab")
	}
	if m.tabs[1] != gitDt {
		t.Error("expected second tab to be the git panel diff tab")
	}
}

func TestCommentSubmit_EnterSavesComment_Enhanced(t *testing.T) {
	t.Parallel()
	content := "line1\nline2\nline3"
	m := newTestModelWithFile(t, content)
	m.enhancedKeyboard = true
	tab := m.tabs[0]

	tab.inputMode = true
	tab.inputStart = 0
	tab.inputEnd = 0
	tab.commentInput.Focus()
	tab.commentInput.SetValue("test comment")

	msg := tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter})
	m.Update(msg)

	if tab.inputMode {
		t.Fatal("expected inputMode=false after Enter submit")
	}
	comments, err := m.commentRepo.List("", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(comments) != 1 {
		t.Fatalf("expected 1 comment in store, got %d", len(comments))
	}
	if comments[0].Text != "test comment" {
		t.Errorf("expected comment text 'test comment', got %q",
			comments[0].Text)
	}
}

func TestCommentSubmit_EnterInsertsNewline_Basic(t *testing.T) {
	t.Parallel()
	content := "line1\nline2\nline3"
	m := newTestModelWithFile(t, content)
	m.enhancedKeyboard = false
	tab := m.tabs[0]

	tab.inputMode = true
	tab.inputStart = 0
	tab.inputEnd = 0
	tab.commentInput.Focus()
	tab.commentInput.SetValue("first line")

	msg := tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter})
	m.Update(msg)

	if !tab.inputMode {
		t.Fatal("expected inputMode=true: Enter should not submit without enhanced keyboard")
	}
}

func TestCommentSubmit_ShiftEnterInsertsNewline_Enhanced(t *testing.T) {
	t.Parallel()
	content := "line1\nline2\nline3"
	m := newTestModelWithFile(t, content)
	m.enhancedKeyboard = true
	tab := m.tabs[0]

	tab.inputMode = true
	tab.inputStart = 0
	tab.inputEnd = 0
	tab.commentInput.Focus()
	tab.commentInput.SetValue("first line")

	msg := tea.KeyPressMsg(tea.Key{
		Code: tea.KeyEnter,
		Mod:  tea.ModShift,
	})
	m.Update(msg)

	if !tab.inputMode {
		t.Fatal("expected inputMode=true after Shift+Enter")
	}
	comments, err := m.commentRepo.List("", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(comments) != 0 {
		t.Fatalf("expected 0 comments (not submitted), got %d",
			len(comments))
	}
}

func TestCommentSubmit_CtrlDSavesComment(t *testing.T) {
	t.Parallel()
	content := "line1\nline2\nline3"
	m := newTestModelWithFile(t, content)
	tab := m.tabs[0]

	tab.inputMode = true
	tab.inputStart = 1
	tab.inputEnd = 1
	tab.commentInput.Focus()
	tab.commentInput.SetValue("ctrl-d comment")

	msg := tea.KeyPressMsg(tea.Key{Code: 'd', Mod: tea.ModCtrl})
	m.Update(msg)

	if tab.inputMode {
		t.Fatal("expected inputMode=false after Ctrl+D submit")
	}
	comments, err := m.commentRepo.List("", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(comments) != 1 {
		t.Fatalf("expected 1 comment in store, got %d", len(comments))
	}
	if comments[0].Text != "ctrl-d comment" {
		t.Errorf("expected 'ctrl-d comment', got %q",
			comments[0].Text)
	}
}

func TestAcceptDiff_NotCalledOnFileTab(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)

	ft := newFileTab()
	ft.lines = []string{"line1"}
	m.tabs = append(m.tabs, ft)
	m.activeTab = 0
	m.focusPane = paneEditor

	msg := tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter})
	m.Update(msg)

	if len(m.tabs) != 1 {
		t.Fatal("file tab should not be closed by accept key")
	}
}

func TestContextKeyMap_DiffReviewBindings(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)

	dt := newDiffTab("/workspace/file.go",
		[]string{"line1"},
		func(string) {},
		func() {},
	)
	m.tabs = append(m.tabs, dt)
	m.activeTab = 0
	m.focusPane = paneEditor

	km := m.contextKeyMap().(keyMap)

	if !km.AcceptDiff.Enabled() {
		t.Fatal("AcceptDiff should be enabled for diff review tab")
	}
	if !km.RejectDiff.Enabled() {
		t.Fatal("RejectDiff should be enabled for diff review tab")
	}
}

func TestTabIndexAtX(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)

	// No tabs: always -1.
	if got := m.tabIndexAtX(40); got != -1 {
		t.Errorf("no tabs: expected -1, got %d", got)
	}

	// Add two tabs.
	t1 := newFileTab()
	t1.filePath = "/workspace/main.go"
	t2 := newFileTab()
	t2.filePath = "/workspace/util.go"
	m.tabs = []*tab{t1, t2}

	lo := m.computeLayout()
	// Tab 0 label: " main.go " (9 runes), starts at editorStartX.
	label0 := tabLabel(t1)
	w0 := ansi.StringWidth(label0)

	// Click on first tab start.
	if got := m.tabIndexAtX(lo.editorStartX); got != 0 {
		t.Errorf("first tab start: expected 0, got %d", got)
	}
	// Click on first tab end - 1.
	if got := m.tabIndexAtX(lo.editorStartX + w0 - 1); got != 0 {
		t.Errorf("first tab end-1: expected 0, got %d", got)
	}

	// Second tab starts at editorStartX + w0 + 1 (separator).
	secondStart := lo.editorStartX + w0 + 1
	if got := m.tabIndexAtX(secondStart); got != 1 {
		t.Errorf("second tab start: expected 1, got %d", got)
	}

	// Click before tabs.
	if got := m.tabIndexAtX(0); got != -1 {
		t.Errorf("before tabs: expected -1, got %d", got)
	}

	// Click after all tabs.
	label1 := tabLabel(t2)
	w1 := ansi.StringWidth(label1)
	afterAll := secondStart + w1
	if got := m.tabIndexAtX(afterAll); got != -1 {
		t.Errorf("after all tabs: expected -1, got %d", got)
	}
}

func TestMouseClick_TabBar(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)

	t1 := newFileTab()
	t1.filePath = "/workspace/main.go"
	t1.lines = []string{"package main"}
	t2 := newFileTab()
	t2.filePath = "/workspace/util.go"
	t2.lines = []string{"package util"}
	m.tabs = []*tab{t1, t2}
	m.activeTab = 0

	lo := m.computeLayout()
	label0 := tabLabel(t1)
	w0 := ansi.StringWidth(label0)
	secondTabX := lo.editorStartX + w0 + 1

	// Click on second tab (Y = headerHeight, the label row).
	m.Update(tea.MouseClickMsg{
		X:      secondTabX,
		Y:      headerHeight,
		Button: tea.MouseLeft,
	})

	if m.activeTab != 1 {
		t.Errorf("expected activeTab=1 after click, got %d", m.activeTab)
	}

	// Click on first tab (Y = headerHeight+1, the underline row).
	m.Update(tea.MouseClickMsg{
		X:      lo.editorStartX,
		Y:      headerHeight + 1,
		Button: tea.MouseLeft,
	})

	if m.activeTab != 0 {
		t.Errorf("expected activeTab=0 after click, got %d", m.activeTab)
	}
}

func TestContextKeyMap_NoDiffReviewOnFileTab(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)

	ft := newFileTab()
	ft.lines = []string{"line1"}
	m.tabs = append(m.tabs, ft)
	m.activeTab = 0
	m.focusPane = paneEditor

	km := m.contextKeyMap().(keyMap)

	if km.AcceptDiff.Enabled() {
		t.Fatal("AcceptDiff should be disabled for file tab")
	}
	if km.RejectDiff.Enabled() {
		t.Fatal("RejectDiff should be disabled for file tab")
	}
}

func TestDiffSide_DefaultIsNew(t *testing.T) {
	t.Parallel()
	m := newTestModelWithDiff(t,
		[]string{"same", "old"},
		[]string{"same", "new"},
	)
	tab := m.tabs[0]
	if tab.diffSide != diffSideNew {
		t.Errorf("expected default diffSide=diffSideNew, got %d", tab.diffSide)
	}
}

func TestDiffSide_HLSwitchesSide(t *testing.T) {
	t.Parallel()
	// unchanged row: both sides available → h/l should switch.
	m := newTestModelWithDiff(t,
		[]string{"same line"},
		[]string{"same line"},
	)
	tab := m.tabs[0]
	// Cursor starts on the unchanged row.
	tab.diffCursor = 0
	tab.diffSide = diffSideNew

	// Press h → old side.
	m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	if tab.diffSide != diffSideOld {
		t.Errorf("expected diffSideOld after h, got %d", tab.diffSide)
	}

	// Press l → new side.
	m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	if tab.diffSide != diffSideNew {
		t.Errorf("expected diffSideNew after l, got %d", tab.diffSide)
	}
}

func TestDiffSide_AutoSnapDeleted(t *testing.T) {
	t.Parallel()
	// old has "deleted", new does not → deleted row must snap to old.
	m := newTestModelWithDiff(t,
		[]string{"same", "deleted"},
		[]string{"same"},
	)
	tab := m.tabs[0]
	tab.diffSide = diffSideNew

	// Find the deleted row.
	deletedIdx := -1
	for i, row := range tab.diffViewData.Rows {
		if row.Type == diff.RowDeleted {
			deletedIdx = i
			break
		}
	}
	if deletedIdx < 0 {
		t.Fatal("expected a deleted row in diff data")
	}

	// Move cursor to the deleted row.
	tab.diffCursor = deletedIdx
	tab.snapDiffSide()

	if tab.diffSide != diffSideOld {
		t.Errorf("expected auto-snap to diffSideOld on deleted row, got %d", tab.diffSide)
	}
}

func TestDiffSide_AutoSnapAdded(t *testing.T) {
	t.Parallel()
	// new has "added", old does not → added row must snap to new.
	m := newTestModelWithDiff(t,
		[]string{"same"},
		[]string{"same", "added"},
	)
	tab := m.tabs[0]
	tab.diffSide = diffSideOld

	// Find the added row.
	addedIdx := -1
	for i, row := range tab.diffViewData.Rows {
		if row.Type == diff.RowAdded {
			addedIdx = i
			break
		}
	}
	if addedIdx < 0 {
		t.Fatal("expected an added row in diff data")
	}

	tab.diffCursor = addedIdx
	tab.snapDiffSide()

	if tab.diffSide != diffSideNew {
		t.Errorf("expected auto-snap to diffSideNew on added row, got %d", tab.diffSide)
	}
}

func TestDiffSide_NoSwitchOnDeletedRow(t *testing.T) {
	t.Parallel()
	// On a deleted row, pressing l (right) should jump to the nearest
	// row with new-side content.
	m := newTestModelWithDiff(t,
		[]string{"ctx", "deleted", "end"},
		[]string{"ctx", "end"},
	)
	tab := m.tabs[0]

	deletedIdx := -1
	for i, row := range tab.diffViewData.Rows {
		if row.Type == diff.RowDeleted && deletedIdx < 0 {
			deletedIdx = i
		}
	}
	if deletedIdx < 0 {
		t.Fatal("expected a deleted row in diff data")
	}

	tab.diffCursor = deletedIdx
	tab.diffSide = diffSideOld

	// Press l → should jump to nearest row with new-side content.
	m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	if tab.diffSide != diffSideNew {
		t.Errorf("expected diffSideNew after l on deleted row, got %d", tab.diffSide)
	}
	row := tab.diffViewData.Rows[tab.diffCursor]
	if diffRowAvailableSide(row, diffSideNew) != diffSideNew {
		t.Errorf("cursor row %d has no new-side content", tab.diffCursor)
	}
}

func TestDiffSide_MouseClick_OldSide(t *testing.T) {
	t.Parallel()
	m := newTestModelWithDiff(t,
		[]string{"same line"},
		[]string{"same line"},
	)
	tab := m.tabs[0]
	tab.diffSide = diffSideNew

	lo := m.computeLayout()

	// Click on left half of editor (old side).
	m.Update(tea.MouseClickMsg{
		X:      lo.editorStartX + 1,
		Y:      contentStartY,
		Button: tea.MouseLeft,
	})

	if tab.diffSide != diffSideOld {
		t.Errorf("expected diffSideOld after clicking left side, got %d", tab.diffSide)
	}
}

func TestDiffSide_MouseClick_NewSide(t *testing.T) {
	t.Parallel()
	m := newTestModelWithDiff(t,
		[]string{"same line"},
		[]string{"same line"},
	)
	tab := m.tabs[0]
	tab.diffSide = diffSideOld

	lo := m.computeLayout()

	// Click on right half of editor (new side).
	sideWidth := (lo.editorWidth - diffSeparatorWidth) / 2
	m.Update(tea.MouseClickMsg{
		X:      lo.editorStartX + sideWidth + diffSeparatorWidth + 1,
		Y:      contentStartY,
		Button: tea.MouseLeft,
	})

	if tab.diffSide != diffSideNew {
		t.Errorf("expected diffSideNew after clicking right side, got %d", tab.diffSide)
	}
}

func TestDiffSide_SelectionTextMatchesSide(t *testing.T) {
	t.Parallel()
	// Modified row: old="hello", new="world"
	m := newTestModelWithDiff(t,
		[]string{"hello"},
		[]string{"world"},
	)
	tab := m.tabs[0]

	// Find the modified row.
	modIdx := -1
	for i, row := range tab.diffViewData.Rows {
		if row.Type == diff.RowModified {
			modIdx = i
			break
		}
	}
	if modIdx < 0 {
		t.Fatal("expected a modified row in diff data")
	}

	tab.diffCursor = modIdx
	tab.diffAnchor = modIdx

	// Old side.
	tab.diffSide = diffSideOld
	oldText := tab.diffSelectedText()
	if oldText != "hello" {
		t.Errorf("expected 'hello' for old side, got %q", oldText)
	}

	// New side.
	tab.diffSide = diffSideNew
	newText := tab.diffSelectedText()
	if newText != "world" {
		t.Errorf("expected 'world' for new side, got %q", newText)
	}
}

func TestDiffSide_ChangeJumpSkipsWrongSide(t *testing.T) {
	t.Parallel()
	// old: [same, old1, old2]  →  deleted block + added block
	// new: [same, new1, new2]
	// Two change blocks: one with modified rows, then done.
	// But a more targeted test: block has [modified, added].
	// On old side, ] from the modified row should stay on the
	// modified row (last matching row), not jump to the added row.
	m := newTestModelWithDiff(t,
		[]string{"old1", "old2"},
		[]string{"new1", "new2", "added"},
	)
	tab := m.tabs[0]
	rows := tab.diffViewData.Rows

	// Find first modified row and the added row.
	modIdx := -1
	addedIdx := -1
	for i, row := range rows {
		if row.Type == diff.RowModified && modIdx < 0 {
			modIdx = i
		}
		if row.Type == diff.RowAdded {
			addedIdx = i
		}
	}
	if modIdx < 0 || addedIdx < 0 {
		t.Fatalf("expected modified and added rows, got mod=%d added=%d", modIdx, addedIdx)
	}

	// Start on first modified row, old side.
	tab.diffCursor = modIdx
	tab.diffSide = diffSideOld

	// Press ] — should land on last modified row, NOT the added row.
	m.Update(tea.KeyPressMsg{Code: ']', Text: "]"})

	if tab.diffSide != diffSideOld {
		t.Errorf("expected diffSideOld preserved after ], got %d", tab.diffSide)
	}
	if tab.diffCursor == addedIdx {
		t.Errorf("cursor should not land on added-only row %d when on old side", addedIdx)
	}
	// Verify cursor is on a row that has old-side content.
	if diffRowAvailableSide(rows[tab.diffCursor], diffSideOld) != diffSideOld {
		t.Errorf("cursor row %d does not have old-side content", tab.diffCursor)
	}
}

func TestDiffSide_ChangeJumpSkipsAddedOnlyBlock(t *testing.T) {
	t.Parallel()
	// old: [same1, old1, same2]
	// new: [same1, new1, same2, added1, added2]
	// Block 1: modified (old1→new1). Block 2: added-only (added1, added2).
	// On old side, pressing ] from block 1's end should NOT land in block 2.
	m := newTestModelWithDiff(t,
		[]string{"same1", "old1", "same2"},
		[]string{"same1", "new1", "same2", "added1", "added2"},
	)
	tab := m.tabs[0]
	rows := tab.diffViewData.Rows

	// Find the modified row.
	modIdx := -1
	for i, row := range rows {
		if row.Type == diff.RowModified {
			modIdx = i
			break
		}
	}
	if modIdx < 0 {
		t.Fatal("expected a modified row")
	}

	tab.diffCursor = modIdx
	tab.diffSide = diffSideOld

	// Press ] — should not move to the added-only block.
	m.Update(tea.KeyPressMsg{Code: ']', Text: "]"})

	// Cursor should stay on the modified row (no further matching block).
	if tab.diffCursor != modIdx {
		row := rows[tab.diffCursor]
		if diffRowAvailableSide(row, diffSideOld) != diffSideOld {
			t.Errorf("cursor landed on row %d which has no old-side content", tab.diffCursor)
		}
	}
}

func TestDiffSide_JKSkipsOppositeSideRows(t *testing.T) {
	t.Parallel()
	// j/k on old side should skip RowAdded rows entirely.
	// Rows: unchanged(ctx), added(added), unchanged(end)
	m := newTestModelWithDiff(t,
		[]string{"ctx", "end"},
		[]string{"ctx", "added", "end"},
	)
	tab := m.tabs[0]
	rows := tab.diffViewData.Rows

	// Find row indices by type.
	firstUnchanged := -1
	addedIdx := -1
	lastUnchanged := -1
	for i, row := range rows {
		switch row.Type {
		case diff.RowUnchanged:
			if firstUnchanged < 0 {
				firstUnchanged = i
			}
			lastUnchanged = i
		case diff.RowAdded:
			addedIdx = i
		}
	}
	if addedIdx < 0 {
		t.Fatal("expected an added row")
	}

	// Start on first unchanged row, old side.
	tab.diffCursor = firstUnchanged
	tab.diffSide = diffSideOld

	// Press j — should skip added row and land on last unchanged.
	m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if tab.diffCursor == addedIdx {
		t.Errorf("j should skip added row %d on old side", addedIdx)
	}
	if tab.diffCursor != lastUnchanged {
		t.Errorf("expected cursor at %d (last unchanged), got %d", lastUnchanged, tab.diffCursor)
	}
	if tab.diffSide != diffSideOld {
		t.Errorf("expected diffSideOld preserved, got %d", tab.diffSide)
	}

	// Press k — should skip back over added row.
	m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if tab.diffCursor == addedIdx {
		t.Errorf("k should skip added row %d on old side", addedIdx)
	}
	if tab.diffCursor != firstUnchanged {
		t.Errorf("expected cursor at %d (first unchanged), got %d", firstUnchanged, tab.diffCursor)
	}
}

func TestDiffSide_JKSkipsDeletedOnNewSide(t *testing.T) {
	t.Parallel()
	// j/k on new side should skip RowDeleted rows.
	m := newTestModelWithDiff(t,
		[]string{"ctx", "deleted", "end"},
		[]string{"ctx", "end"},
	)
	tab := m.tabs[0]
	rows := tab.diffViewData.Rows

	firstUnchanged := -1
	deletedIdx := -1
	lastUnchanged := -1
	for i, row := range rows {
		switch row.Type {
		case diff.RowUnchanged:
			if firstUnchanged < 0 {
				firstUnchanged = i
			}
			lastUnchanged = i
		case diff.RowDeleted:
			deletedIdx = i
		}
	}
	if deletedIdx < 0 {
		t.Fatal("expected a deleted row")
	}

	tab.diffCursor = firstUnchanged
	tab.diffSide = diffSideNew

	// Press j — should skip deleted row.
	m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if tab.diffCursor == deletedIdx {
		t.Errorf("j should skip deleted row %d on new side", deletedIdx)
	}
	if tab.diffCursor != lastUnchanged {
		t.Errorf("expected cursor at %d, got %d", lastUnchanged, tab.diffCursor)
	}
}

func TestDiffSide_JKNoMoveWhenNoMoreRows(t *testing.T) {
	t.Parallel()
	// When no more rows with current side exist, cursor should not move.
	m := newTestModelWithDiff(t,
		[]string{"ctx", "end"},
		[]string{"ctx", "end", "added1", "added2"},
	)
	tab := m.tabs[0]

	// Find the last row that has old-side content.
	lastOldRow := -1
	for i, row := range tab.diffViewData.Rows {
		if diffRowAvailableSide(row, diffSideOld) == diffSideOld {
			lastOldRow = i
		}
	}
	if lastOldRow < 0 {
		t.Fatal("expected a row with old-side content")
	}

	tab.diffCursor = lastOldRow
	tab.diffSide = diffSideOld

	// Press j — only added rows ahead, should not move.
	m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if tab.diffCursor != lastOldRow {
		t.Errorf("expected cursor unchanged at %d, got %d", lastOldRow, tab.diffCursor)
	}
}

func TestDiffSide_SameSideNoOp(t *testing.T) {
	t.Parallel()
	// Pressing h when already on old side should be a no-op.
	m := newTestModelWithDiff(t,
		[]string{"same line"},
		[]string{"same line"},
	)
	tab := m.tabs[0]
	tab.diffCursor = 0
	tab.diffSide = diffSideOld

	srv := m.server.(*mockServer)
	srv.notifications = nil

	m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})

	if tab.diffSide != diffSideOld {
		t.Errorf("expected diffSideOld unchanged, got %d", tab.diffSide)
	}
	// No notification should be sent for no-op.
	if _, ok := srv.lastNotification(); ok {
		t.Error("expected no notification for same-side no-op")
	}
}

func TestDiffSide_JumpToNearestOldFromAdded(t *testing.T) {
	t.Parallel()
	// RowAdded で h → 最寄りの old 行にジャンプ
	// Need multiple lines so diff detects unchanged rows correctly.
	m := newTestModelWithDiff(t,
		[]string{"ctx", "end"},
		[]string{"ctx", "added", "end"},
	)
	tab := m.tabs[0]

	addedIdx := -1
	for i, row := range tab.diffViewData.Rows {
		if row.Type == diff.RowAdded && addedIdx < 0 {
			addedIdx = i
		}
	}
	if addedIdx < 0 {
		t.Fatal("expected an added row")
	}

	tab.diffCursor = addedIdx
	tab.diffSide = diffSideNew

	// Press h → should jump to nearest row with old-side content.
	m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	if tab.diffSide != diffSideOld {
		t.Errorf("expected diffSideOld after h on added row, got %d", tab.diffSide)
	}
	// Cursor should have moved to a row with old-side content.
	row := tab.diffViewData.Rows[tab.diffCursor]
	if diffRowAvailableSide(row, diffSideOld) != diffSideOld {
		t.Errorf("cursor row %d has no old-side content", tab.diffCursor)
	}
}

func TestDiffSide_JumpToNearestNewFromDeleted(t *testing.T) {
	t.Parallel()
	// RowDeleted で l → 最寄りの new 行にジャンプ
	m := newTestModelWithDiff(t,
		[]string{"ctx", "deleted", "end"},
		[]string{"ctx", "end"},
	)
	tab := m.tabs[0]

	deletedIdx := -1
	for i, row := range tab.diffViewData.Rows {
		if row.Type == diff.RowDeleted && deletedIdx < 0 {
			deletedIdx = i
		}
	}
	if deletedIdx < 0 {
		t.Fatal("expected a deleted row")
	}

	tab.diffCursor = deletedIdx
	tab.diffSide = diffSideOld

	m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	if tab.diffSide != diffSideNew {
		t.Errorf("expected diffSideNew after l on deleted row, got %d", tab.diffSide)
	}
	// Cursor should have moved to a row with new-side content.
	row := tab.diffViewData.Rows[tab.diffCursor]
	if diffRowAvailableSide(row, diffSideNew) != diffSideNew {
		t.Errorf("cursor row %d has no new-side content", tab.diffCursor)
	}
}

func TestDiffSide_JumpPrefersUpward(t *testing.T) {
	t.Parallel()
	// 同距離で上方向優先
	// old: [same1, deleted, same2]
	// new: [same1, same2]
	// deleted 行から l → same1 (上) と same2 (下) が等距離、上優先
	m := newTestModelWithDiff(t,
		[]string{"same1", "deleted", "same2"},
		[]string{"same1", "same2"},
	)
	tab := m.tabs[0]

	deletedIdx := -1
	firstSameIdx := -1
	for i, row := range tab.diffViewData.Rows {
		if row.Type == diff.RowDeleted && deletedIdx < 0 {
			deletedIdx = i
		}
		if row.Type == diff.RowUnchanged && firstSameIdx < 0 {
			firstSameIdx = i
		}
	}
	if deletedIdx < 0 {
		t.Fatal("expected a deleted row")
	}

	tab.diffCursor = deletedIdx
	tab.diffSide = diffSideOld

	m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	if tab.diffCursor != firstSameIdx {
		t.Errorf("expected upward preference: cursor at %d, got %d", firstSameIdx, tab.diffCursor)
	}
}

func TestDiffSide_NoJumpWhenNoTarget(t *testing.T) {
	t.Parallel()
	// 全行が added-only → h で対向行なし、no-op
	m := newTestModelWithDiff(t,
		[]string{},
		[]string{"added1", "added2"},
	)
	tab := m.tabs[0]

	// Find an added row.
	addedIdx := -1
	for i, row := range tab.diffViewData.Rows {
		if row.Type == diff.RowAdded {
			addedIdx = i
			break
		}
	}
	if addedIdx < 0 {
		t.Fatal("expected an added row")
	}

	tab.diffCursor = addedIdx
	tab.diffSide = diffSideNew
	origCursor := tab.diffCursor

	// Press h → no old-side rows exist, should be no-op.
	m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	if tab.diffCursor != origCursor {
		t.Errorf("expected cursor unchanged at %d, got %d", origCursor, tab.diffCursor)
	}
	if tab.diffSide != diffSideNew {
		t.Errorf("expected side unchanged at diffSideNew, got %d", tab.diffSide)
	}
}

func TestDiffSide_BlankLineJumpSnaps(t *testing.T) {
	t.Parallel()
	// } で RowAdded 行に着地 → side が snap される
	// old: [line1, ""]
	// new: [line1, "", added]
	m := newTestModelWithDiff(t,
		[]string{"line1", ""},
		[]string{"line1", "", "added"},
	)
	tab := m.tabs[0]

	tab.diffCursor = 0
	tab.diffSide = diffSideOld

	// Press } to jump to blank-line boundary.
	m.Update(tea.KeyPressMsg{Code: '}', Text: "}"})

	// If cursor landed on an added-only row, side must snap to new.
	if tab.diffCursor < len(tab.diffViewData.Rows) {
		row := tab.diffViewData.Rows[tab.diffCursor]
		expected := diffRowAvailableSide(row, tab.diffSide)
		if tab.diffSide != expected {
			t.Errorf("expected side to snap to %v, got %v", expected, tab.diffSide)
		}
	}
}

func TestDiffSide_BlankLineJumpPreservesSide(t *testing.T) {
	t.Parallel()
	m := newTestModelWithDiff(t,
		[]string{"line1", "", "line3"},
		[]string{"line1", "", "line3-changed"},
	)
	tab := m.tabs[0]

	tab.diffCursor = 0
	tab.diffSide = diffSideOld

	// Press } to jump to next blank-line boundary.
	m.Update(tea.KeyPressMsg{Code: '}', Text: "}"})

	if tab.diffSide != diffSideOld {
		t.Errorf("expected diffSideOld preserved after }, got %d", tab.diffSide)
	}
}
