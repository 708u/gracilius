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
