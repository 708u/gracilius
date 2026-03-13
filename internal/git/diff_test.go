package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run(t, dir, "git", "init")
	run(t, dir, "git", "config", "user.email", "test@test.com")
	run(t, dir, "git", "config", "user.name", "test")
	return dir
}

func run(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v failed: %v\n%s", name, args, err, out)
	}
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestChangedFiles_NotGitRepo(t *testing.T) {
	dir := t.TempDir()
	_, err := ChangedFiles(dir)
	if err == nil {
		t.Fatal("expected error for non-git repo")
	}
}

func TestChangedFiles_NoChanges(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "hello.txt", "hello\n")
	run(t, dir, "git", "add", ".")
	run(t, dir, "git", "commit", "-m", "init")

	files, err := ChangedFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Fatalf("expected 0 files, got %d", len(files))
	}
}

func TestChangedFiles_Modified(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "hello.txt", "hello\n")
	run(t, dir, "git", "add", ".")
	run(t, dir, "git", "commit", "-m", "init")

	writeFile(t, dir, "hello.txt", "hello world\n")

	files, err := ChangedFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}

	f := files[0]
	if f.Status != "M" {
		t.Fatalf("expected status M, got %s", f.Status)
	}
	if f.Path != "hello.txt" {
		t.Fatalf("expected path hello.txt, got %s", f.Path)
	}
	if len(f.OldContent) != 1 || f.OldContent[0] != "hello" {
		t.Fatalf("unexpected old content: %v", f.OldContent)
	}
	if len(f.NewContent) != 1 || f.NewContent[0] != "hello world" {
		t.Fatalf("unexpected new content: %v", f.NewContent)
	}
	if f.Binary {
		t.Fatal("expected non-binary")
	}
}

func TestChangedFiles_NewFile(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "first.txt", "first\n")
	run(t, dir, "git", "add", ".")
	run(t, dir, "git", "commit", "-m", "init")

	writeFile(t, dir, "second.txt", "second\n")
	run(t, dir, "git", "add", "second.txt")

	// git diff --name-status compares index vs working tree.
	// Staged-only changes don't appear in unstaged diff.
	files, err := ChangedFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Fatalf("expected 0 files for staged-only change, got %d", len(files))
	}
}

func TestChangedFiles_DeletedFile(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "hello.txt", "hello\n")
	run(t, dir, "git", "add", ".")
	run(t, dir, "git", "commit", "-m", "init")

	os.Remove(filepath.Join(dir, "hello.txt"))

	files, err := ChangedFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}

	f := files[0]
	if f.Status != "D" {
		t.Fatalf("expected status D, got %s", f.Status)
	}
	if f.OldContent == nil {
		t.Fatal("expected old content for deleted file")
	}
	if len(f.OldContent) != 1 || f.OldContent[0] != "hello" {
		t.Fatalf("unexpected old content: %v", f.OldContent)
	}
	if f.NewContent != nil {
		t.Fatal("expected nil new content for deleted file")
	}
}

func TestChangedFiles_BinaryFile(t *testing.T) {
	dir := initRepo(t)
	// Create a binary file (contains null bytes)
	writeFile(t, dir, "image.bin", "header\x00data\x00end\n")
	run(t, dir, "git", "add", ".")
	run(t, dir, "git", "commit", "-m", "init")

	// Modify the binary file
	writeFile(t, dir, "image.bin", "header\x00modified\x00end\n")

	files, err := ChangedFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}

	f := files[0]
	if !f.Binary {
		t.Fatal("expected binary file")
	}
	if f.OldContent != nil {
		t.Fatal("expected nil old content for binary")
	}
	if f.NewContent != nil {
		t.Fatal("expected nil new content for binary")
	}
}

func TestChangedFiles_RenamedFile(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "old.txt", "content\n")
	run(t, dir, "git", "add", ".")
	run(t, dir, "git", "commit", "-m", "init")

	// git mv stages the rename in the index.
	// git diff --name-status shows R for index vs working tree
	// only if the working tree file differs from the staged version.
	// Use git diff --cached to see staged renames.
	// Since ChangedFiles uses unstaged diff, we simulate
	// a rename that appears in unstaged diff by manipulating
	// the index directly.
	run(t, dir, "git", "mv", "old.txt", "new.txt")

	// Modify the renamed file so it shows in unstaged diff too.
	writeFile(t, dir, "new.txt", "modified content\n")

	files, err := ChangedFiles(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Unstaged diff after git mv + modify shows M (not R),
	// because git diff compares index vs working tree and
	// the file is already tracked as new.txt in the index.
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	f := files[0]
	if f.Status != "M" {
		t.Fatalf("expected status M, got %s", f.Status)
	}
	if f.Path != "new.txt" {
		t.Fatalf("expected path new.txt, got %s", f.Path)
	}
}

func TestChangedFiles_EmptyRepo(t *testing.T) {
	dir := initRepo(t)

	files, err := ChangedFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Fatalf("expected 0 files, got %d", len(files))
	}
}

func TestDiffUncommitted_StagedAndUnstaged(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "a.txt", "line1\n")
	run(t, dir, "git", "add", ".")
	run(t, dir, "git", "commit", "-m", "init")

	// Staged change
	writeFile(t, dir, "a.txt", "line1\nline2\n")
	run(t, dir, "git", "add", "a.txt")

	// Unstaged change on top
	writeFile(t, dir, "a.txt", "line1\nline2\nline3\n")

	files, err := ChangedFilesWithOptions(dir, DiffOptions{Mode: DiffUncommitted})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	f := files[0]
	if f.Status != "M" {
		t.Fatalf("expected status M, got %s", f.Status)
	}
	// Old should be HEAD content ("line1"), new should be working tree ("line1\nline2\nline3")
	if len(f.OldContent) != 1 || f.OldContent[0] != "line1" {
		t.Fatalf("unexpected old content: %v", f.OldContent)
	}
	if len(f.NewContent) != 3 {
		t.Fatalf("expected 3 lines in new, got %d", len(f.NewContent))
	}
}

func TestDiffStaged(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "a.txt", "original\n")
	run(t, dir, "git", "add", ".")
	run(t, dir, "git", "commit", "-m", "init")

	writeFile(t, dir, "a.txt", "modified\n")
	run(t, dir, "git", "add", "a.txt")

	files, err := ChangedFilesWithOptions(dir, DiffOptions{Mode: DiffStaged})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	f := files[0]
	if f.Status != "M" {
		t.Fatalf("expected status M, got %s", f.Status)
	}
	if len(f.OldContent) != 1 || f.OldContent[0] != "original" {
		t.Fatalf("unexpected old (HEAD) content: %v", f.OldContent)
	}
	if len(f.NewContent) != 1 || f.NewContent[0] != "modified" {
		t.Fatalf("unexpected new (index) content: %v", f.NewContent)
	}
}

func TestDiffBranch(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "a.txt", "base\n")
	run(t, dir, "git", "add", ".")
	run(t, dir, "git", "commit", "-m", "init")
	run(t, dir, "git", "branch", "main")

	run(t, dir, "git", "checkout", "-b", "feature")
	writeFile(t, dir, "a.txt", "feature\n")
	run(t, dir, "git", "add", ".")
	run(t, dir, "git", "commit", "-m", "feature change")

	base, err := MergeBase(dir, "main")
	if err != nil {
		t.Fatal(err)
	}

	files, err := ChangedFilesWithOptions(dir, DiffOptions{
		Mode:    DiffBranch,
		BaseRef: base,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	f := files[0]
	if f.OldContent[0] != "base" {
		t.Fatalf("expected old=base, got %v", f.OldContent)
	}
	if f.NewContent[0] != "feature" {
		t.Fatalf("expected new=feature, got %v", f.NewContent)
	}
}

func TestUntrackedFiles(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "tracked.txt", "tracked\n")
	run(t, dir, "git", "add", ".")
	run(t, dir, "git", "commit", "-m", "init")

	writeFile(t, dir, "untracked.txt", "new\n")

	paths, err := UntrackedFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 {
		t.Fatalf("expected 1 untracked, got %d", len(paths))
	}
	if paths[0] != "untracked.txt" {
		t.Fatalf("expected untracked.txt, got %s", paths[0])
	}
}

func TestUntrackedFiles_IncludedInUncommitted(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "tracked.txt", "tracked\n")
	run(t, dir, "git", "add", ".")
	run(t, dir, "git", "commit", "-m", "init")

	writeFile(t, dir, "newfile.txt", "new content\n")

	files, err := ChangedFilesWithOptions(dir, DiffOptions{Mode: DiffUncommitted})
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, f := range files {
		if f.Path == "newfile.txt" && f.Status == "?" {
			found = true
			if f.NewContent == nil || f.NewContent[0] != "new content" {
				t.Fatalf("unexpected content for untracked: %v", f.NewContent)
			}
		}
	}
	if !found {
		t.Fatal("expected untracked file in uncommitted diff")
	}
}

func TestDefaultBranch(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "a.txt", "a\n")
	run(t, dir, "git", "add", ".")
	run(t, dir, "git", "commit", "-m", "init")
	run(t, dir, "git", "branch", "-m", "main")

	branch, err := DefaultBranch(dir)
	if err != nil {
		t.Fatal(err)
	}
	if branch != "main" {
		t.Fatalf("expected main, got %s", branch)
	}
}

func TestMergeBase(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "a.txt", "a\n")
	run(t, dir, "git", "add", ".")
	run(t, dir, "git", "commit", "-m", "init")
	run(t, dir, "git", "branch", "main")

	run(t, dir, "git", "checkout", "-b", "feature")
	writeFile(t, dir, "b.txt", "b\n")
	run(t, dir, "git", "add", ".")
	run(t, dir, "git", "commit", "-m", "feature")

	base, err := MergeBase(dir, "main")
	if err != nil {
		t.Fatal(err)
	}
	if base == "" {
		t.Fatal("expected non-empty merge-base")
	}
}

func TestDiffUncommitted_EmptyRepo(t *testing.T) {
	dir := initRepo(t)

	// No commits, DiffUncommitted should return nil (not error)
	files, err := ChangedFilesWithOptions(dir, DiffOptions{Mode: DiffUncommitted})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Fatalf("expected nil or empty files, got %d", len(files))
	}
}

func TestDiffBranch_MissingBaseRef(t *testing.T) {
	dir := initRepo(t)

	_, err := ChangedFilesWithOptions(dir, DiffOptions{Mode: DiffBranch})
	if err == nil {
		t.Fatal("expected error for missing BaseRef")
	}
}
