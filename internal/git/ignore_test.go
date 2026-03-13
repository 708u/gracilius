package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// initGitRepo initializes a bare-minimum git repo in dir
// so that git check-ignore works.
func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	for _, args := range [][]string{
		{"init"},
		{"config", "user.email", "test@test.com"},
		{"config", "user.name", "test"},
	} {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

func TestCheckIgnored_MatchesFiles(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	// Write .gitignore that ignores *.log and build/ directory
	if err := os.WriteFile(
		filepath.Join(dir, ".gitignore"),
		[]byte("*.log\nbuild/\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}

	ignored := CheckIgnored(dir, []string{
		"app.log",
		"main.go",
		"build/",
		"src/",
	})

	if !ignored["app.log"] {
		t.Error("app.log should be ignored")
	}
	if ignored["main.go"] {
		t.Error("main.go should not be ignored")
	}
	if !ignored["build/"] {
		t.Error("build/ should be ignored")
	}
	if ignored["src/"] {
		t.Error("src/ should not be ignored")
	}
}

func TestCheckIgnored_EmptyList(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	result := CheckIgnored(dir, nil)
	if result != nil {
		t.Errorf("expected nil for empty input, got %v", result)
	}

	result = CheckIgnored(dir, []string{})
	if result != nil {
		t.Errorf("expected nil for empty slice, got %v", result)
	}
}

func TestCheckIgnored_NoMatch(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	if err := os.WriteFile(
		filepath.Join(dir, ".gitignore"),
		[]byte("*.log\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}

	result := CheckIgnored(dir, []string{"main.go", "README.md"})
	if len(result) != 0 {
		t.Errorf("expected no matches, got %v", result)
	}
}

func TestCheckIgnored_NonGitDir(t *testing.T) {
	dir := t.TempDir()

	// Should not panic or error — just returns nil.
	result := CheckIgnored(dir, []string{"some/file.txt"})
	if result != nil {
		t.Errorf("expected nil for non-git dir, got %v", result)
	}
}
