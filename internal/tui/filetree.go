package tui

import (
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/fsnotify/fsnotify"
)

// fileEntry represents a single entry in the file tree.
type fileEntry struct {
	path     string
	name     string
	isDir    bool
	depth    int
	expanded bool
}

// excludeDirs lists directories to exclude from the tree.
var excludeDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	".vscode":      true,
	".idea":        true,
	"vendor":       true,
	"__pycache__":  true,
}

// WatchDirRecursive recursively adds directories to the watcher.
func WatchDirRecursive(watcher *fsnotify.Watcher, dir string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			return nil
		}
		name := info.Name()
		if excludeDirs[name] {
			return filepath.SkipDir
		}
		if strings.HasPrefix(name, ".") && name != ".claude" && path != dir {
			return filepath.SkipDir
		}
		if err := watcher.Add(path); err != nil {
			log.Printf("Failed to watch dir %s: %v", path, err)
		}
		return nil
	})
}

// buildFileTree scans rootDir recursively and returns a flat list of entries.
func buildFileTree(rootDir string) []fileEntry {
	var entries []fileEntry
	entries = scanDir(rootDir, rootDir, 0, entries)
	return entries
}

// scanDir recursively scans a directory.
func scanDir(rootDir, dir string, depth int, entries []fileEntry) []fileEntry {
	files, err := os.ReadDir(dir)
	if err != nil {
		return entries
	}

	var dirs, regularFiles []os.DirEntry
	for _, f := range files {
		name := f.Name()
		if strings.HasPrefix(name, ".") && name != ".claude" {
			continue
		}
		if f.IsDir() {
			if !excludeDirs[name] {
				dirs = append(dirs, f)
			}
		} else {
			regularFiles = append(regularFiles, f)
		}
	}

	sort.Slice(dirs, func(i, j int) bool {
		return dirs[i].Name() < dirs[j].Name()
	})
	sort.Slice(regularFiles, func(i, j int) bool {
		return regularFiles[i].Name() < regularFiles[j].Name()
	})

	for _, d := range dirs {
		fullPath := filepath.Join(dir, d.Name())
		entries = append(entries, fileEntry{
			path:     fullPath,
			name:     d.Name(),
			isDir:    true,
			depth:    depth,
			expanded: false,
		})
	}

	for _, f := range regularFiles {
		fullPath := filepath.Join(dir, f.Name())
		entries = append(entries, fileEntry{
			path:  fullPath,
			name:  f.Name(),
			isDir: false,
			depth: depth,
		})
	}

	return entries
}

// expandDir expands a directory entry and inserts its children.
func expandDir(entries []fileEntry, index int) []fileEntry {
	if index < 0 || index >= len(entries) || !entries[index].isDir {
		return entries
	}

	entry := &entries[index]
	entry.expanded = true

	var children []fileEntry
	children = scanDir(entry.path, entry.path, entry.depth+1, children)

	result := make([]fileEntry, 0, len(entries)+len(children))
	result = append(result, entries[:index+1]...)
	result = append(result, children...)
	result = append(result, entries[index+1:]...)

	return result
}

// collapseDir collapses a directory entry and removes its children.
func collapseDir(entries []fileEntry, index int) []fileEntry {
	if index < 0 || index >= len(entries) || !entries[index].isDir {
		return entries
	}

	entry := &entries[index]
	entry.expanded = false
	parentDepth := entry.depth

	endIndex := index + 1
	for endIndex < len(entries) && entries[endIndex].depth > parentDepth {
		endIndex++
	}

	result := make([]fileEntry, 0, len(entries)-(endIndex-index-1))
	result = append(result, entries[:index+1]...)
	result = append(result, entries[endIndex:]...)

	return result
}
