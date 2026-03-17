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

func TestCheckIgnored(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		gitignore string
		paths     []string
		wantMap   map[string]bool // nil means expect nil result
	}{
		{
			name:      "MatchesFiles",
			gitignore: "*.log\nbuild/\n",
			paths:     []string{"app.log", "main.go", "build/", "src/"},
			wantMap:   map[string]bool{"app.log": true, "build/": true},
		},
		{
			name:    "EmptyNilInput",
			paths:   nil,
			wantMap: nil,
		},
		{
			name:    "EmptySliceInput",
			paths:   []string{},
			wantMap: nil,
		},
		{
			name:      "NoMatch",
			gitignore: "*.log\n",
			paths:     []string{"main.go", "README.md"},
			wantMap:   map[string]bool{},
		},
		{
			name:    "NonGitDir_no_init",
			paths:   []string{"some/file.txt"},
			wantMap: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()

			skipInit := tt.name == "NonGitDir_no_init"
			if !skipInit {
				initGitRepo(t, dir)
			}

			if tt.gitignore != "" {
				if err := os.WriteFile(
					filepath.Join(dir, ".gitignore"),
					[]byte(tt.gitignore),
					0o644,
				); err != nil {
					t.Fatal(err)
				}
			}

			result := CheckIgnored(dir, tt.paths)

			if tt.wantMap == nil {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
				return
			}

			for path, wantIgnored := range tt.wantMap {
				if result[path] != wantIgnored {
					t.Errorf("%s: expected ignored=%v, got %v", path, wantIgnored, result[path])
				}
			}

			// Verify non-listed paths are not ignored.
			for _, path := range tt.paths {
				if _, listed := tt.wantMap[path]; !listed {
					if result[path] {
						t.Errorf("%s: should not be ignored", path)
					}
				}
			}
		})
	}
}
