package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestPanelLabel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		p    panel
		want string
	}{
		{panelFiles, "Files"},
		{panelGitDiff, "Git Changes"},
	}
	for _, tt := range tests {
		if got := tt.p.label(); got != tt.want {
			t.Errorf("panel(%d).label() = %q, want %q", tt.p, got, tt.want)
		}
	}
}

func TestSwitchPanel(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)

	if m.activePanel != panelFiles {
		t.Fatalf("initial panel = %d, want panelFiles", m.activePanel)
	}

	msg := tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
	m.Update(msg)
	if m.activePanel != panelGitDiff {
		t.Errorf("after 1st switch: panel = %d, want panelGitDiff", m.activePanel)
	}

	m.Update(msg)
	if m.activePanel != panelFiles {
		t.Errorf("after 2nd switch: panel = %d, want panelFiles (wrap)", m.activePanel)
	}
}

func TestToggleSidebar(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)

	if !m.sidebarVisible {
		t.Fatal("initial sidebarVisible should be true")
	}

	msg := tea.KeyPressMsg{Code: 'b', Mod: tea.ModCtrl}
	m.Update(msg)
	if m.sidebarVisible {
		t.Error("after toggle: sidebarVisible should be false")
	}

	m.Update(msg)
	if !m.sidebarVisible {
		t.Error("after 2nd toggle: sidebarVisible should be true")
	}
}

func TestToggleSidebar_ForcesEditorFocus(t *testing.T) {
	t.Parallel()
	m := newTestModelWithFile(t, "line1\nline2")
	m.focusPane = paneTree

	msg := tea.KeyPressMsg{Code: 'b', Mod: tea.ModCtrl}
	m.Update(msg)

	if m.sidebarVisible {
		t.Fatal("sidebarVisible should be false")
	}
	if m.focusPane != paneEditor {
		t.Errorf("focusPane = %d, want paneEditor when sidebar hidden", m.focusPane)
	}
}

func TestComputeLayout_SidebarHidden(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.sidebarVisible = false

	lo := m.computeLayout()
	if lo.treeWidth != 0 {
		t.Errorf("treeWidth = %d, want 0", lo.treeWidth)
	}
	if lo.editorStartX != 0 {
		t.Errorf("editorStartX = %d, want 0", lo.editorStartX)
	}
	if lo.editorWidth != m.width {
		t.Errorf("editorWidth = %d, want %d", lo.editorWidth, m.width)
	}
}

func TestComputeLayout_SidebarVisible(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.sidebarVisible = true

	lo := m.computeLayout()
	if lo.treeWidth == 0 {
		t.Error("treeWidth should be > 0 when sidebar visible")
	}
	if lo.editorStartX != lo.treeWidth+separatorWidth {
		t.Errorf("editorStartX = %d, want %d", lo.editorStartX, lo.treeWidth+separatorWidth)
	}
	if lo.editorWidth != m.width-lo.treeWidth-separatorWidth {
		t.Errorf("editorWidth = %d, want %d", lo.editorWidth, m.width-lo.treeWidth-separatorWidth)
	}
}

func TestSwitchPane_DisabledWhenHidden(t *testing.T) {
	t.Parallel()
	m := newTestModelWithFile(t, "line1\nline2")
	m.sidebarVisible = false
	m.focusPane = paneEditor

	msg := tea.KeyPressMsg{Code: tea.KeyTab}
	m.Update(msg)

	if m.focusPane != paneEditor {
		t.Errorf("focusPane = %d, want paneEditor (Tab disabled when sidebar hidden)", m.focusPane)
	}
}

func TestRenderLeftPane_LineCount(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.sidebarVisible = true

	height := 20
	lines := m.renderLeftPane(30, height)
	if len(lines) != height {
		t.Errorf("len(lines) = %d, want %d", len(lines), height)
	}
}
