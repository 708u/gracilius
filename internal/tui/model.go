package tui

import (
	"context"
	"path/filepath"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/fsnotify/fsnotify"
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
	width            int
	height           int
	server           MCPServer
	ctx              context.Context
	filePath         string
	lines            []string
	highlightedLines []highlightedLine // nil = no highlighting
	cursorLine       int
	cursorChar       int
	anchorLine       int // selection start
	anchorChar       int
	selecting        bool
	err              error
	watcher          *fsnotify.Watcher

	// file tree
	fileTree   []fileEntry
	treeCursor int
	focusPane  int // 0: tree, 1: editor
	rootDir    string

	// comments
	comments     map[int]string
	commentInput textinput.Model
	inputMode    bool
	inputLine    int

	// mouse
	lastMouseLine int
	lastMouseChar int
	resizingPane  bool
	treeWidth     int

	// scroll
	scrollOffset     int
	treeScrollOffset int

	// directory watcher
	dirWatcher *fsnotify.Watcher

	// keybindings
	keys keyMap
	help help.Model
}

// NewModel creates a new TUI Model.
func NewModel(srv MCPServer, ctx context.Context, rootDir string, watcher *fsnotify.Watcher, dirWatcher *fsnotify.Watcher) Model {
	absRootDir, err := filepath.Abs(rootDir)
	if err != nil {
		return Model{server: srv, ctx: ctx, err: err}
	}

	ft := buildFileTree(absRootDir)

	ti := textinput.New()
	ti.Placeholder = "Enter comment..."
	ti.CharLimit = 500

	return Model{
		server:       srv,
		ctx:          ctx,
		rootDir:      absRootDir,
		fileTree:     ft,
		treeCursor:   0,
		focusPane:    0,
		watcher:      watcher,
		dirWatcher:   dirWatcher,
		comments:     make(map[int]string),
		commentInput: ti,
		treeWidth:    30,
		keys:         newKeyMap(),
		help:         help.New(),
	}
}
