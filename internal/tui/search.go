package tui

import (
	"path/filepath"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
	l.DisableQuitKeybindings()

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
func (s *searchOverlay) open(rootDir string) {
	items := scanAllFiles(rootDir)
	s.list.SetItems(items)
	s.list.ResetFilter()
	s.list.FilterInput.Focus()
	s.list.SetFilterState(list.Filtering)
	s.active = true
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

// view renders the search overlay centered on the screen.
func (s *searchOverlay) view(width, height int) string {
	overlayW := min(width*3/4, 80)
	overlayH := min(height*3/4, 20)

	innerW := overlayW - 4 // border + padding
	innerH := overlayH - 2 // border

	s.list.SetSize(innerW, innerH)

	title := "Search Files"
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(activeTheme.tabActiveFg)).
		Bold(true)

	content := titleStyle.Render(title) + "\n" + s.list.View()

	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(activeTheme.tabActiveBorder)).
		Padding(0, 1)

	box := borderStyle.
		Width(innerW).
		Render(content)

	return lipgloss.Place(
		width, height,
		lipgloss.Center, lipgloss.Center,
		box,
		lipgloss.WithWhitespaceChars(" "),
	)
}
