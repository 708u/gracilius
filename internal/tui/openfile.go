package tui

import (
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/sahilm/fuzzy"
)

// fileItem implements list.Item for the open-file overlay.
type fileItem struct {
	path         string // rootDir-relative path
	resolvedPath string // symlink target path (empty if not a symlink)
	matchedRunes []int  // fuzzy match positions (nil when no filter active)
}

func (f fileItem) Title() string       { return f.path }
func (f fileItem) Description() string { return "" }
func (f fileItem) FilterValue() string { return f.path }

// openFileDelegate renders file items with an icon prefix.
type openFileDelegate struct {
	iconMode   iconMode
	matchStyle lipgloss.Style // bold + match fg color
	selBgStyle lipgloss.Style // selection background
}

func (d *openFileDelegate) Height() int                         { return 1 }
func (d *openFileDelegate) Spacing() int                        { return 0 }
func (d *openFileDelegate) Update(tea.Msg, *list.Model) tea.Cmd { return nil }

func (d *openFileDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
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
			ms = ms.Background(d.selBgStyle.GetBackground())
			us = d.selBgStyle
		}
		pathStr = lipgloss.StyleRunes(pathStr, fi.matchedRunes, ms, us)
	} else if selected {
		pathStr = d.selBgStyle.Render(pathStr)
	}

	line := icon.prefix() + pathStr
	maxW := m.Width()

	// Truncate to fit within list width
	if ansi.StringWidth(line) > maxW {
		line = ansi.Truncate(line, maxW-1, "…")
	}

	if selected {
		if lineW := ansi.StringWidth(line); lineW < maxW {
			line += d.selBgStyle.Render(strings.Repeat(" ", maxW-lineW))
		}
	}

	line = icon.colorize(line)
	fmt.Fprint(w, line)
}

// openFileOverlay manages the file search overlay state.
type openFileOverlay struct {
	active   bool
	input    textinput.Model
	list     list.Model
	allItems []fileItem // all scanned files (unfiltered)
	targets  []string   // cached paths for fuzzy matching
	iconMode iconMode
	theme    themeConfig
}

func newOpenFileOverlay(mode iconMode, theme themeConfig) openFileOverlay {
	delegate := &openFileDelegate{
		iconMode: mode,
		matchStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color(theme.openFileMatchFg)).
			Bold(true),
		selBgStyle: lipgloss.NewStyle().
			Background(lipgloss.Color(theme.openFileSelectionBg)),
	}

	l := list.New(nil, delegate, 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetShowPagination(false)
	l.SetFilteringEnabled(false)
	l.SetShowFilter(false)
	l.DisableQuitKeybindings()

	ti := textinput.New()
	ti.Placeholder = "Open file..."
	ti.Prompt = "⌕ "
	ti.SetVirtualCursor(false)

	return openFileOverlay{list: l, input: ti, iconMode: mode, theme: theme}
}

// updateTheme updates the overlay's theme and rebuilds the delegate styles.
func (s *openFileOverlay) updateTheme(theme themeConfig) {
	s.theme = theme
	d := &openFileDelegate{
		iconMode: s.iconMode,
		matchStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color(theme.openFileMatchFg)).
			Bold(true),
		selBgStyle: lipgloss.NewStyle().
			Background(lipgloss.Color(theme.openFileSelectionBg)),
	}
	s.list.SetDelegate(d)
}

// scanAllFiles recursively scans rootDir and returns all non-ignored files
// as fileItem values with rootDir-relative paths.
// When exclude is non-nil, gitignore-based filtering is applied;
// otherwise isHiddenEntry is used.
func scanAllFiles(rootDir string, exclude ExcludeFunc) []fileItem {
	var entries []fileEntry
	entries = scanDir(rootDir, 0, entries, exclude)
	var items []fileItem
	collectFiles(rootDir, entries, &items, exclude)
	return items
}

// collectFiles recursively collects file items from entries,
// expanding directories to get all nested files.
func collectFiles(rootDir string, entries []fileEntry, items *[]fileItem, exclude ExcludeFunc) {
	for _, e := range entries {
		if e.isDir {
			var children []fileEntry
			children = scanDir(e.path, 0, children, exclude)
			collectFiles(rootDir, children, items, exclude)
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

// open activates the open-file overlay and populates it with files.
func (s *openFileOverlay) open(rootDir string, exclude ExcludeFunc) tea.Cmd {
	s.allItems = scanAllFiles(rootDir, exclude)
	s.targets = make([]string, len(s.allItems))
	for i, fi := range s.allItems {
		s.targets[i] = fi.path
	}
	s.input.Reset()
	cmd := s.input.Focus()
	s.applyFilter()
	s.active = true
	return cmd
}

// close deactivates the open-file overlay and frees the item list.
func (s *openFileOverlay) close() {
	s.active = false
	s.allItems = nil
	s.targets = nil
	s.list.SetItems(nil)
}

// applyFilter filters allItems by the current input value and updates the list.
func (s *openFileOverlay) applyFilter() {
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

// update handles messages for the open-file overlay.
// Arrow keys navigate the list; all other input goes to textinput.
func (s *openFileOverlay) update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.Code {
		case tea.KeyUp, tea.KeyDown:
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

// openFileLayout holds the computed layout for the search overlay.
type openFileLayout struct {
	overlayW int // total overlay width
	innerW   int // content width (overlayW - border - padding)
	startX   int // horizontal start position
	startY   int // vertical start position
	innerH   int // inner content height (input + separator + list)
	listH    int // list area height
	boxH     int // total box height (innerH + border)
}

const (
	overlayMaxW       = 80 // max overlay width in columns
	overlayWidthRatio = 3  // overlay uses 3/4 of terminal width
	overlayBorderW    = 2  // left + right border
	overlayPaddingW   = 2  // left + right padding
	overlayBorderH    = 2  // top + bottom border
	overlayInputRows  = 2  // input field + separator
	overlayMinItems   = 1  // show at least 1 item row
	overlayMinInnerH  = 3  // minimum inner height
	overlayListOffset = 3  // rows from box top to list: border + input + separator
)

// computeLayout calculates the overlay layout from terminal dimensions.
func (s *openFileOverlay) computeLayout(width, height int) openFileLayout {
	overlayW := min(width*overlayWidthRatio/4, overlayMaxW)
	innerW := overlayW - overlayBorderW - overlayPaddingW

	startY := contentStartY
	available := height - startY - footerHeight - overlayBorderH
	maxInnerH := max(available, overlayMinInnerH)
	itemCount := len(s.list.Items())
	innerH := min(overlayInputRows+max(itemCount, overlayMinItems), maxInnerH)
	listH := max(innerH-overlayInputRows, overlayMinItems)

	boxH := innerH + overlayBorderH
	maxBottom := height - footerHeight
	if startY+boxH > maxBottom {
		startY = max(maxBottom-boxH, 0)
	}

	startX := max((width-overlayW)/2, 0)

	return openFileLayout{
		overlayW: overlayW,
		innerW:   innerW,
		startX:   startX,
		startY:   startY,
		innerH:   innerH,
		listH:    listH,
		boxH:     boxH,
	}
}

// handleClick processes a mouse click within the open-file overlay.
// It returns the relative path of the clicked item (empty if none)
// and whether the overlay should be closed.
func (s *openFileOverlay) handleClick(mouseX, mouseY, width, height int) (path string, closeOverlay bool) {
	g := s.computeLayout(width, height)

	// Click outside overlay: close
	if mouseY < g.startY || mouseY >= g.startY+g.boxH ||
		mouseX < g.startX || mouseX >= g.startX+g.overlayW {
		return "", true
	}

	listStartY := g.startY + overlayListOffset

	if mouseY >= listStartY && mouseY < listStartY+g.listH {
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
func (s *openFileOverlay) selectedPath() string {
	item := s.list.SelectedItem()
	if item == nil {
		return ""
	}
	if fi, ok := item.(fileItem); ok {
		return fi.path
	}
	return ""
}

// cursorPos returns the screen-space cursor position for the overlay's text input.
func (s *openFileOverlay) cursorPos(width, height int) cursorPosition {
	if !s.input.Focused() {
		return cursorPosition{}
	}
	g := s.computeLayout(width, height)
	// Compute display width instead of using Cursor().X, which returns
	// a rune index and is incorrect for wide characters (CJK).
	// See: https://github.com/charmbracelet/bubbles/issues/906
	val := s.input.Value()
	pos := s.input.Position()
	cursorCol := displayWidthRange(val, 0, pos)
	promptW := ansi.StringWidth(s.input.Prompt)
	return cursorPosition{
		x: g.startX + overlayBorderW/2 + overlayPaddingW/2 + promptW + cursorCol,
		y: g.startY + overlayBorderH/2,
	}
}

// overlay renders the open-file overlay on top of the background view.

func (s *openFileOverlay) overlay(bg string, width, height int) string {
	g := s.computeLayout(width, height)

	s.list.SetSize(g.innerW, g.listH)
	s.input.SetWidth(g.innerW)

	inputView := s.input.View()
	separator := strings.Repeat("─", g.innerW)
	content := inputView + "\n" + separator + "\n" + s.list.View()

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(s.theme.tabActiveBorder)).
		Padding(0, 1).
		Width(g.innerW + overlayBorderW + overlayPaddingW).
		Height(g.innerH).
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
		bgLine += strings.Repeat(" ", startX+fgW-bgW)
	}

	before := ansi.Truncate(bgLine, startX, "")
	after := ansi.TruncateLeft(bgLine, startX+fgW, "")

	return before + fgLine + after
}
