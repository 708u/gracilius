package git

import (
	"fmt"
	"os/exec"
	"strings"
)

// gitCmd runs git -C {dir} {args...} and returns stdout.
// GIT_TERMINAL_PROMPT=0 to prevent interactive prompts.
func gitCmd(dir string, args ...string) ([]byte, error) {
	cmdArgs := append([]string{"-C", dir}, args...)
	cmd := exec.Command("git", cmdArgs...)
	cmd.Env = append(cmd.Environ(), "GIT_TERMINAL_PROMPT=0")
	return cmd.Output()
}

// RepoRoot returns the repo root for dir,
// or error if not a git repo.
func RepoRoot(dir string) (string, error) {
	out, err := gitCmd(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("not a git repository: %s", dir)
	}
	return strings.TrimSpace(string(out)), nil
}
