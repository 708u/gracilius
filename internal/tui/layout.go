package tui

// Screen geometry constants.
const (
	paneHeaderRows      = 2 // panel header + mode/tab underline rows
	footerHeight        = 4 // border-top + help + status
	separatorWidth      = 3
	minLineNumberWidth  = 4
	tabSeparatorWidth   = 1 // " " between tab labels
	minTreeWidth        = 15
	defaultTreePercent  = 30
	maxTreeWidthPercent = 70
	minContentHeight    = 5
)

// Comment / input block geometry (renderBlock).
const (
	commentBlockMargin = 4 // left/right padding around comment block
	commentBorderChars = 3 // "│ " (2) + "│" (1)
	blockBorderLeft    = 2 // "│ " prefix in body row
	blockBorderTop     = 1 // top border row
)

// layout holds all derived dimensions for a single render frame.
// Computed fresh on each View() / mouse-handling pass; never stored.
//
// Vertical:
//
//	+-----------------------+ --
//	| panel hdr | tab label |  2  paneHeaderRows
//	| mode/stat | tab uline |
//	+-----------------------+ --
//	| tree | sep | editor   |  paneBodyHeight
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
	contentHeight  int // total usable rows (paneHeaderRows + paneBodyHeight)
	paneBodyHeight int // usable rows for tree body and editor content
	treeWidth      int // file tree pane width
	editorStartX   int // treeWidth + separatorWidth
	editorWidth    int // total width - treeWidth - separatorWidth
	lineNumWidth   int // line number gutter width (digits + 1 space)
	textWidth      int // editorWidth - lineNumWidth
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

// getContentHeight returns the total content area height
// (pane header + body).
func (m *Model) getContentHeight() int {
	return max(m.height-footerHeight, minContentHeight)
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
	lnw := minLineNumberWidth
	t, ok := m.activeTabState()
	if ok && len(t.lines) > 0 {
		lnw = lineNumWidthFor(len(t.lines))
	}

	tw := 0
	sx := 0
	ew := m.width
	if m.sidebarVisible {
		tw = m.getTreeWidth()
		sx = tw + separatorWidth
		ew = m.width - tw - separatorWidth
	}

	ch := m.getContentHeight()

	return layout{
		contentHeight:  ch,
		paneBodyHeight: ch - paneHeaderRows,
		treeWidth:      tw,
		editorStartX:   sx,
		editorWidth:    ew,
		lineNumWidth:   lnw,
		textWidth:      ew - lnw,
	}
}
