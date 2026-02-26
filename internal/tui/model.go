package tui

import (
	"context"
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
	ctx    context.Context
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
}

// activeTabState returns the active tab.
func (m *Model) activeTabState() *tab {
	return m.tabs[m.activeTab]
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
		if err := m.loadFile(entry.path); err != nil {
			m.err = err
		} else {
			m.focusPane = paneEditor
			m.notifySelectionChanged()
		}
	}
}

// NewModel creates a new TUI Model.
func NewModel(srv MCPServer, ctx context.Context, rootDir string, watcher *fsnotify.Watcher, dirWatcher *fsnotify.Watcher) *Model {
	absRootDir, err := filepath.Abs(rootDir)
	if err != nil {
		return &Model{server: srv, ctx: ctx, err: err}
	}

	ft := buildFileTree(absRootDir)

	return &Model{
		server:     srv,
		ctx:        ctx,
		rootDir:    absRootDir,
		fileTree:   ft,
		treeCursor: 0,
		focusPane:  paneTree,
		watcher:    watcher,
		dirWatcher: dirWatcher,
		tabs:       []*tab{newFileTab()},
		treeWidth:  30,
		keys:       newKeyMap(),
		help:       help.New(),
	}
}
