package tui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanDir_SymlinkToDirectory(t *testing.T) {
	tmp := t.TempDir()

	// Create a real directory with a file inside.
	realDir := filepath.Join(tmp, "realdir")
	if err := os.Mkdir(realDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(realDir, "hello.txt"), []byte("hi"), 0o644,
	); err != nil {
		t.Fatal(err)
	}

	// Create a symlink pointing to the real directory.
	symDir := filepath.Join(tmp, "linkdir")
	if err := os.Symlink(realDir, symDir); err != nil {
		t.Fatal(err)
	}

	entries := scanDir(tmp, 0, nil)

	// Both realdir and linkdir should appear as directories.
	dirCount := 0
	for _, e := range entries {
		if !e.isDir {
			t.Errorf("entry %q should be a directory", e.name)
		}
		dirCount++
	}
	if dirCount != 2 {
		t.Errorf("expected 2 directory entries, got %d", dirCount)
	}

	// Expanding the symlink directory should show its contents.
	var symlinkIdx int
	for i, e := range entries {
		if e.name == "linkdir" {
			symlinkIdx = i
			break
		}
	}

	expanded := expandDir(entries, symlinkIdx)
	found := false
	for _, e := range expanded {
		if e.name == "hello.txt" {
			found = true
			break
		}
	}
	if !found {
		t.Error(
			"expanding symlink directory should show hello.txt",
		)
	}
}

func TestScanDir_SymlinkToFile(t *testing.T) {
	tmp := t.TempDir()

	// Create a real file.
	realFile := filepath.Join(tmp, "real.txt")
	if err := os.WriteFile(
		realFile, []byte("content"), 0o644,
	); err != nil {
		t.Fatal(err)
	}

	// Create a symlink pointing to the file.
	symFile := filepath.Join(tmp, "link.txt")
	if err := os.Symlink(realFile, symFile); err != nil {
		t.Fatal(err)
	}

	entries := scanDir(tmp, 0, nil)

	// Both should appear as files.
	fileCount := 0
	for _, e := range entries {
		if e.isDir {
			t.Errorf("entry %q should be a file", e.name)
		}
		fileCount++
	}
	if fileCount != 2 {
		t.Errorf("expected 2 file entries, got %d", fileCount)
	}
}

func TestScanDir_BrokenSymlink(t *testing.T) {
	tmp := t.TempDir()

	// Create a symlink pointing to a non-existent target.
	brokenLink := filepath.Join(tmp, "broken")
	if err := os.Symlink("/nonexistent/path", brokenLink); err != nil {
		t.Fatal(err)
	}

	// Create a regular file for comparison.
	if err := os.WriteFile(
		filepath.Join(tmp, "valid.txt"), []byte("ok"), 0o644,
	); err != nil {
		t.Fatal(err)
	}

	entries := scanDir(tmp, 0, nil)

	// Broken symlink should be skipped; only valid.txt remains.
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d: %+v", len(entries), entries)
	}
	if entries[0].name != "valid.txt" {
		t.Errorf(
			"expected valid.txt, got %q", entries[0].name,
		)
	}
}

func TestBuildFileTree_SymlinkLoop(t *testing.T) {
	tmp := t.TempDir()

	// Create a symlink loop: a -> b, b -> a
	a := filepath.Join(tmp, "a")
	b := filepath.Join(tmp, "b")
	if err := os.Symlink(b, a); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(a, b); err != nil {
		t.Fatal(err)
	}

	// This should not hang or crash.
	entries := buildFileTree(tmp)

	// Both symlinks should be skipped (broken: loop).
	if len(entries) != 0 {
		t.Errorf("expected 0 entries for symlink loop, got %d", len(entries))
	}
}
