package git

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// FileStatus represents a git file status.
type FileStatus string

// String implements fmt.Stringer.
func (s FileStatus) String() string { return string(s) }

// Git file status constants.
const (
	StatusAdded     FileStatus = "A"
	StatusModified  FileStatus = "M"
	StatusDeleted   FileStatus = "D"
	StatusRenamed   FileStatus = "R"
	StatusUntracked FileStatus = "?"
)

// ChangedFile represents a file changed in the working tree.
type ChangedFile struct {
	Path       string
	Status     FileStatus
	OldContent []string // nil for new files
	NewContent []string // nil for deleted files
	Binary     bool
}

// blobReader reads file content from a specific source.
type blobReader func(dir, path string) ([]string, bool, error)

// diffReader holds the old/new content readers for a diff.
type diffReader struct {
	readOld blobReader
	readNew blobReader
}

// StatusReader reads git status information from a repository.
type StatusReader struct {
	dir      string
	root     string
	staged   diffReader
	unstaged diffReader
}

// NewStatusReader creates a StatusReader for the given directory.
func NewStatusReader(dir string) (*StatusReader, error) {
	root, err := RepoRoot(dir)
	if err != nil {
		return nil, err
	}
	return &StatusReader{
		dir:  dir,
		root: root,
		staged: diffReader{
			readOld: readHEADBlob,
			readNew: readGitBlob,
		},
		unstaged: diffReader{
			readOld: readGitBlob,
			readNew: func(_, path string) ([]string, bool, error) {
				return readWorkFile(root, path)
			},
		},
	}, nil
}

// ChangedFiles returns unstaged changed files.
func (s *StatusReader) ChangedFiles() ([]ChangedFile, error) {
	out, err := gitCmd(s.dir, "diff", "--name-status")
	if err != nil {
		return nil, fmt.Errorf("git diff: %w", err)
	}
	return parseChangedFiles(s.dir, out, s.unstaged)
}

// StagedFiles returns staged (cached) changed files.
func (s *StatusReader) StagedFiles() ([]ChangedFile, error) {
	out, err := gitCmd(s.dir, "diff", "--cached", "--name-status")
	if err != nil {
		return nil, fmt.Errorf("git diff --cached: %w", err)
	}
	return parseChangedFiles(s.dir, out, s.staged)
}

// UntrackedFiles returns untracked files.
func (s *StatusReader) UntrackedFiles() ([]ChangedFile, error) {
	out, err := gitCmd(s.dir, "ls-files", "--others", "--exclude-standard")
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
			Status: StatusUntracked,
		}
		content, bin, readErr := readWorkFile(s.root, path)
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
// using the provided diffReader for old and new content.
func parseChangedFiles(
	dir string,
	nameStatusOutput []byte,
	readers diffReader,
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

		status := FileStatus(fields[0])
		cf := ChangedFile{}

		switch {
		case status == StatusAdded:
			cf.Status = StatusAdded
			cf.Path = fields[1]
			content, bin, readErr := readers.readNew(dir, cf.Path)
			if readErr != nil {
				return nil, readErr
			}
			cf.Binary = bin
			if !bin {
				cf.NewContent = content
			}

		case status == StatusModified:
			cf.Status = StatusModified
			cf.Path = fields[1]
			old, oldBin, oldErr := readers.readOld(dir, cf.Path)
			if oldErr != nil {
				return nil, oldErr
			}
			new_, newBin, newErr := readers.readNew(dir, cf.Path)
			if newErr != nil {
				return nil, newErr
			}
			cf.Binary = oldBin || newBin
			if !cf.Binary {
				cf.OldContent = old
				cf.NewContent = new_
			}

		case status == StatusDeleted:
			cf.Status = StatusDeleted
			cf.Path = fields[1]
			old, bin, oldErr := readers.readOld(dir, cf.Path)
			if oldErr != nil {
				return nil, oldErr
			}
			cf.Binary = bin
			if !bin {
				cf.OldContent = old
			}

		case strings.HasPrefix(status.String(), StatusRenamed.String()):
			cf.Status = StatusRenamed
			if len(fields) < 3 {
				continue
			}
			oldPath := fields[1]
			newPath := fields[2]
			cf.Path = newPath
			old, oldBin, oldErr := readers.readOld(dir, oldPath)
			if oldErr != nil {
				return nil, oldErr
			}
			new_, newBin, newErr := readers.readNew(dir, newPath)
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

// BranchDiff returns changed files between merge-base and HEAD.
func BranchDiff(dir, baseRef string) ([]ChangedFile, error) {
	if baseRef == "" {
		return nil, fmt.Errorf("baseRef required for BranchDiff")
	}

	out, err := gitCmd(dir, "diff", baseRef+"..HEAD", "--name-status")
	if err != nil {
		return nil, fmt.Errorf("git diff: %w", err)
	}

	dr := diffReader{
		readOld: func(d, path string) ([]string, bool, error) {
			return readCommitBlob(d, baseRef, path)
		},
		readNew: readHEADBlob,
	}

	return parseChangedFiles(dir, out, dr)
}

// MergeBase returns the merge-base between HEAD and the given ref.
func MergeBase(dir, ref string) (string, error) {
	out, err := gitCmd(dir, "merge-base", "HEAD", ref)
	if err != nil {
		return "", fmt.Errorf("git merge-base: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// DefaultBranch detects the default branch from the remote HEAD.
// Falls back to checking main/master if remote HEAD is unavailable.
func DefaultBranch(dir string) (string, error) {
	out, err := gitCmd(dir, "symbolic-ref", "refs/remotes/origin/HEAD")
	if err == nil {
		ref := strings.TrimSpace(string(out))
		// refs/remotes/origin/main -> main
		if name, ok := strings.CutPrefix(ref, "refs/remotes/origin/"); ok {
			return name, nil
		}
	}
	for _, name := range []string{"main", "master"} {
		_, err := gitCmd(dir, "rev-parse", "--verify", name)
		if err == nil {
			return name, nil
		}
	}
	return "", fmt.Errorf("no default branch found")
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

// readGitBlob reads content from git show :<path> (index).
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
		if _, ok := errors.AsType[*exec.ExitError](err); ok {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("git show HEAD:%s: %w", path, err)
	}
	if isBinaryContent(data) {
		return nil, true, nil
	}
	return splitLines(data), false, nil
}

// readCommitBlob reads a file from a specific commit.
func readCommitBlob(dir, ref, path string) ([]string, bool, error) {
	data, err := gitCmd(dir, "show", ref+":"+path)
	if err != nil {
		return nil, false, fmt.Errorf("git show %s:%s: %w", ref, path, err)
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
