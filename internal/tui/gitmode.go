package tui

import tea "charm.land/bubbletea/v2"

// gitDiffMode identifies which diff comparison is active.
type gitDiffMode int

const (
	gitModeWorking gitDiffMode = iota // staged/unstaged/untracked with categories
	gitModeBranch                     // merge-base(default-branch)..HEAD
)

// gitDiffModes lists all valid diff modes.
var gitDiffModes = []gitDiffMode{
	gitModeWorking,
	gitModeBranch,
}

// label returns the display name for the diff mode.
// For gitModeBranch, pass the default branch name to show "vs <branch>".
func (m gitDiffMode) label(defaultBranch string) string {
	switch m {
	case gitModeWorking:
		return "working"
	case gitModeBranch:
		if defaultBranch != "" {
			return "vs " + defaultBranch
		}
		return "vs main"
	default:
		return "working"
	}
}

// tabPrefix returns the bracketed prefix for diff tab labels.
func (m gitDiffMode) tabPrefix(defaultBranch string) string {
	return "[" + m.label(defaultBranch) + "]"
}

// gitPanelState holds per-mode state for the git changes panel.
type gitPanelState struct {
	entries          []changedFileEntry
	visualRows       []gitVisualRow
	entryToVisualIdx map[int]int // entryIdx -> visual row index
	cursor           int
	scrollOffset     int
	loaded           bool
	stale            bool            // needs reload on next access
	viewed           map[string]bool // relative path -> viewed flag
	viewedCount      int             // cached count of viewed entries
}

// toggleViewed flips the viewed state for the given file name.
func (gs *gitPanelState) toggleViewed(name string) {
	if gs.viewed == nil {
		gs.viewed = make(map[string]bool)
	}
	if gs.viewed[name] {
		gs.viewed[name] = false
		gs.viewedCount--
	} else {
		gs.viewed[name] = true
		gs.viewedCount++
	}
}

// recomputeViewedCount recalculates the cached viewed count from scratch.
// Called when entries are replaced (e.g. after git reload).
func (gs *gitPanelState) recomputeViewedCount() {
	gs.viewedCount = 0
	for i := range gs.entries {
		if gs.viewed[gs.entries[i].name] {
			gs.viewedCount++
		}
	}
}

// viewedCountTotal returns (viewed, total) counts for all entries.
func (gs *gitPanelState) viewedCountTotal() (int, int) {
	return gs.viewedCount, len(gs.entries)
}

// switchGitMode changes the active git diff mode by delta (-1 or +1).
// Returns a tea.Cmd if the new mode needs loading or is stale.
func (m *Model) switchGitMode(delta int) tea.Cmd {
	count := len(gitDiffModes)
	m.gitDiffMode = gitDiffMode((int(m.gitDiffMode) + delta + count) % count)

	gs := m.gitState()
	if !gs.loaded || gs.stale {
		if m.gitDiffMode == gitModeBranch && m.gitMergeBase == "" {
			return m.initGitBranchInfoAsync()
		}
		gs.stale = false
		return m.loadGitChangesForMode(m.gitDiffMode)
	}
	return nil
}
