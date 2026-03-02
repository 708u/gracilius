package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const asciiArt = `                __ _  _ _
   __ _ _ _ __ _ __(_) (_)_  _ ___
  / _` + "`" + ` | '_/ _` + "`" + ` / _| | | | || (_-<
  \__, |_| \__,_\__|_|_|_|\_,_/__/
  |___/`

const subtitle = "Code Review TUI for Claude Code"

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
			{"\u2191/k  \u2193/j", "Navigate"},
			{"\u2190/h  \u2192/l", "Collapse / Expand"},
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

const quitHint = "Ctrl+C x2      Quit"

// renderWelcome generates the welcome screen as a single string
// that fills the given width x height area.
func renderWelcome(width, height int) string {
	stylePrimary := lipgloss.NewStyle().
		Foreground(lipgloss.Color(activeTheme.tabActiveFg))
	styleSecondary := lipgloss.NewStyle().
		Foreground(lipgloss.Color(activeTheme.tabInactiveFg))
	styleSection := lipgloss.NewStyle().
		Foreground(lipgloss.Color(activeTheme.tabActiveBorder))

	lines := make([]string, 0, 24)

	for _, l := range strings.Split(asciiArt, "\n") {
		lines = append(lines, stylePrimary.Render(l))
	}

	lines = append(lines, "")
	lines = append(lines, styleSecondary.Render("      "+subtitle))
	lines = append(lines, "")

	for _, sec := range welcomeHelp {
		lines = append(lines, "")
		lines = append(lines, styleSection.Render("  "+sec.title))
		for _, e := range sec.entries {
			key := stylePrimary.Render(padRight(e.key, 12))
			desc := styleSecondary.Render(e.desc)
			lines = append(lines, "    "+key+" "+desc)
		}
	}

	lines = append(lines, "")
	lines = append(lines, "  "+stylePrimary.Render(quitHint))

	content := strings.Join(lines, "\n")

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, content)
}
