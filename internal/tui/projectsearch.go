package tui

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/708u/gracilius/internal/fileutil"
	"github.com/708u/gracilius/internal/tui/render"
	"github.com/charmbracelet/x/ansi"
)

const (
	projectSearchMaxFileSize = 1 << 20 // 1 MB
	projectSearchMaxMatches  = 1000
)

// searchResultItem implements list.Item for the project search overlay.
type searchResultItem struct {
	relPath   string // rootDir-relative path
	absPath   string
	line      int    // 0-based line number
	text      string // full line text
	startChar int    // match start (rune offset)
	endChar   int    // match end (rune offset)
}

func (s searchResultItem) Title() string       { return s.relPath }
func (s searchResultItem) Description() string { return "" }
func (s searchResultItem) FilterValue() string { return s.relPath }

// searchResultDelegate renders search result items (2 lines each).
type searchResultDelegate struct {
	matchStyle lipgloss.Style // bold + match fg color
	selBgStyle lipgloss.Style // selection background
	dimStyle   lipgloss.Style // dim for file path line
}

func (d *searchResultDelegate) Height() int                         { return 2 }
func (d *searchResultDelegate) Spacing() int                        { return 0 }
func (d *searchResultDelegate) Update(tea.Msg, *list.Model) tea.Cmd { return nil }

func (d *searchResultDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	ri, ok := item.(searchResultItem)
	if !ok {
		return
	}
	selected := index == m.Index()
	maxW := m.Width()

	// Line 1: file path:lineNum (dim)
	// Truncate plain text first, then apply styles to ensure consistent width.
	pathLine := ansi.Truncate(fmt.Sprintf("%s:%d", ri.relPath, ri.line+1), maxW, "…")
	if selected {
		pathLine = d.selBgStyle.Render(d.dimStyle.Render(pathLine))
		if lineW := ansi.StringWidth(pathLine); lineW < maxW {
			pathLine += d.selBgStyle.Render(strings.Repeat(" ", maxW-lineW))
		}
	} else {
		pathLine = d.dimStyle.Render(pathLine)
	}

	// Line 2: match line text with highlight
	// Truncate plain text first to fix layout shift between selected/unselected rows.
	textLine := ansi.Truncate(ri.text, maxW, "…")
	if ri.startChar < ri.endChar {
		runes := []rune(textLine)
		sc := min(ri.startChar, len(runes))
		ec := min(ri.endChar, len(runes))
		before := string(runes[:sc])
		match := string(runes[sc:ec])
		after := string(runes[ec:])

		ms := d.matchStyle
		if selected {
			ms = ms.Background(d.selBgStyle.GetBackground())
			textLine = d.selBgStyle.Render(before) + ms.Render(match) + d.selBgStyle.Render(after)
		} else {
			textLine = before + ms.Render(match) + after
		}
	} else if selected {
		textLine = d.selBgStyle.Render(textLine)
	}
	if selected {
		if lineW := ansi.StringWidth(textLine); lineW < maxW {
			textLine += d.selBgStyle.Render(strings.Repeat(" ", maxW-lineW))
		}
	}

	fmt.Fprint(w, pathLine+"\n"+textLine)
}

// projectSearchResultMsg carries async search results.
type projectSearchResultMsg struct {
	gen       int
	items     []searchResultItem
	total     int
	truncated bool
}

// projectSearchOverlay manages the project-wide search overlay state.
type projectSearchOverlay struct {
	active    bool
	input     textinput.Model
	list      list.Model
	query     string // confirmed query
	searching bool   // async search in progress
	gen       int    // generation counter
	total     int    // total match count
	truncated bool   // whether results were truncated
	theme     render.Theme
}

func newProjectSearchOverlay(theme render.Theme) projectSearchOverlay {
	delegate := &searchResultDelegate{
		matchStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color(theme.OpenFileMatchFg)).
			Bold(true),
		selBgStyle: lipgloss.NewStyle().
			Background(lipgloss.Color(theme.OpenFileSelectionBg)),
		dimStyle: lipgloss.NewStyle().Faint(true),
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
	ti.Placeholder = "Find in project..."
	ti.Prompt = "⌕ "
	ti.SetVirtualCursor(false)

	return projectSearchOverlay{list: l, input: ti, theme: theme}
}

// updateTheme updates the overlay's theme and rebuilds the delegate styles.
func (s *projectSearchOverlay) updateTheme(theme render.Theme) {
	s.theme = theme
	d := &searchResultDelegate{
		matchStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color(theme.OpenFileMatchFg)).
			Bold(true),
		selBgStyle: lipgloss.NewStyle().
			Background(lipgloss.Color(theme.OpenFileSelectionBg)),
		dimStyle: lipgloss.NewStyle().Faint(true),
	}
	s.list.SetDelegate(d)
}

// open activates the project search overlay.
func (s *projectSearchOverlay) open() tea.Cmd {
	s.active = true
	s.query = ""
	s.input.Reset()
	return s.input.Focus()
}

// close deactivates the project search overlay.
func (s *projectSearchOverlay) close() {
	s.active = false
	s.searching = false
	s.list.SetItems(nil)
}

// startSearch initiates an async search.
func (s *projectSearchOverlay) startSearch(rootDir string, exclude ExcludeFunc) tea.Cmd {
	q := s.input.Value()
	if q == "" {
		return nil
	}
	s.query = q
	s.searching = true
	s.gen++
	gen := s.gen
	return func() tea.Msg {
		items, total, truncated := projectSearchExec(rootDir, q, exclude)
		return projectSearchResultMsg{
			gen:       gen,
			items:     items,
			total:     total,
			truncated: truncated,
		}
	}
}

// applyResults updates the list with search results.
func (s *projectSearchOverlay) applyResults(msg projectSearchResultMsg) {
	if msg.gen != s.gen {
		return
	}
	s.searching = false
	s.total = msg.total
	s.truncated = msg.truncated
	items := make([]list.Item, len(msg.items))
	for i := range msg.items {
		items[i] = msg.items[i]
	}
	s.list.SetItems(items)
}

// selectedResult returns the absPath and line of the selected item.
func (s *projectSearchOverlay) selectedResult() (string, int) {
	item := s.list.SelectedItem()
	if item == nil {
		return "", 0
	}
	if ri, ok := item.(searchResultItem); ok {
		return ri.absPath, ri.line
	}
	return "", 0
}

// update handles messages for the project search overlay.
func (s *projectSearchOverlay) update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.Code {
		case tea.KeyUp, tea.KeyDown:
			var cmd tea.Cmd
			s.list, cmd = s.list.Update(msg)
			return cmd
		default:
			if !s.input.Focused() {
				s.input.Focus()
			}
			var cmd tea.Cmd
			s.input, cmd = s.input.Update(msg)
			return cmd
		}
	default:
		var cmd tea.Cmd
		s.input, cmd = s.input.Update(msg)
		return cmd
	}
}

// computeLayout calculates the overlay layout from terminal dimensions.
func (s *projectSearchOverlay) computeLayout(width, height int) openFileLayout {
	overlayW := min(width*overlayWidthRatio/4, overlayMaxW)
	innerW := overlayW - overlayBorderW - overlayPaddingW

	startY := contentStartY
	available := height - startY - footerHeight - overlayBorderH
	maxInnerH := max(available, overlayMinInnerH)

	// Status line (e.g. "12 results in 4 files") takes 2 rows (text + separator).
	statusRows := 2
	itemCount := len(s.list.Items())
	innerH := min(overlayInputRows+statusRows+max(itemCount*2, overlayMinItems), maxInnerH)
	listH := max(innerH-overlayInputRows-statusRows, overlayMinItems)

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

// cursorPos returns the screen-space cursor position for the overlay's text input.
// It replicates placeOverlay's startY calculation using the actual rendered box
// height, which may differ from computeLayout's boxH due to lipgloss expansion.
func (s *projectSearchOverlay) cursorPos(width, height int) cursorPosition {
	if !s.input.Focused() {
		return cursorPosition{}
	}
	g := s.computeLayout(width, height)

	// Match placeOverlay's startY calculation using actual box height.
	fgH := g.innerH + overlayBorderH
	maxBottom := height - footerHeight
	startY := headerHeight + tabBarHeight
	if startY+fgH > maxBottom {
		startY = max(maxBottom-fgH, 0)
	}

	val := s.input.Value()
	pos := s.input.Position()
	cursorCol := render.DisplayWidthRange(val, 0, pos)
	promptW := ansi.StringWidth(s.input.Prompt)
	return cursorPosition{
		x: g.startX + overlayBorderW/2 + overlayPaddingW/2 + promptW + cursorCol,
		y: startY + overlayBorderH/2,
	}
}

// handleClick processes a mouse click within the project search overlay.
func (s *projectSearchOverlay) handleClick(mouseX, mouseY, width, height int) (absPath string, line int, closeOverlay bool) {
	g := s.computeLayout(width, height)

	// Click outside overlay: close
	if mouseY < g.startY || mouseY >= g.startY+g.boxH ||
		mouseX < g.startX || mouseX >= g.startX+g.overlayW {
		return "", 0, true
	}

	// Status line takes 2 rows after input.
	listStartY := g.startY + overlayListOffset + 2 // +2 for status line + separator

	if mouseY >= listStartY && mouseY < listStartY+g.listH {
		visualRow := (mouseY - listStartY) / 2 // each item is 2 rows
		totalItems := len(s.list.Items())
		start, _ := s.list.Paginator.GetSliceBounds(totalItems)
		absIdx := start + visualRow
		if absIdx < totalItems {
			s.list.Select(absIdx)
			p, l := s.selectedResult()
			return p, l, false
		}
	}

	return "", 0, false
}

// overlay renders the project search overlay on top of the background view.
func (s *projectSearchOverlay) overlay(bg string, width, height int) string {
	g := s.computeLayout(width, height)

	s.list.SetSize(g.innerW, g.listH)
	s.input.SetWidth(g.innerW)

	inputView := s.input.View()

	// Status line
	var statusLine string
	switch {
	case s.searching:
		statusLine = "Searching..."
	case s.query != "" && s.total == 0:
		statusLine = "No results"
	case s.query != "":
		statusLine = fmt.Sprintf("%d results", s.total)
		if s.truncated {
			statusLine += fmt.Sprintf(" (showing first %d)", len(s.list.Items()))
		}
	}
	statusLine = ansi.Truncate(statusLine, g.innerW, "…")

	separator := strings.Repeat("─", g.innerW)
	content := inputView + "\n" + separator + "\n" +
		statusLine + "\n" + separator + "\n" + s.list.View()

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(s.theme.TabActiveBorder)).
		Padding(0, 1).
		Width(g.innerW + overlayBorderW + overlayPaddingW).
		Height(g.innerH).
		Render(content)

	dimmedBg := dimBackground(bg)
	return placeOverlay(width, height, box, dimmedBg)
}

// projectSearchExec performs the actual file search (runs in a goroutine).
func projectSearchExec(rootDir, query string, exclude ExcludeFunc) ([]searchResultItem, int, bool) {
	caseSensitive := isSmartCaseSensitive(query)
	queryRunes := normalizeForSearch(query, caseSensitive)

	allFiles := scanAllFiles(rootDir, exclude)

	var items []searchResultItem
	total := 0

	for _, fi := range allFiles {
		if len(items) >= projectSearchMaxMatches {
			break
		}

		absPath := filepath.Join(rootDir, fi.path)

		content, err := os.ReadFile(absPath)
		if err != nil {
			continue
		}
		if len(content) > projectSearchMaxFileSize {
			continue
		}
		if fileutil.IsBinary(content) {
			continue
		}

		lines := fileutil.SplitLines(content)
		for i, line := range lines {
			lineRunes := normalizeForSearch(line, caseSensitive)
			for _, pos := range findSubstringPositions(lineRunes, queryRunes) {
				total++
				if len(items) < projectSearchMaxMatches {
					items = append(items, searchResultItem{
						relPath:   fi.path,
						absPath:   absPath,
						line:      i,
						text:      strings.TrimRight(line, "\r\n"),
						startChar: pos,
						endChar:   pos + len(queryRunes),
					})
				}
			}
		}
	}

	return items, total, total > len(items)
}
