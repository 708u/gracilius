package tui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanAllFiles(t *testing.T) {
	dir := t.TempDir()

	// Create directory structure:
	//   dir/
	//     a.go
	//     sub/
	//       b.txt
	os.MkdirAll(filepath.Join(dir, "sub"), 0o755)
	os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a"), 0o644)
	os.WriteFile(filepath.Join(dir, "sub", "b.txt"), []byte("hello"), 0o644)

	items := scanAllFiles(dir)

	paths := make(map[string]bool)
	for _, fi := range items {
		paths[fi.path] = true
	}

	if !paths["a.go"] {
		t.Error("expected a.go in results")
	}
	expected := filepath.Join("sub", "b.txt")
	if !paths[expected] {
		t.Errorf("expected %s in results", expected)
	}
	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}
}

func TestScanAllFiles_ExcludesHidden(t *testing.T) {
	dir := t.TempDir()

	// Create directory structure:
	//   dir/
	//     visible.go
	//     .git/
	//       config
	//     node_modules/
	//       pkg.js
	//     .hidden_file
	os.MkdirAll(filepath.Join(dir, ".git"), 0o755)
	os.MkdirAll(filepath.Join(dir, "node_modules"), 0o755)
	os.WriteFile(filepath.Join(dir, "visible.go"), []byte("package v"), 0o644)
	os.WriteFile(filepath.Join(dir, ".git", "config"), []byte("[core]"), 0o644)
	os.WriteFile(filepath.Join(dir, "node_modules", "pkg.js"), []byte("module.exports"), 0o644)
	os.WriteFile(filepath.Join(dir, ".hidden_file"), []byte("secret"), 0o644)

	items := scanAllFiles(dir)

	paths := make(map[string]bool)
	for _, fi := range items {
		paths[fi.path] = true
	}

	if !paths["visible.go"] {
		t.Error("expected visible.go in results")
	}
	if paths[filepath.Join(".git", "config")] {
		t.Error(".git/config should be excluded")
	}
	if paths[filepath.Join("node_modules", "pkg.js")] {
		t.Error("node_modules/pkg.js should be excluded")
	}
	if paths[".hidden_file"] {
		t.Error(".hidden_file should be excluded")
	}
	if len(items) != 1 {
		t.Errorf("expected 1 item, got %d", len(items))
	}
}
