package git

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ChangedFile represents a file changed in the working tree.
type ChangedFile struct {
	Path       string
	Status     string   // A, M, D, R, ?
	OldContent []string // nil for new files
	NewContent []string // nil for deleted files
	Binary     bool
}

// blobReader reads content of a file from some source.
type blobReader func(dir, path string) ([]string, bool, error)

// workFileReader returns a blobReader that reads from the working tree.
func workFileReader(root string) blobReader {
	return func(_, relPath string) ([]string, bool, error) {
		return readWorkFile(root, relPath)
	}
}

// ChangedFiles returns unstaged changed files.
// Runs: git diff --name-status
func ChangedFiles(dir string) ([]ChangedFile, error) {
	root, err := repoRoot(dir)
	if err != nil {
		return nil, err
	}

	out, err := gitCmd(dir, "diff", "--name-status")
	if err != nil {
		return nil, fmt.Errorf("git diff: %w", err)
	}

	return parseChangedFiles(dir, out, readGitBlob, workFileReader(root))
}

// StagedFiles returns staged (cached) changed files.
// Runs: git diff --cached --name-status
func StagedFiles(dir string) ([]ChangedFile, error) {
	_, err := repoRoot(dir)
	if err != nil {
		return nil, err
	}

	out, err := gitCmd(dir, "diff", "--cached", "--name-status")
	if err != nil {
		return nil, fmt.Errorf("git diff --cached: %w", err)
	}

	return parseChangedFiles(dir, out, readHEADBlob, readGitBlob)
}

// UntrackedFiles returns untracked files.
// Runs: git ls-files --others --exclude-standard
func UntrackedFiles(dir string) ([]ChangedFile, error) {
	root, err := repoRoot(dir)
	if err != nil {
		return nil, err
	}

	out, err := gitCmd(dir, "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return nil, fmt.Errorf("git ls-files: %w", err)
	}

	output := strings.TrimSpace(string(out))
	if output == "" {
		return nil, nil
	}

	var files []ChangedFile
	for path := range strings.SplitSeq(output, "\n") {
		if path == "" {
			continue
		}
		cf := ChangedFile{
			Path:   path,
			Status: "?",
		}
		content, bin, readErr := readWorkFile(root, path)
		if readErr != nil {
			return nil, readErr
		}
		cf.Binary = bin
		if !bin {
			cf.NewContent = content
		}
		files = append(files, cf)
	}

	return files, nil
}

// parseChangedFiles parses git diff --name-status output
// using the provided blob readers for old and new content.
func parseChangedFiles(
	dir string,
	nameStatusOutput []byte,
	readOld, readNew blobReader,
) ([]ChangedFile, error) {
	output := strings.TrimSpace(string(nameStatusOutput))
	if output == "" {
		return nil, nil
	}

	var files []ChangedFile
	for line := range strings.SplitSeq(output, "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) < 2 {
			continue
		}

		status := fields[0]
		cf := ChangedFile{}

		switch {
		case status == "A":
			cf.Status = "A"
			cf.Path = fields[1]
			content, bin, readErr := readNew(dir, cf.Path)
			if readErr != nil {
				return nil, readErr
			}
			cf.Binary = bin
			if !bin {
				cf.NewContent = content
			}

		case status == "M":
			cf.Status = "M"
			cf.Path = fields[1]
			old, oldBin, oldErr := readOld(dir, cf.Path)
			if oldErr != nil {
				return nil, oldErr
			}
			new_, newBin, newErr := readNew(dir, cf.Path)
			if newErr != nil {
				return nil, newErr
			}
			cf.Binary = oldBin || newBin
			if !cf.Binary {
				cf.OldContent = old
				cf.NewContent = new_
			}

		case status == "D":
			cf.Status = "D"
			cf.Path = fields[1]
			old, bin, oldErr := readOld(dir, cf.Path)
			if oldErr != nil {
				return nil, oldErr
			}
			cf.Binary = bin
			if !bin {
				cf.OldContent = old
			}

		case strings.HasPrefix(status, "R"):
			cf.Status = "R"
			if len(fields) < 3 {
				continue
			}
			oldPath := fields[1]
			newPath := fields[2]
			cf.Path = newPath
			old, oldBin, oldErr := readOld(dir, oldPath)
			if oldErr != nil {
				return nil, oldErr
			}
			new_, newBin, newErr := readNew(dir, newPath)
			if newErr != nil {
				return nil, newErr
			}
			cf.Binary = oldBin || newBin
			if !cf.Binary {
				cf.OldContent = old
				cf.NewContent = new_
			}

		default:
			continue
		}

		files = append(files, cf)
	}

	return files, nil
}

func readWorkFile(root, relPath string) ([]string, bool, error) {
	abs := filepath.Join(root, relPath)
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, false, fmt.Errorf("read file %s: %w", relPath, err)
	}
	if isBinaryContent(data) {
		return nil, true, nil
	}
	return splitLines(data), false, nil
}

func readGitBlob(dir, path string) ([]string, bool, error) {
	data, err := gitCmd(dir, "show", ":"+path)
	if err != nil {
		return nil, false, fmt.Errorf("git show :%s: %w", path, err)
	}
	if isBinaryContent(data) {
		return nil, true, nil
	}
	return splitLines(data), false, nil
}

// readHEADBlob reads a file from HEAD.
// Returns (nil, false, nil) if HEAD does not exist (no commits yet).
func readHEADBlob(dir, path string) ([]string, bool, error) {
	data, err := gitCmd(dir, "show", "HEAD:"+path)
	if err != nil {
		// HEAD doesn't exist (no commits) or file not in HEAD.
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("git show HEAD:%s: %w", path, err)
	}
	if isBinaryContent(data) {
		return nil, true, nil
	}
	return splitLines(data), false, nil
}

func isBinaryContent(data []byte) bool {
	checkSize := min(len(data), 8192)
	for i := range checkSize {
		if data[i] == 0 {
			return true
		}
	}
	return false
}

func splitLines(data []byte) []string {
	s := string(data)
	if s == "" {
		return []string{}
	}
	s = strings.TrimSuffix(s, "\n")
	return strings.Split(s, "\n")
}
