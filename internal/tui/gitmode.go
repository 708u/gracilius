package tui

import tea "charm.land/bubbletea/v2"

// gitDiffMode identifies which diff comparison is active.
type gitDiffMode int

const (
	gitModeUncommitted gitDiffMode = iota
	gitModeUnstaged
	gitModeStaged
	gitModeBranch
	gitDiffModeCount
)

// label returns the display name for the diff mode.
func (m gitDiffMode) label() string {
	switch m {
	case gitModeUncommitted:
		return "Uncommitted"
	case gitModeUnstaged:
		return "Unstaged"
	case gitModeStaged:
		return "Staged"
	case gitModeBranch:
		return "Branch"
	default:
		return "Uncommitted"
	}
}

// tabPrefix returns the bracketed prefix for diff tab labels.
func (m gitDiffMode) tabPrefix() string {
	switch m {
	case gitModeUncommitted:
		return "[uncommit]"
	case gitModeUnstaged:
		return "[unstaged]"
	case gitModeStaged:
		return "[staged]"
	case gitModeBranch:
		return "[branch]"
	default:
		return "[diff]"
	}
}

// gitPanelState holds per-mode state for the git changes panel.
type gitPanelState struct {
	entries      []changedFileEntry
	cursor       int
	scrollOffset int
	loaded       bool
	stale        bool // needs reload on next access
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
