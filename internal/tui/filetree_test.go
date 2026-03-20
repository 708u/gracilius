package tui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanDir_SymlinkToDirectory(t *testing.T) {
	t.Parallel()
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

	entries := scanDir(tmp, 0, nil, nil)

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

	expanded := expandDir(entries, symlinkIdx, nil)
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
	t.Parallel()
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

	entries := scanDir(tmp, 0, nil, nil)

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
	t.Parallel()
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

	entries := scanDir(tmp, 0, nil, nil)

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
	t.Parallel()
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
	entries := buildFileTree(tmp, nil)

	// Both symlinks should be skipped (broken: loop).
	if len(entries) != 0 {
		t.Errorf("expected 0 entries for symlink loop, got %d", len(entries))
	}
}

// mkdirs creates nested directories under root.
func mkdirs(t *testing.T, paths ...string) {
	t.Helper()
	for _, p := range paths {
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

// touch creates an empty file at path.
func touch(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCompactSingleChildDirs(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()

	// a/b/c with a file in c
	mkdirs(t, filepath.Join(tmp, "a", "b", "c"))
	touch(t, filepath.Join(tmp, "a", "b", "c", "file.txt"))

	name, leaf, intermediates := compactSingleChildDirs(
		filepath.Join(tmp, "a"), "a", nil)

	if name != "a/b/c" {
		t.Errorf("want name %q, got %q", "a/b/c", name)
	}
	if leaf != filepath.Join(tmp, "a", "b", "c") {
		t.Errorf("want leaf %q, got %q",
			filepath.Join(tmp, "a", "b", "c"), leaf)
	}
	if len(intermediates) != 2 {
		t.Fatalf("want 2 intermediates, got %d", len(intermediates))
	}
	if intermediates[0] != filepath.Join(tmp, "a") {
		t.Errorf("intermediates[0]: want %q, got %q",
			filepath.Join(tmp, "a"), intermediates[0])
	}
	if intermediates[1] != filepath.Join(tmp, "a", "b") {
		t.Errorf("intermediates[1]: want %q, got %q",
			filepath.Join(tmp, "a", "b"), intermediates[1])
	}
}

func TestCompactSingleChildDirs_StopsAtFiles(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()

	// a/b where b has a file sibling in a
	mkdirs(t, filepath.Join(tmp, "a", "b"))
	touch(t, filepath.Join(tmp, "a", "readme.txt"))

	name, leaf, intermediates := compactSingleChildDirs(
		filepath.Join(tmp, "a"), "a", nil)

	if name != "a" {
		t.Errorf("want name %q, got %q", "a", name)
	}
	if leaf != filepath.Join(tmp, "a") {
		t.Errorf("want leaf %q, got %q",
			filepath.Join(tmp, "a"), leaf)
	}
	if len(intermediates) != 0 {
		t.Errorf("want 0 intermediates, got %d", len(intermediates))
	}
}

func TestCompactSingleChildDirs_StopsAtMultipleDirs(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()

	// a contains two subdirs
	mkdirs(t, filepath.Join(tmp, "a", "x"))
	mkdirs(t, filepath.Join(tmp, "a", "y"))

	name, leaf, intermediates := compactSingleChildDirs(
		filepath.Join(tmp, "a"), "a", nil)

	if name != "a" {
		t.Errorf("want name %q, got %q", "a", name)
	}
	if leaf != filepath.Join(tmp, "a") {
		t.Errorf("want leaf %q, got %q",
			filepath.Join(tmp, "a"), leaf)
	}
	if len(intermediates) != 0 {
		t.Errorf("want 0 intermediates, got %d", len(intermediates))
	}
}

func TestCompactSingleChildDirs_NoCompaction(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()

	// a has a file and a dir
	mkdirs(t, filepath.Join(tmp, "a", "sub"))
	touch(t, filepath.Join(tmp, "a", "file.txt"))

	name, leaf, intermediates := compactSingleChildDirs(
		filepath.Join(tmp, "a"), "a", nil)

	if name != "a" {
		t.Errorf("want name %q, got %q", "a", name)
	}
	if leaf != filepath.Join(tmp, "a") {
		t.Errorf("want leaf unchanged")
	}
	if len(intermediates) != 0 {
		t.Errorf("want 0 intermediates, got %d", len(intermediates))
	}
}

func TestCompactSingleChildDirs_EmptyDir(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()

	mkdirs(t, filepath.Join(tmp, "empty"))

	name, leaf, intermediates := compactSingleChildDirs(
		filepath.Join(tmp, "empty"), "empty", nil)

	if name != "empty" {
		t.Errorf("want name %q, got %q", "empty", name)
	}
	if leaf != filepath.Join(tmp, "empty") {
		t.Errorf("want leaf unchanged")
	}
	if len(intermediates) != 0 {
		t.Errorf("want 0 intermediates, got %d", len(intermediates))
	}
}

func TestScanDir_CompactedEntry(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()

	// a/b/c/file.txt — should compact to "a/b/c"
	mkdirs(t, filepath.Join(tmp, "a", "b", "c"))
	touch(t, filepath.Join(tmp, "a", "b", "c", "file.txt"))

	entries := scanDir(tmp, 0, nil, nil)

	if len(entries) != 1 {
		t.Fatalf("want 1 entry, got %d: %+v", len(entries), entries)
	}
	e := entries[0]
	if e.name != "a/b/c" {
		t.Errorf("want name %q, got %q", "a/b/c", e.name)
	}
	if e.path != filepath.Join(tmp, "a", "b", "c") {
		t.Errorf("want path %q, got %q",
			filepath.Join(tmp, "a", "b", "c"), e.path)
	}
	if !e.isDir {
		t.Error("want isDir=true")
	}
	if len(e.compactedPaths) != 2 {
		t.Errorf("want 2 compactedPaths, got %d", len(e.compactedPaths))
	}
}

func TestExpandDir_CompactedEntry(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()

	// a/b/c/file.txt
	mkdirs(t, filepath.Join(tmp, "a", "b", "c"))
	touch(t, filepath.Join(tmp, "a", "b", "c", "file.txt"))

	entries := scanDir(tmp, 0, nil, nil)
	entries = expandDir(entries, 0, nil)

	// Should show: a/b/c (expanded), file.txt
	if len(entries) != 2 {
		t.Fatalf("want 2 entries after expand, got %d: %+v",
			len(entries), entries)
	}
	if entries[0].name != "a/b/c" || !entries[0].expanded {
		t.Errorf("first entry: want expanded a/b/c, got %q expanded=%v",
			entries[0].name, entries[0].expanded)
	}
	if entries[1].name != "file.txt" {
		t.Errorf("second entry: want file.txt, got %q", entries[1].name)
	}
}

func TestCollapseDir_CompactedEntry(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()

	// a/b/c/file.txt
	mkdirs(t, filepath.Join(tmp, "a", "b", "c"))
	touch(t, filepath.Join(tmp, "a", "b", "c", "file.txt"))

	entries := scanDir(tmp, 0, nil, nil)
	entries = expandDir(entries, 0, nil)
	entries = collapseDir(entries, 0)

	if len(entries) != 1 {
		t.Fatalf("want 1 entry after collapse, got %d", len(entries))
	}
	if entries[0].expanded {
		t.Error("entry should not be expanded after collapse")
	}
	if entries[0].name != "a/b/c" {
		t.Errorf("want name %q, got %q", "a/b/c", entries[0].name)
	}
}

func TestRestoreExpanded_ChainBreaks(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()

	// Start with a/b/c (compacted, expanded)
	mkdirs(t, filepath.Join(tmp, "a", "b", "c"))
	touch(t, filepath.Join(tmp, "a", "b", "c", "file.txt"))

	entries := scanDir(tmp, 0, nil, nil)
	entries = expandDir(entries, 0, nil)
	paths := expandedPaths(entries)

	// Break the chain: add a file to a/ so a won't compact anymore
	touch(t, filepath.Join(tmp, "a", "extra.txt"))

	// Rebuild and restore
	var newEntries []fileEntry
	newEntries = scanDir(tmp, 0, newEntries, nil)
	newEntries = restoreExpanded(newEntries, paths, nil)

	// "a" should now be expanded (it was in compactedPaths)
	if len(newEntries) == 0 {
		t.Fatal("expected entries after rebuild")
	}
	if newEntries[0].name != "a" {
		t.Errorf("want name %q, got %q", "a", newEntries[0].name)
	}
	if !newEntries[0].expanded {
		t.Error("a should be restored to expanded state")
	}
}

func TestRestoreExpanded_ChainForms(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()

	// Start with a (not compacted because b has a sibling file)
	mkdirs(t, filepath.Join(tmp, "a", "b", "c"))
	touch(t, filepath.Join(tmp, "a", "extra.txt"))
	touch(t, filepath.Join(tmp, "a", "b", "c", "file.txt"))

	entries := scanDir(tmp, 0, nil, nil)
	// Expand "a" (not compacted)
	entries = expandDir(entries, 0, nil)
	// Expand "b/c" (compacted inside a)
	for i, e := range entries {
		if e.name == "b/c" {
			entries = expandDir(entries, i, nil)
			break
		}
	}
	paths := expandedPaths(entries)

	// Now remove extra.txt — a should compact into a/b/c
	if err := os.Remove(filepath.Join(tmp, "a", "extra.txt")); err != nil {
		t.Fatal(err)
	}

	// Rebuild and restore
	var newEntries []fileEntry
	newEntries = scanDir(tmp, 0, newEntries, nil)
	newEntries = restoreExpanded(newEntries, paths, nil)

	// "a/b/c" should be expanded (restored via intermediate path match)
	if len(newEntries) == 0 {
		t.Fatal("expected entries after rebuild")
	}
	if newEntries[0].name != "a/b/c" {
		t.Errorf("want name %q, got %q", "a/b/c", newEntries[0].name)
	}
	if !newEntries[0].expanded {
		t.Error("a/b/c should be restored to expanded state")
	}
}
