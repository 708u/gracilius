package tui

import (
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/fsnotify/fsnotify"
)

// fileEntry represents a single entry in the file tree.
type fileEntry struct {
	path         string
	name         string
	isDir        bool
	isBinary     bool
	depth        int
	expanded     bool
	resolvedPath string // symlink target path (empty if not a symlink)
}

// TODO: make configurable (e.g. .gitignore or config file)
var excludeDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	".vscode":      true,
	".idea":        true,
	"vendor":       true,
	"__pycache__":  true,
}

// isHiddenEntry returns true if the named entry should be excluded
// based on naming conventions (dot-prefix or excludeDirs).
func isHiddenEntry(name string) bool {
	if excludeDirs[name] {
		return true
	}
	// TODO: make configurable instead of hardcoding ".claude"
	return strings.HasPrefix(name, ".") && name != ".claude"
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
		if isHiddenEntry(info.Name()) && path != dir {
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
	entries = scanDir(rootDir, 0, entries)
	return entries
}

// scanDir recursively scans a directory.
func scanDir(dir string, depth int, entries []fileEntry) []fileEntry {
	files, err := os.ReadDir(dir)
	if err != nil {
		return entries
	}

	type dirEntryInfo struct {
		entry        os.DirEntry
		resolvedPath string // non-empty for symlinks
	}
	var dirs, regularFiles []dirEntryInfo
	for _, f := range files {
		if isHiddenEntry(f.Name()) {
			continue
		}
		isDir := f.IsDir()
		var resolvedPath string
		// Resolve symlinks: DirEntry.IsDir() returns false for
		// symlinks, so we need to follow the link to determine
		// if the target is a directory.
		if f.Type()&os.ModeSymlink != 0 {
			fullPath := filepath.Join(dir, f.Name())
			resolved, err := filepath.EvalSymlinks(fullPath)
			if err != nil {
				// Broken symlink (target does not exist): skip
				continue
			}
			target, err := os.Stat(resolved)
			if err != nil {
				continue
			}
			isDir = target.IsDir()
			resolvedPath = resolved
		}
		info := dirEntryInfo{entry: f, resolvedPath: resolvedPath}
		if isDir {
			dirs = append(dirs, info)
		} else {
			regularFiles = append(regularFiles, info)
		}
	}

	slices.SortFunc(dirs, func(a, b dirEntryInfo) int {
		return strings.Compare(a.entry.Name(), b.entry.Name())
	})
	slices.SortFunc(regularFiles, func(a, b dirEntryInfo) int {
		return strings.Compare(a.entry.Name(), b.entry.Name())
	})

	for _, d := range dirs {
		fullPath := filepath.Join(dir, d.entry.Name())
		entries = append(entries, fileEntry{
			path:         fullPath,
			name:         d.entry.Name(),
			isDir:        true,
			depth:        depth,
			expanded:     false,
			resolvedPath: d.resolvedPath,
		})
	}

	for _, f := range regularFiles {
		fullPath := filepath.Join(dir, f.entry.Name())
		entries = append(entries, fileEntry{
			path:         fullPath,
			name:         f.entry.Name(),
			isDir:        false,
			isBinary:     sniffBinary(fullPath),
			depth:        depth,
			resolvedPath: f.resolvedPath,
		})
	}

	return entries
}

// expandedPaths returns the set of currently expanded directory paths.
func expandedPaths(entries []fileEntry) map[string]bool {
	paths := make(map[string]bool)
	for _, e := range entries {
		if e.isDir && e.expanded {
			paths[e.path] = true
		}
	}
	return paths
}

// restoreExpanded expands directories whose paths are in the given set.
func restoreExpanded(
	entries []fileEntry, paths map[string]bool,
) []fileEntry {
	for i := 0; i < len(entries); i++ {
		if entries[i].isDir && paths[entries[i].path] {
			entries = expandDir(entries, i)
		}
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
	children = scanDir(entry.path, entry.depth+1, children)

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
