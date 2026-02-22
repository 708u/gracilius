package tui

import (
	"context"
	"path/filepath"

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

// FilePreviewMsg notifies the TUI of an MCP preview request.
type FilePreviewMsg struct {
	FilePath string
	Lines    []string
}

// ClearPreviewMsg clears the current preview.
type ClearPreviewMsg struct{}

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
	width        int
	height       int
	server       MCPServer
	ctx          context.Context
	filePath     string
	lines        []string
	previewLines []string // nil = no preview
	cursorLine   int
	cursorChar   int
	anchorLine   int // selection start
	anchorChar   int
	selecting    bool
	err          error
	watcher      *fsnotify.Watcher

	// file tree
	fileTree   []fileEntry
	treeCursor int
	focusPane  int // 0: tree, 1: editor
	rootDir    string

	// comments
	comments     map[int]string
	commentInput string
	inputMode    bool
	inputLine    int

	// mouse
	lastMouseLine int
	lastMouseChar int
	resizingPane  bool
	treeWidth     int

	// scroll
	scrollOffset int

	// directory watcher
	dirWatcher *fsnotify.Watcher
}

// NewModel creates a new TUI Model.
func NewModel(srv MCPServer, ctx context.Context, rootDir string, watcher *fsnotify.Watcher, dirWatcher *fsnotify.Watcher) Model {
	absRootDir, err := filepath.Abs(rootDir)
	if err != nil {
		return Model{server: srv, ctx: ctx, err: err}
	}

	ft := buildFileTree(absRootDir)

	return Model{
		server:     srv,
		ctx:        ctx,
		rootDir:    absRootDir,
		fileTree:   ft,
		treeCursor: 0,
		focusPane:  0,
		watcher:    watcher,
		dirWatcher: dirWatcher,
		comments:   make(map[int]string),
		treeWidth:  30,
	}
}
