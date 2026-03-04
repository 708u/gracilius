package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newTestModel creates a minimal Model with mock server and temp directory.
func newTestModel(t *testing.T) *Model {
	t.Helper()
	tmpDir := t.TempDir()
	srv := &mockServer{port: 18765}
	m := &Model{
		server:    srv,
		rootDir:   tmpDir,
		tabs:      []*tab{},
		treeWidth: 30,
		keys:      newKeyMap(),
		iconMode:  iconSymbol,
		openFile:  newOpenFileOverlay(iconSymbol, darkTheme),
		width:     120,
		height:    40,
	}
	return m
}

// newTestModelWithFile creates a Model with a file tab loaded.
func newTestModelWithFile(t *testing.T, content string) *Model {
	t.Helper()
	m := newTestModel(t)

	filePath := filepath.Join(m.rootDir, "test.go")
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	ft := newFileTab()
	ft.filePath = filePath
	ft.lines = strings.Split(content, "\n")
	ft.highlightedLines = highlightFile(filePath, content, m.theme)

	m.tabs = append(m.tabs, ft)
	m.activeTab = 0
	m.focusPane = paneEditor
	return m
}
