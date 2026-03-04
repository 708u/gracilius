package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

// mockMCPServer satisfies the MCPServer interface for testing.
type mockMCPServer struct {
	port int
}

func (m *mockMCPServer) Port() int { return m.port }
func (m *mockMCPServer) NotifySelectionChanged(string, string, int, int, int, int) {
}

func newTestModel() *Model {
	return &Model{
		server:    &mockMCPServer{port: 12345},
		width:     80,
		height:    24,
		keys:      newKeyMap(),
		focusPane: paneEditor,
		treeWidth: 30,
		tabs:      []*tab{},
	}
}

func TestAcceptDiff_CallsOnAccept(t *testing.T) {
	m := newTestModel()

	var accepted bool
	var acceptedContents string
	dt := newDiffTab("/workspace/file.go", "file.go",
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

	msg := tea.KeyPressMsg(tea.Key{Code: 'a', Text: "a"})
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
	m := newTestModel()

	var rejected bool
	dt := newDiffTab("/workspace/file.go", "file.go",
		[]string{"line1"},
		func(string) { t.Error("accept should not be called") },
		func() { rejected = true },
	)
	m.tabs = append(m.tabs, dt)
	m.activeTab = 0
	m.focusPane = paneEditor

	msg := tea.KeyPressMsg(tea.Key{Code: 'x', Text: "x"})
	m.Update(msg)

	if !rejected {
		t.Fatal("onReject should have been called")
	}
	if len(m.tabs) != 0 {
		t.Fatalf("expected 0 tabs, got %d", len(m.tabs))
	}
}

func TestCloseTab_CallsOnReject(t *testing.T) {
	m := newTestModel()

	var rejected bool
	dt := newDiffTab("/workspace/file.go", "file.go",
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
	m := newTestModel()

	var rejectCount int
	for range 3 {
		dt := newDiffTab("/workspace/file.go", "file.go",
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

func TestAcceptDiff_NotCalledOnFileTab(t *testing.T) {
	m := newTestModel()

	ft := newFileTab()
	ft.lines = []string{"line1"}
	m.tabs = append(m.tabs, ft)
	m.activeTab = 0
	m.focusPane = paneEditor

	msg := tea.KeyPressMsg(tea.Key{Code: 'a', Text: "a"})
	m.Update(msg)

	if len(m.tabs) != 1 {
		t.Fatal("file tab should not be closed by accept key")
	}
}

func TestContextKeyMap_DiffReviewBindings(t *testing.T) {
	m := newTestModel()

	dt := newDiffTab("/workspace/file.go", "file.go",
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

func TestContextKeyMap_NoDiffReviewOnFileTab(t *testing.T) {
	m := newTestModel()

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
