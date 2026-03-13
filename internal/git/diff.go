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

// DiffMode selects which pair of trees to compare.
type DiffMode int

const (
	DiffUncommitted DiffMode = iota // HEAD vs working tree
	DiffUnstaged                    // index vs working tree
	DiffStaged                      // HEAD vs index
	DiffBranch                      // merge-base vs HEAD
)

// DiffOptions controls how ChangedFilesWithOptions retrieves diffs.
type DiffOptions struct {
	Mode    DiffMode
	BaseRef string // merge-base hash (DiffBranch only)
}

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
	root, err := repoRoot(dir)
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
	return s.parseChangedFiles(out, s.unstaged)
}

// StagedFiles returns staged (cached) changed files.
func (s *StatusReader) StagedFiles() ([]ChangedFile, error) {
	out, err := gitCmd(s.dir, "diff", "--cached", "--name-status")
	if err != nil {
		return nil, fmt.Errorf("git diff --cached: %w", err)
	}
	return s.parseChangedFiles(out, s.staged)
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
func (s *StatusReader) parseChangedFiles(
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
			content, bin, readErr := readers.readNew(s.dir, cf.Path)
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
			old, oldBin, oldErr := readers.readOld(s.dir, cf.Path)
			if oldErr != nil {
				return nil, oldErr
			}
			new_, newBin, newErr := readers.readNew(s.dir, cf.Path)
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
			old, bin, oldErr := readers.readOld(s.dir, cf.Path)
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
			old, oldBin, oldErr := readers.readOld(s.dir, oldPath)
			if oldErr != nil {
				return nil, oldErr
			}
			new_, newBin, newErr := readers.readNew(s.dir, newPath)
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

// ChangedFilesWithOptions returns changed files for the given diff mode.
func ChangedFilesWithOptions(dir string, opts DiffOptions) ([]ChangedFile, error) {
	root, err := repoRoot(dir)
	if err != nil {
		return nil, err
	}

	var diffArgs []string
	switch opts.Mode {
	case DiffUncommitted:
		diffArgs = []string{"diff", "HEAD", "--name-status"}
	case DiffUnstaged:
		diffArgs = []string{"diff", "--name-status"}
	case DiffStaged:
		diffArgs = []string{"diff", "--cached", "--name-status"}
	case DiffBranch:
		if opts.BaseRef == "" {
			return nil, fmt.Errorf("BaseRef required for DiffBranch")
		}
		diffArgs = []string{"diff", opts.BaseRef + "..HEAD", "--name-status"}
	default:
		return nil, fmt.Errorf("unknown diff mode: %d", opts.Mode)
	}

	out, err := gitCmd(dir, diffArgs...)
	if err != nil {
		// HEAD may not exist yet (empty repo).
		if opts.Mode == DiffUncommitted || opts.Mode == DiffStaged {
			return nil, nil
		}
		return nil, fmt.Errorf("git diff: %w", err)
	}

	oldRef, newRef := refsForMode(opts)
	dr := diffReader{
		readOld: func(d, path string) ([]string, bool, error) {
			return readRef(d, root, path, oldRef)
		},
		readNew: func(d, path string) ([]string, bool, error) {
			return readRef(d, root, path, newRef)
		},
	}

	sr := &StatusReader{dir: dir, root: root}
	files, err := sr.parseChangedFiles(out, dr)
	if err != nil {
		return nil, err
	}

	if includesUntracked(opts.Mode) {
		reader, err := NewStatusReader(dir)
		if err != nil {
			return nil, err
		}
		untracked, err := reader.UntrackedFiles()
		if err != nil {
			return nil, err
		}
		files = append(files, untracked...)
	}

	return files, nil
}

// includesUntracked returns true if the mode should list untracked files.
func includesUntracked(mode DiffMode) bool {
	return mode == DiffUncommitted || mode == DiffUnstaged
}

// refKind identifies how to read a file version.
type refKind int

const (
	refIndex   refKind = iota // git show :<path>
	refHEAD                   // git show HEAD:<path>
	refCommit                 // git show <hash>:<path>
	refWorkDir                // os.ReadFile
)

type refSpec struct {
	kind refKind
	ref  string // commit hash for refCommit
}

// refsForMode returns (oldRef, newRef) specs for the given diff mode.
func refsForMode(opts DiffOptions) (old, new_ refSpec) {
	switch opts.Mode {
	case DiffUncommitted:
		return refSpec{kind: refHEAD}, refSpec{kind: refWorkDir}
	case DiffUnstaged:
		return refSpec{kind: refIndex}, refSpec{kind: refWorkDir}
	case DiffStaged:
		return refSpec{kind: refHEAD}, refSpec{kind: refIndex}
	case DiffBranch:
		return refSpec{kind: refCommit, ref: opts.BaseRef}, refSpec{kind: refHEAD}
	}
	return refSpec{kind: refIndex}, refSpec{kind: refWorkDir}
}

// readRef reads file content for the given refSpec.
func readRef(dir, root, path string, spec refSpec) ([]string, bool, error) {
	switch spec.kind {
	case refWorkDir:
		return readWorkFile(root, path)
	case refIndex:
		return readGitBlob(dir, path)
	case refHEAD:
		return readHEADBlob(dir, path)
	case refCommit:
		data, err := gitCmd(dir, "show", spec.ref+":"+path)
		if err != nil {
			return nil, false, fmt.Errorf("git show %s:%s: %w", spec.ref, path, err)
		}
		if isBinaryContent(data) {
			return nil, true, nil
		}
		return splitLines(data), false, nil
	}
	return nil, false, fmt.Errorf("unknown ref kind: %d", spec.kind)
}

// MergeBase returns the merge-base between HEAD and the given ref.
func MergeBase(dir, ref string) (string, error) {
	out, err := gitCmd(dir, "merge-base", "HEAD", ref)
	if err != nil {
		return "", fmt.Errorf("git merge-base: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// DefaultBranch detects the default branch (main or master).
func DefaultBranch(dir string) (string, error) {
	for _, name := range []string{"main", "master"} {
		_, err := gitCmd(dir, "rev-parse", "--verify", name)
		if err == nil {
			return name, nil
		}
	}
	return "", fmt.Errorf("no default branch found (tried main, master)")
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
