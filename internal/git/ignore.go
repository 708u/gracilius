package git

import (
	"os/exec"
	"strings"
)

// CheckIgnored returns the set of paths (from the input list) that are
// ignored by .gitignore rules in the given repository root.
// Paths should be relative to repoRoot; directories should have a trailing "/".
// Exit code 1 (no match) is not treated as an error.
func CheckIgnored(repoRoot string, paths []string) map[string]bool {
	if len(paths) == 0 {
		return nil
	}

	cmd := exec.Command("git", "-C", repoRoot, "check-ignore", "--stdin")
	cmd.Env = append(cmd.Environ(), "GIT_TERMINAL_PROMPT=0")
	cmd.Stdin = strings.NewReader(strings.Join(paths, "\n") + "\n")

	out, err := cmd.Output()
	if err != nil {
		// exit code 1 means no paths matched; all other errors
		// are treated as "nothing ignored".
		return nil
	}

	result := make(map[string]bool)
	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			result[line] = true
		}
	}
	return result
}
