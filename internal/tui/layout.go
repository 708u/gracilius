package tui

const (
	headerHeight        = 1
	tabBarHeight        = 2 // labels + underline
	footerHeight        = 4 // border-top + help + status
	separatorWidth      = 3
	minLineNumberWidth  = 4
	minTreeWidth        = 15
	defaultTreePercent  = 30
	maxTreeWidthPercent = 70
	contentStartY       = headerHeight + tabBarHeight
	minContentHeight    = 5
)

// layout holds all derived dimensions for a single render frame.
// Computed fresh on each View() / mouse-handling pass; never stored.
//
// Vertical:
//
//	+-----------------------+ --
//	| header                |  1  headerHeight
//	| tab labels            |  2  tabBarHeight
//	| tab underline         |
//	+-----------------------+ --
//	| tree | sep | editor   |  contentHeight
//	|      |     |          |
//	+-----------------------+ --
//	| footer border         |  4  footerHeight
//	| help keys             |
//	| cursor/status         |
//	+-----------------------+ --
//
// Horizontal:
//
//	|<-treeWidth->|<sep>|<----editorWidth-------->|
//	|             | (3) |<-lnw->|<----text------->|
//	              ^             ^
//	              editorStartX  editorStartX + lineNumberWidth
type layout struct {
	contentHeight int // usable rows for tree and editor panes
	treeWidth     int // file tree pane width
	editorStartX  int // treeWidth + separatorWidth
	editorWidth   int // total width - treeWidth - separatorWidth
	lineNumWidth  int // line number gutter width (digits + 1 space)
}

// getTreeWidth returns the tree pane width.
func (m *Model) getTreeWidth() int {
	if m.treeWidth > 0 {
		tw := max(m.treeWidth, minTreeWidth)
		maxWidth := m.width * maxTreeWidthPercent / 100
		if tw > maxWidth {
			tw = maxWidth
		}
		return tw
	}
	return m.width * defaultTreePercent / 100
}

// getContentHeight returns the content area height.
func (m *Model) getContentHeight() int {
	chrome := headerHeight + tabBarHeight + footerHeight
	return max(m.height-chrome, minContentHeight)
}

// lineNumWidthFor returns the gutter width needed for n lines.
// Format: "[marker][digits][space]" where marker is " " or "▎".
func lineNumWidthFor(n int) int {
	digits := 1
	for v := n; v >= 10; v /= 10 {
		digits++
	}
	w := 1 + digits + 1 // marker + digits + trailing space
	return max(w, minLineNumberWidth)
}

func (m *Model) computeLayout() layout {
	tw := m.getTreeWidth()
	lnw := minLineNumberWidth
	t, ok := m.activeTabState()
	if ok && len(t.lines) > 0 {
		lnw = lineNumWidthFor(len(t.lines))
	}
	return layout{
		contentHeight: m.getContentHeight(),
		treeWidth:     tw,
		editorStartX:  tw + separatorWidth,
		editorWidth:   m.width - tw - separatorWidth,
		lineNumWidth:  lnw,
	}
}
