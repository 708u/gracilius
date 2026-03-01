package tui

import (
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// fileItem implements list.Item and list.DefaultItem for file search.
type fileItem struct {
	path string // rootDir-relative path
}

func (f fileItem) Title() string       { return f.path }
func (f fileItem) Description() string { return "" }
func (f fileItem) FilterValue() string { return f.path }

// searchOverlay manages the file search overlay state.
type searchOverlay struct {
	active bool
	list   list.Model
}

func newSearchOverlay() searchOverlay {
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = false
	delegate.SetHeight(1)
	delegate.SetSpacing(0)

	l := list.New(nil, delegate, 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetShowPagination(false)
	l.SetFilteringEnabled(true)
	l.SetShowFilter(false)
	l.DisableQuitKeybindings()
	l.FilterInput.Placeholder = "Search files..."
	l.FilterInput.PromptStyle = lipgloss.NewStyle()
	l.FilterInput.Prompt = ""

	return searchOverlay{list: l}
}

// scanAllFiles recursively scans rootDir using scanDir (from filetree.go)
// and returns all non-hidden files as list.Item values with rootDir-relative paths.
func scanAllFiles(rootDir string) []list.Item {
	var entries []fileEntry
	entries = scanDir(rootDir, 0, entries)
	var items []list.Item
	collectFiles(rootDir, entries, &items)
	return items
}

// collectFiles recursively collects file items from entries,
// expanding directories to get all nested files.
func collectFiles(rootDir string, entries []fileEntry, items *[]list.Item) {
	for _, e := range entries {
		if e.isDir {
			var children []fileEntry
			children = scanDir(e.path, 0, children)
			collectFiles(rootDir, children, items)
		} else {
			rel, err := filepath.Rel(rootDir, e.path)
			if err != nil {
				continue
			}
			*items = append(*items, fileItem{path: rel})
		}
	}
}

// open activates the search overlay and populates it with files.
// open activates the search overlay and populates it with files.
// It triggers the list's built-in filter mode via a synthetic "/"
// keypress so the user can start typing immediately.
func (s *searchOverlay) open(rootDir string) tea.Cmd {
	items := scanAllFiles(rootDir)
	s.list.SetItems(items)
	s.list.ResetFilter()
	// Trigger the list's own filter activation (handles internal
	// state setup that SetFilterState alone does not).
	s.list, _ = s.list.Update(tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune{'/'},
	})
	s.active = true
	// Return the blink cmd so the cursor is visible immediately.
	return s.list.FilterInput.Cursor.BlinkCmd()
}

// close deactivates the search overlay and frees the item list.
func (s *searchOverlay) close() {
	s.active = false
	s.list.SetItems(nil)
}

// update delegates a message to the embedded list.Model.
func (s *searchOverlay) update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	s.list, cmd = s.list.Update(msg)
	return cmd
}

// selectedPath returns the relative path of the currently selected item,
// or empty string if nothing is selected.
func (s *searchOverlay) selectedPath() string {
	item := s.list.SelectedItem()
	if item == nil {
		return ""
	}
	if fi, ok := item.(fileItem); ok {
		return fi.path
	}
	return ""
}

// overlay renders the search overlay on top of the background view.
func (s *searchOverlay) overlay(bg string, width, height int) string {
	overlayW := min(width*3/4, 80)
	overlayH := min(height*3/4, 20)

	innerW := overlayW - 4 // border + padding
	innerH := overlayH - 2 // border

	s.list.SetSize(innerW, innerH)

	filterLine := s.list.FilterInput.View()
	content := filterLine + "\n" + s.list.View()

	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(activeTheme.tabActiveBorder)).
		Padding(0, 1)

	box := borderStyle.
		Width(innerW).
		Render(content)

	return placeOverlay(width, height, box, bg)
}

// placeOverlay composites fg on top of bg, centered.
func placeOverlay(width, height int, fg, bg string) string {
	fgLines := strings.Split(fg, "\n")
	bgLines := strings.Split(bg, "\n")

	// Pad background to fill height
	for len(bgLines) < height {
		bgLines = append(bgLines, "")
	}

	fgH := len(fgLines)
	fgW := 0
	for _, l := range fgLines {
		if w := ansi.StringWidth(l); w > fgW {
			fgW = w
		}
	}

	startY := (height - fgH) / 2
	startX := (width - fgW) / 2
	if startY < 0 {
		startY = 0
	}
	if startX < 0 {
		startX = 0
	}

	result := make([]string, len(bgLines))
	for i, bgLine := range bgLines {
		if i >= startY && i < startY+fgH {
			fgIdx := i - startY
			result[i] = composeLine(bgLine, fgLines[fgIdx], startX)
		} else {
			result[i] = bgLine
		}
	}

	return strings.Join(result, "\n")
}

// composeLine overlays fgLine onto bgLine at the given x offset.
func composeLine(bgLine, fgLine string, startX int) string {
	bgW := ansi.StringWidth(bgLine)
	fgW := ansi.StringWidth(fgLine)

	// Ensure background is wide enough
	if bgW < startX+fgW {
		bgLine = bgLine + strings.Repeat(" ", startX+fgW-bgW)
	}

	before := ansi.Truncate(bgLine, startX, "")
	after := ansi.TruncateLeft(bgLine, startX+fgW, "")

	return before + fgLine + after
}
