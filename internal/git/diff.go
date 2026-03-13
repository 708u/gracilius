package git

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	Status     string   // A, M, D, R, ?
	OldContent []string // nil for new files
	NewContent []string // nil for deleted files
	Binary     bool
}

// ChangedFiles returns unstaged changed files (index vs working tree).
// This is a convenience wrapper around ChangedFilesWithOptions.
func ChangedFiles(dir string) ([]ChangedFile, error) {
	return ChangedFilesWithOptions(dir, DiffOptions{Mode: DiffUnstaged})
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

	output := strings.TrimSpace(string(out))
	if output == "" && !includesUntracked(opts.Mode) {
		return nil, nil
	}

	oldRef, newRef := refsForMode(opts)

	var files []ChangedFile
	if output != "" {
		for line := range strings.SplitSeq(output, "\n") {
			cf, ok, parseErr := parseNameStatus(dir, root, line, oldRef, newRef)
			if parseErr != nil {
				return nil, parseErr
			}
			if ok {
				files = append(files, cf)
			}
		}
	}

	if includesUntracked(opts.Mode) {
		untracked, utErr := UntrackedFiles(dir)
		if utErr != nil {
			return nil, utErr
		}
		for _, p := range untracked {
			cf := ChangedFile{Status: "?", Path: p}
			content, bin, readErr := readWorkFile(root, p)
			if readErr != nil {
				return nil, readErr
			}
			cf.Binary = bin
			if !bin {
				cf.NewContent = content
			}
			files = append(files, cf)
		}
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
		return readGitBlob(dir, ":"+path)
	case refHEAD:
		return readGitBlob(dir, "HEAD:"+path)
	case refCommit:
		return readGitBlob(dir, spec.ref+":"+path)
	}
	return nil, false, fmt.Errorf("unknown ref kind: %d", spec.kind)
}

// parseNameStatus parses a single line from git diff --name-status output.
func parseNameStatus(dir, root, line string, oldRef, newRef refSpec) (ChangedFile, bool, error) {
	fields := strings.Split(line, "\t")
	if len(fields) < 2 {
		return ChangedFile{}, false, nil
	}

	status := fields[0]
	cf := ChangedFile{}

	switch {
	case status == "A":
		cf.Status = "A"
		cf.Path = fields[1]
		content, bin, err := readRef(dir, root, cf.Path, newRef)
		if err != nil {
			return cf, false, err
		}
		cf.Binary = bin
		if !bin {
			cf.NewContent = content
		}

	case status == "M":
		cf.Status = "M"
		cf.Path = fields[1]
		old, oldBin, oldErr := readRef(dir, root, cf.Path, oldRef)
		if oldErr != nil {
			return cf, false, oldErr
		}
		new_, newBin, newErr := readRef(dir, root, cf.Path, newRef)
		if newErr != nil {
			return cf, false, newErr
		}
		cf.Binary = oldBin || newBin
		if !cf.Binary {
			cf.OldContent = old
			cf.NewContent = new_
		}

	case status == "D":
		cf.Status = "D"
		cf.Path = fields[1]
		old, bin, oldErr := readRef(dir, root, cf.Path, oldRef)
		if oldErr != nil {
			return cf, false, oldErr
		}
		cf.Binary = bin
		if !bin {
			cf.OldContent = old
		}

	case strings.HasPrefix(status, "R"):
		cf.Status = "R"
		if len(fields) < 3 {
			return cf, false, nil
		}
		oldPath := fields[1]
		newPath := fields[2]
		cf.Path = newPath
		old, oldBin, oldErr := readRef(dir, root, oldPath, oldRef)
		if oldErr != nil {
			return cf, false, oldErr
		}
		new_, newBin, newErr := readRef(dir, root, newPath, newRef)
		if newErr != nil {
			return cf, false, newErr
		}
		cf.Binary = oldBin || newBin
		if !cf.Binary {
			cf.OldContent = old
			cf.NewContent = new_
		}

	default:
		return cf, false, nil
	}

	return cf, true, nil
}

// UntrackedFiles returns paths of untracked files.
func UntrackedFiles(dir string) ([]string, error) {
	out, err := gitCmd(dir, "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return nil, fmt.Errorf("git ls-files: %w", err)
	}
	output := strings.TrimSpace(string(out))
	if output == "" {
		return nil, nil
	}
	var paths []string
	for p := range strings.SplitSeq(output, "\n") {
		if p != "" {
			paths = append(paths, p)
		}
	}
	return paths, nil
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

// readGitBlob reads content from git show <ref> where ref is e.g. ":path", "HEAD:path".
func readGitBlob(dir, ref string) ([]string, bool, error) {
	data, err := gitCmd(dir, "show", ref)
	if err != nil {
		return nil, false, fmt.Errorf("git show %s: %w", ref, err)
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
