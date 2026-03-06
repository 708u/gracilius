package git

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ChangedFile represents a file changed in the working tree.
type ChangedFile struct {
	Path       string
	Status     string   // A, M, D, R
	OldContent []string // nil for new files
	NewContent []string // nil for deleted files
	Binary     bool
}

// ChangedFiles returns unstaged changed files.
// Runs: git diff --name-status
// Content retrieval per status:
//
//	A: old=nil,              new=os.ReadFile(path)
//	M: old=git show :{path}, new=os.ReadFile(path)
//	D: old=git show :{path}, new=nil
//	R: old=git show :{old},  new=os.ReadFile(new)
func ChangedFiles(dir string) ([]ChangedFile, error) {
	root, err := repoRoot(dir)
	if err != nil {
		return nil, err
	}

	out, err := gitCmd(dir, "diff", "--name-status")
	if err != nil {
		return nil, fmt.Errorf("git diff: %w", err)
	}

	output := strings.TrimSpace(string(out))
	if output == "" {
		return nil, nil
	}

	var files []ChangedFile
	for _, line := range strings.Split(output, "\n") {
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
			content, bin, readErr := readWorkFile(root, cf.Path)
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
			old, oldBin, oldErr := readGitBlob(dir, cf.Path)
			if oldErr != nil {
				return nil, oldErr
			}
			new_, newBin, newErr := readWorkFile(root, cf.Path)
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
			old, bin, oldErr := readGitBlob(dir, cf.Path)
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
			old, oldBin, oldErr := readGitBlob(dir, oldPath)
			if oldErr != nil {
				return nil, oldErr
			}
			new_, newBin, newErr := readWorkFile(root, newPath)
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
