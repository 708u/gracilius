package tui

import (
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/sahilm/fuzzy"
)

// fileItem implements list.Item for file search.
type fileItem struct {
	path         string // rootDir-relative path
	resolvedPath string // symlink target path (empty if not a symlink)
	matchedRunes []int  // fuzzy match positions (nil when no filter active)
}

func (f fileItem) Title() string       { return f.path }
func (f fileItem) Description() string { return "" }
func (f fileItem) FilterValue() string { return f.path }

// searchDelegate renders file items with an icon prefix.
type searchDelegate struct {
	iconMode   iconMode
	matchStyle lipgloss.Style // bold + match fg color
	selBgStyle lipgloss.Style // selection background
}

func (d *searchDelegate) Height() int                         { return 1 }
func (d *searchDelegate) Spacing() int                        { return 0 }
func (d *searchDelegate) Update(tea.Msg, *list.Model) tea.Cmd { return nil }

func (d *searchDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	fi, ok := item.(fileItem)
	if !ok {
		return
	}

	entry := fileEntry{
		path:         fi.path,
		name:         filepath.Base(fi.path),
		resolvedPath: fi.resolvedPath,
	}
	icon := iconFor(d.iconMode, entry)
	selected := index == m.Index()

	pathStr := fi.path

	if len(fi.matchedRunes) > 0 {
		ms := d.matchStyle
		us := lipgloss.Style{}
		if selected {
			ms = ms.Background(lipgloss.Color(activeTheme.searchSelectionBg))
			us = d.selBgStyle
		}
		pathStr = lipgloss.StyleRunes(pathStr, fi.matchedRunes, ms, us)
	} else if selected {
		pathStr = d.selBgStyle.Render(pathStr)
	}

	line := icon.prefix() + pathStr

	if selected {
		if lineW := ansi.StringWidth(line); lineW < m.Width() {
			line += d.selBgStyle.Render(strings.Repeat(" ", m.Width()-lineW))
		}
	}

	line = icon.colorize(line)
	fmt.Fprint(w, line)
}

// searchOverlay manages the file search overlay state.
type searchOverlay struct {
	active   bool
	input    textinput.Model
	list     list.Model
	allItems []fileItem // all scanned files (unfiltered)
	targets  []string   // cached paths for fuzzy matching
}

func newSearchOverlay(mode iconMode) searchOverlay {
	delegate := &searchDelegate{
		iconMode: mode,
		matchStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color(activeTheme.searchMatchFg)).
			Bold(true),
		selBgStyle: lipgloss.NewStyle().
			Background(lipgloss.Color(activeTheme.searchSelectionBg)),
	}

	l := list.New(nil, delegate, 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetShowPagination(false)
	l.SetFilteringEnabled(false)
	l.DisableQuitKeybindings()

	ti := textinput.New()
	ti.Placeholder = "Search files..."
	ti.Prompt = ""
	ti.PromptStyle = lipgloss.NewStyle()

	return searchOverlay{list: l, input: ti}
}

// scanAllFiles recursively scans rootDir using scanDir (from filetree.go)
// and returns all non-hidden files as fileItem values with rootDir-relative paths.
func scanAllFiles(rootDir string) []fileItem {
	var entries []fileEntry
	entries = scanDir(rootDir, 0, entries)
	var items []fileItem
	collectFiles(rootDir, entries, &items)
	return items
}

// collectFiles recursively collects file items from entries,
// expanding directories to get all nested files.
func collectFiles(rootDir string, entries []fileEntry, items *[]fileItem) {
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
			*items = append(*items, fileItem{
				path:         rel,
				resolvedPath: e.resolvedPath,
			})
		}
	}
}

// open activates the search overlay and populates it with files.
func (s *searchOverlay) open(rootDir string) tea.Cmd {
	s.allItems = scanAllFiles(rootDir)
	s.targets = make([]string, len(s.allItems))
	for i, fi := range s.allItems {
		s.targets[i] = fi.path
	}
	s.input.Reset()
	s.input.Focus()
	s.applyFilter()
	s.active = true
	return s.input.Cursor.BlinkCmd()
}

// close deactivates the search overlay and frees the item list.
func (s *searchOverlay) close() {
	s.active = false
	s.allItems = nil
	s.targets = nil
	s.list.SetItems(nil)
}

// applyFilter filters allItems by the current input value and updates the list.
func (s *searchOverlay) applyFilter() {
	term := s.input.Value()

	if term == "" {
		items := make([]list.Item, len(s.allItems))
		for i := range s.allItems {
			items[i] = s.allItems[i]
		}
		s.list.SetItems(items)
		return
	}

	ranks := fuzzy.Find(term, s.targets)
	sort.Stable(ranks)

	items := make([]list.Item, len(ranks))
	for i, r := range ranks {
		fi := s.allItems[r.Index]
		fi.matchedRunes = r.MatchedIndexes
		items[i] = fi
	}
	s.list.SetItems(items)
}

// update handles messages for the search overlay.
// Printable input goes to textinput; navigation goes to list.
func (s *searchOverlay) update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyUp, tea.KeyDown,
			tea.KeyCtrlN, tea.KeyCtrlP:
			var cmd tea.Cmd
			s.list, cmd = s.list.Update(msg)
			return cmd
		default:
			prevValue := s.input.Value()
			var cmd tea.Cmd
			s.input, cmd = s.input.Update(msg)
			if s.input.Value() != prevValue {
				s.applyFilter()
			}
			return cmd
		}
	default:
		// Route non-key messages (e.g. cursor blink) to textinput.
		var cmd tea.Cmd
		s.input, cmd = s.input.Update(msg)
		return cmd
	}
}

// handleClick processes a mouse click within the search overlay.
// It returns the relative path of the clicked item (empty if none)
// and whether the overlay should be closed.
func (s *searchOverlay) handleClick(mouseX, mouseY, width, height int) (path string, closeOverlay bool) {
	overlayW := min(width*3/4, 80)

	oStartY := headerHeight + tabBarHeight
	maxInnerH := max(height-oStartY-footerHeight-2, 3)
	itemCount := len(s.list.Items())
	innerH := min(2+max(itemCount, 1), maxInnerH)

	boxH := innerH + 2 // content + top/bottom border
	maxBottom := height - footerHeight
	if oStartY+boxH > maxBottom {
		oStartY = max(maxBottom-boxH, 0)
	}

	fgW := overlayW
	startX := max((width-fgW)/2, 0)

	// Click outside overlay: close
	if mouseY < oStartY || mouseY >= oStartY+boxH ||
		mouseX < startX || mouseX >= startX+overlayW {
		return "", true
	}

	// List area: border(1) + input(1) + separator(1) = 3 rows offset
	listStartY := oStartY + 3
	listH := max(innerH-2, 1)

	if mouseY >= listStartY && mouseY < listStartY+listH {
		visualRow := mouseY - listStartY
		totalItems := len(s.list.Items())
		start, _ := s.list.Paginator.GetSliceBounds(totalItems)
		absIdx := start + visualRow
		if absIdx < totalItems {
			s.list.Select(absIdx)
			return s.selectedPath(), false
		}
	}

	return "", false
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
	innerW := overlayW - 4 // border(2) + padding(2)

	// Max height: available space between startY and footer, minus border
	startY := headerHeight + tabBarHeight
	maxInnerH := max(height-startY-footerHeight-2, 3) // border(2)
	itemCount := len(s.list.Items())
	innerH := min(2+max(itemCount, 1), maxInnerH) // input(1) + separator(1) + items
	listH := max(innerH-2, 1)

	s.list.SetSize(innerW, listH)
	s.input.Width = innerW

	inputView := s.input.View()
	separator := strings.Repeat("─", innerW)
	content := inputView + "\n" + separator + "\n" + s.list.View()

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(activeTheme.tabActiveBorder)).
		Padding(0, 1).
		Height(innerH).
		Render(content)

	dimmedBg := dimBackground(bg)
	return placeOverlay(width, height, box, dimmedBg)
}

// dimBackground wraps each line with ANSI faint to dim the background.
func dimBackground(bg string) string {
	lines := strings.Split(bg, "\n")
	for i, line := range lines {
		lines[i] = ansiFaint + line + ansiReset
	}
	return strings.Join(lines, "\n")
}

// placeOverlay composites fg on top of bg, horizontally centered
// and positioned at the upper 1/5 of the screen.
func placeOverlay(width, height int, fg, bg string) string {
	fgLines := strings.Split(fg, "\n")
	bgLines := strings.Split(bg, "\n")

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

	maxBottom := height - footerHeight
	startY := headerHeight + tabBarHeight
	if startY+fgH > maxBottom {
		startY = max(maxBottom-fgH, 0)
	}
	startX := max((width-fgW)/2, 0)

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

	if bgW < startX+fgW {
		bgLine = bgLine + strings.Repeat(" ", startX+fgW-bgW)
	}

	before := ansi.Truncate(bgLine, startX, "")
	after := ansi.TruncateLeft(bgLine, startX+fgW, "")

	return before + fgLine + after
}
