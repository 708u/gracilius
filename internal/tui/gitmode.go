package tui

import tea "charm.land/bubbletea/v2"

// gitDiffMode identifies which diff comparison is active.
type gitDiffMode int

const (
	gitModeWorking gitDiffMode = iota // staged/unstaged/untracked with categories
	gitModeBranch                     // merge-base(default-branch)..HEAD
	gitDiffModeCount
)

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
	switch m {
	case gitModeWorking:
		return "[working]"
	case gitModeBranch:
		if defaultBranch != "" {
			return "[vs " + defaultBranch + "]"
		}
		return "[vs main]"
	default:
		return "[working]"
	}
}

// gitPanelState holds per-mode state for the git changes panel.
type gitPanelState struct {
	entries          []changedFileEntry
	visualRows       []gitVisualRow
	entryToVisualIdx map[int]int // entryIdx -> visual row index
	cursor           int
	scrollOffset     int
	loaded           bool
	stale            bool // needs reload on next access
}

// switchGitMode changes the active git diff mode by delta (-1 or +1).
// Returns a tea.Cmd if the new mode needs loading or is stale.
func (m *Model) switchGitMode(delta int) tea.Cmd {
	count := int(gitDiffModeCount)
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
