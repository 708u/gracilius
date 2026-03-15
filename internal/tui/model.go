package tui

import (
	"fmt"
	"path/filepath"

	"charm.land/bubbles/v2/help"
	"github.com/708u/gracilius/internal/comment"
	"github.com/708u/gracilius/internal/tui/render"
	"github.com/fsnotify/fsnotify"
)

// pane identifies which pane has focus.
type pane int

const (
	paneTree pane = iota
	paneEditor
)

// panel identifies which panel is shown in the left pane.
type panel int

const (
	panelFiles panel = iota
	panelGitDiff
	panelPR
	panelCount // cycling sentinel
)

// label returns the display name for the panel.
func (p panel) label() string {
	switch p {
	case panelFiles:
		return "Files"
	case panelGitDiff:
		return "Git Changes"
	case panelPR:
		return "PR Changes"
	default:
		return "Files"
	}
}

// MCPServer is the interface that the TUI uses to communicate with
// the WebSocket server. server.Server satisfies this implicitly.
type MCPServer interface {
	Port() int
	NotifySelectionChanged(
		filePath, text string,
		startLine, startChar, endLine, endChar int,
	)
	ResendSelection()
}

// CommentRepository is the interface for comment persistence.
// comment.Repository satisfies this implicitly.
type CommentRepository interface {
	List(filePath string, includeResolved bool) ([]comment.Entry, error)
	Add(c comment.Entry) error
	Replace(oldID string, c comment.Entry) error
	Delete(id string) error
	DeleteByFile(filePath string) error
	DataPath() string
}

// OpenDiffMsg notifies the TUI to open a diff tab.
type OpenDiffMsg struct {
	FilePath string
	Contents string
	Accept   func(string) // called with new file contents on accept
	Reject   func()       // called on reject
}

// CloseDiffMsg notifies the TUI to close diff tab(s).
type CloseDiffMsg struct{}

// IdeConnectedMsg notifies the TUI that Claude Code has connected.
type IdeConnectedMsg struct{}

// fileChangedMsg notifies the TUI that the watched file has changed.
type fileChangedMsg struct {
	path  string
	lines []string
}

// treeChangedMsg notifies the TUI that the directory tree has changed.
type treeChangedMsg struct{}

// commentsChangedMsg notifies the TUI that comments.json has changed on disk.
type commentsChangedMsg struct{}

// gitDirChangedMsg notifies the TUI that a file in .git/ has changed
// (e.g. index, HEAD).
type gitDirChangedMsg struct {
	headChanged bool // true if HEAD changed (invalidates merge-base)
}

// gitSyncMsg fires after debounce delay to trigger git reload.
type gitSyncMsg struct {
	gen int
}

// gitChangedFilesMsg carries the result of loading git changed files.
type gitChangedFilesMsg struct {
	mode    gitDiffMode
	entries []changedFileEntry
	err     error
}

// gitBranchInfoMsg carries the result of async branch info resolution.
type gitBranchInfoMsg struct {
	mergeBase     string
	defaultBranch string
	err           string
}

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

	// panel system
	activePanel    panel // which panel is shown in the left pane
	sidebarVisible bool  // whether the left pane is visible

	// mouse
	lastMouseLine int
	lastMouseChar int
	mouseDown     bool
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

	// clear-all confirmation
	clearAllPending bool

	// gg sequence
	gPending bool

	// status message (temporary, auto-cleared)
	statusMsg string

	// icon display mode
	iconMode iconMode

	// visual row mapping (rebuilt each render)
	lastMapping []visualEntry

	// open-file overlay
	openFile openFileOverlay

	// theme
	isDark bool
	theme  render.Theme

	// keyboard enhancement (Kitty protocol)
	enhancedKeyboard bool

	// comment persistence
	commentRepo    CommentRepository
	commentWatcher *fsnotify.Watcher

	// git directory watcher (.git/index, .git/HEAD)
	gitDirWatcher *fsnotify.Watcher

	// git panel state (per-mode)
	gitDiffMode      gitDiffMode
	gitModeState     []gitPanelState
	gitAnyLoaded     bool // true once any mode has been loaded
	gitSyncGen       int  // generation counter for debounced git sync
	gitMergeBase     string
	gitDefaultBranch string

	// file exclusion (gitignore-based when available)
	excludeFunc ExcludeFunc

	// in-file search
	search searchState
}

// lineKind distinguishes the type of a visual row.
type lineKind int

const (
	lineKindCode lineKind = iota
	lineKindComment
	lineKindInput
)

// visualEntry maps a visual row to its logical line and type.
type visualEntry struct {
	logicalLine int
	kind        lineKind
	wrapOffset  int // rune offset in the logical line where this wrap segment starts
}

// activeTabState returns the active tab and whether it exists.
func (m *Model) activeTabState() (*tab, bool) {
	if len(m.tabs) == 0 {
		return nil, false
	}
	return m.tabs[m.activeTab], true
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

// openFileByPath opens a file by absolute path in a tab.
// If a tab with the same path already exists, it switches to it.
func (m *Model) openFileByPath(absPath string) {
	if i := m.findTabByPath(absPath); i >= 0 {
		m.activeTab = i
	} else {
		var target *tab
		cur, hasTab := m.activeTabState()
		if hasTab && cur.kind == fileTab && cur.filePath == "" {
			target = cur
		} else {
			target = newFileTab()
		}
		if err := m.loadFileIntoTab(target, absPath); err != nil {
			m.statusMsg = fmt.Sprintf("Cannot open: %v", err)
			return
		}
		if target != cur {
			m.tabs = append(m.tabs, target)
			m.activeTab = len(m.tabs) - 1
		}
	}
	m.focusPane = paneEditor
	m.notifySelectionChanged()
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
		m.openFileByPath(absPath)
	}
}

// gitState returns a pointer to the panel state for the active diff mode.
func (m *Model) gitState() *gitPanelState {
	return &m.gitModeState[m.gitDiffMode]
}

// NewModel creates a new TUI Model.
func NewModel(srv MCPServer, store CommentRepository, rootDir string, watcher *fsnotify.Watcher, dirWatcher *fsnotify.Watcher, commentWatcher *fsnotify.Watcher, gitDirWatcher *fsnotify.Watcher, exclude ExcludeFunc) (*Model, error) {
	absRootDir, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, fmt.Errorf("resolve root directory: %w", err)
	}

	ft := buildFileTree(absRootDir)

	im := detectIconMode()
	return &Model{
		server:         srv,
		rootDir:        absRootDir,
		fileTree:       ft,
		treeCursor:     0,
		focusPane:      paneTree,
		watcher:        watcher,
		dirWatcher:     dirWatcher,
		tabs:           []*tab{},
		treeWidth:      30,
		activePanel:    panelFiles,
		sidebarVisible: true,
		keys:           newKeyMap(),
		help:           help.New(),
		iconMode:       im,
		openFile:       newOpenFileOverlay(im, render.Dark),
		isDark:         true,
		theme:          render.Dark,
		commentRepo:    store,
		commentWatcher: commentWatcher,
		gitDirWatcher:  gitDirWatcher,
		gitDiffMode:    gitModeWorking,
		gitModeState:   make([]gitPanelState, len(gitDiffModes)),
		excludeFunc:    exclude,
		search:         newSearchState(),
	}, nil
}
