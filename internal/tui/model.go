package tui

import (
	"fmt"
	"path/filepath"

	"github.com/charmbracelet/bubbles/help"
	"github.com/fsnotify/fsnotify"
)

// pane identifies which pane has focus.
type pane int

const (
	paneTree pane = iota
	paneEditor
)

// MCPServer is the interface that the TUI uses to communicate with
// the WebSocket server. server.Server satisfies this implicitly.
type MCPServer interface {
	Port() int
	NotifySelectionChanged(
		filePath, text string,
		startLine, startChar, endLine, endChar int,
	)
}

// OpenDiffMsg notifies the TUI to open a diff tab.
type OpenDiffMsg struct {
	FilePath string
	Contents string
}

// CloseDiffMsg notifies the TUI to close diff tab(s).
type CloseDiffMsg struct{}

// IdeConnectedMsg notifies the TUI that Claude Code has connected.
type IdeConnectedMsg struct{}

// fileChangedMsg notifies the TUI that the watched file has changed.
type fileChangedMsg struct {
	lines []string
}

// treeChangedMsg notifies the TUI that the directory tree has changed.
type treeChangedMsg struct{}

// Model holds the entire TUI state.
type Model struct {
	width  int
	height int
	server MCPServer
	err    error

	// tabs
	tabs      []*tab
	activeTab int

	// file watcher
	watcher *fsnotify.Watcher

	// file tree
	fileTree   []fileEntry
	treeCursor int
	focusPane  pane // 0: tree, 1: editor
	rootDir    string

	// mouse
	lastMouseLine int
	lastMouseChar int
	resizingPane  bool
	treeWidth     int

	// tree scroll
	treeScrollOffset int

	// directory watcher
	dirWatcher *fsnotify.Watcher

	// keybindings
	keys keyMap
	help help.Model

	// quit confirmation
	quitPending bool

	// status message (temporary, auto-cleared)
	statusMsg string

	// icon display mode
	iconMode iconMode
}

// activeTabState returns the active tab, or nil if no tabs exist.
// TODO: return with bool
func (m *Model) activeTabState() *tab {
	if len(m.tabs) == 0 {
		return nil
	}
	return m.tabs[m.activeTab]
}

// findTabByPath returns the index of the tab with the given file path,
// or -1 if not found.
func (m *Model) findTabByPath(path string) int {
	for i, t := range m.tabs {
		if t.filePath == path {
			return i
		}
	}
	return -1
}

// toggleTreeEntry handles expanding/collapsing dirs or loading files.
func (m *Model) toggleTreeEntry(idx int) {
	if idx < 0 || idx >= len(m.fileTree) {
		return
	}
	entry := m.fileTree[idx]
	if entry.isDir {
		if entry.expanded {
			m.fileTree = collapseDir(m.fileTree, idx)
		} else {
			m.fileTree = expandDir(m.fileTree, idx)
		}
	} else {
		absPath, err := filepath.Abs(entry.path)
		if err != nil {
			m.statusMsg = fmt.Sprintf("Cannot open: %v", err)
			return
		}
		if i := m.findTabByPath(absPath); i >= 0 {
			m.activeTab = i
		} else {
			t := newFileTab()
			if err := m.loadFileIntoTab(t, entry.path); err != nil {
				m.statusMsg = fmt.Sprintf(
					"Cannot open: %v", err,
				)
				return
			}
			m.tabs = append(m.tabs, t)
			m.activeTab = len(m.tabs) - 1
		}
		m.focusPane = paneEditor
		m.notifySelectionChanged()
	}
}

// NewModel creates a new TUI Model.
func NewModel(srv MCPServer, rootDir string, watcher *fsnotify.Watcher, dirWatcher *fsnotify.Watcher) (*Model, error) {
	absRootDir, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, fmt.Errorf("resolve root directory: %w", err)
	}

	ft := buildFileTree(absRootDir)

	return &Model{
		server:     srv,
		rootDir:    absRootDir,
		fileTree:   ft,
		treeCursor: 0,
		focusPane:  paneTree,
		watcher:    watcher,
		dirWatcher: dirWatcher,
		tabs:       []*tab{},
		treeWidth:  30,
		keys:       newKeyMap(),
		help:       help.New(),
		iconMode:   detectIconMode(),
	}, nil
}
