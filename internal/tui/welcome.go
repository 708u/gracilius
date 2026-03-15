package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/708u/gracilius/internal/tui/render"
)

const asciiArt = `                       _ _ _
   __ _ _ __ __ _  ___(_) (_)_   _ ___
  / _` + "`" + ` | '__/ _` + "`" + ` |/ __| | | | | | / __|
 | (_| | | | (_| | (__| | | | |_| \__ \
  \__, |_|  \__,_|\___|_|_|_|\__,_|___/
  |___/`

type helpEntry struct {
	key  string
	desc string
}

type helpSection struct {
	title   string
	entries []helpEntry
}

var welcomeHelp = []helpSection{
	{
		title: "Navigation",
		entries: []helpEntry{
			{"Enter", "Open file / Toggle dir"},
			{"Up/k  Down/j", "Navigate"},
			{"Left/h  Right/l", "Collapse / Expand"},
		},
	},
	{
		title: "Tabs",
		entries: []helpEntry{
			{"Tab", "Switch pane"},
			{"L / H", "Next / Prev tab"},
			{"q", "Close tab"},
		},
	},
	{
		title: "Editor",
		entries: []helpEntry{
			{"v / V", "Select / Select line"},
			{"y", "Copy selection"},
			{"i", "Add comment"},
			{"D", "Clear comments"},
		},
	},
}

// renderWelcome generates the welcome screen as a []string
// of exactly height lines, each padded to width.
func renderWelcome(width, height int, theme render.Theme) []string {
	stylePrimary := lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.TabActiveFg))
	styleSecondary := lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.TabInactiveFg))
	styleSection := lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.TabActiveBorder))

	styleLeaf := lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.LogoLeaf))
	styleTrunk := lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.LogoTrunk))

	// Build content lines with relative indentation.
	var raw []string

	artLines := strings.Split(asciiArt, "\n")
	leafCount := 4 // top 4 lines: leaf/canopy
	for i, l := range artLines {
		if i < leafCount {
			raw = append(raw, styleLeaf.Render(l))
		} else {
			raw = append(raw, styleTrunk.Render(l))
		}
	}

	raw = append(raw,
		"",
		styleLeaf.Render("  The human in the loop."),
		"",
	)

	for _, sec := range welcomeHelp {
		raw = append(raw,
			"",
			styleSection.Render("  "+sec.title),
		)
		for _, e := range sec.entries {
			key := render.PadRight(e.key, 16)
			line := "    " + stylePrimary.Render(key) +
				styleSecondary.Render(e.desc)
			raw = append(raw, line)
		}
	}

	// Find the widest raw line for horizontal centering.
	maxW := 0
	for _, l := range raw {
		if w := lipgloss.Width(l); w > maxW {
			maxW = w
		}
	}

	leftPad := 0
	if maxW < width {
		leftPad = (width - maxW) / 2
	}
	padding := strings.Repeat(" ", leftPad)

	// Vertical centering.
	topPad := 0
	if len(raw) < height {
		topPad = (height - len(raw)) / 2
	}

	result := make([]string, 0, height)

	for i := 0; i < topPad; i++ {
		result = append(result, render.PadRight("", width))
	}
	for _, l := range raw {
		if len(result) >= height {
			break
		}
		result = append(result, padding+l)
	}
	for len(result) < height {
		result = append(result, render.PadRight("", width))
	}

	return result
}
