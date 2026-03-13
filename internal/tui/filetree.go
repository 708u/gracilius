package tui

import (
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/fsnotify/fsnotify"
)

// ExcludeFunc determines which paths should be excluded from the file tree.
// It receives a list of absolute paths (directories end with "/")
// and returns the set of paths that should be excluded.
type ExcludeFunc func(paths []string) map[string]bool

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
// When exclude is non-nil it is used instead of isHiddenEntry.
// Subdirectory exclusion is batched per level to minimize subprocess calls.
func WatchDirRecursive(watcher *fsnotify.Watcher, dir string, exclude ExcludeFunc) error {
	if err := watcher.Add(dir); err != nil {
		log.Printf("Failed to watch dir %s: %v", dir, err)
	}
	return watchDirLevel(watcher, dir, exclude)
}

// watchDirLevel reads one directory level, batch-checks exclusions,
// adds non-excluded subdirectories to the watcher, and recurses.
func watchDirLevel(watcher *fsnotify.Watcher, dir string, exclude ExcludeFunc) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	// Collect subdirectory paths (skip .git unconditionally).
	var subdirs []string
	for _, e := range entries {
		if e.Name() == ".git" {
			continue
		}
		isDir := e.IsDir()
		if e.Type()&os.ModeSymlink != 0 {
			fullPath := filepath.Join(dir, e.Name())
			resolved, err := filepath.EvalSymlinks(fullPath)
			if err != nil {
				continue
			}
			target, err := os.Stat(resolved)
			if err != nil || !target.IsDir() {
				continue
			}
			isDir = true
		}
		if isDir {
			subdirs = append(subdirs, filepath.Join(dir, e.Name()))
		}
	}

	// Batch-filter excluded directories.
	if exclude != nil && len(subdirs) > 0 {
		paths := make([]string, len(subdirs))
		for i, s := range subdirs {
			paths[i] = s + "/"
		}
		ignored := exclude(paths)
		filtered := subdirs[:0]
		for _, s := range subdirs {
			if !ignored[s+"/"] {
				filtered = append(filtered, s)
			}
		}
		subdirs = filtered
	} else if exclude == nil {
		filtered := subdirs[:0]
		for _, s := range subdirs {
			if !isHiddenEntry(filepath.Base(s)) {
				filtered = append(filtered, s)
			}
		}
		subdirs = filtered
	}

	// Add to watcher and recurse.
	for _, s := range subdirs {
		if err := watcher.Add(s); err != nil {
			log.Printf("Failed to watch dir %s: %v", s, err)
		}
		_ = watchDirLevel(watcher, s, exclude)
	}
	return nil
}

// buildFileTree scans rootDir recursively and returns a flat list of entries.
func buildFileTree(rootDir string, exclude ExcludeFunc) []fileEntry {
	var entries []fileEntry
	entries = scanDir(rootDir, 0, entries, exclude)
	return entries
}

// scanDir recursively scans a directory.
func scanDir(dir string, depth int, entries []fileEntry, exclude ExcludeFunc) []fileEntry {
	files, err := os.ReadDir(dir)
	if err != nil {
		return entries
	}

	type dirEntryInfo struct {
		entry        os.DirEntry
		fullPath     string
		resolvedPath string // non-empty for symlinks
		isDir        bool
	}

	// First pass: collect all entries, resolve symlinks, compute fullPath.
	var allEntries []dirEntryInfo
	for _, f := range files {
		// .git is always excluded
		if f.Name() == ".git" {
			continue
		}
		fullPath := filepath.Join(dir, f.Name())
		isDir := f.IsDir()
		var resolvedPath string
		if f.Type()&os.ModeSymlink != 0 {
			resolved, err := filepath.EvalSymlinks(fullPath)
			if err != nil {
				continue
			}
			target, err := os.Stat(resolved)
			if err != nil {
				continue
			}
			isDir = target.IsDir()
			resolvedPath = resolved
		}
		allEntries = append(allEntries, dirEntryInfo{
			entry: f, fullPath: fullPath, resolvedPath: resolvedPath, isDir: isDir,
		})
	}

	// Determine which entries to exclude.
	var ignored map[string]bool
	if exclude != nil {
		paths := make([]string, len(allEntries))
		for i, e := range allEntries {
			if e.isDir {
				paths[i] = e.fullPath + "/"
			} else {
				paths[i] = e.fullPath
			}
		}
		ignored = exclude(paths)
	}

	// Second pass: partition into dirs and files, skipping excluded.
	var dirs, regularFiles []dirEntryInfo
	for i := range allEntries {
		e := &allEntries[i]
		if exclude != nil {
			key := e.fullPath
			if e.isDir {
				key += "/"
			}
			if ignored[key] {
				continue
			}
		} else if isHiddenEntry(e.entry.Name()) {
			continue
		}

		if e.isDir {
			dirs = append(dirs, *e)
		} else {
			regularFiles = append(regularFiles, *e)
		}
	}

	slices.SortFunc(dirs, func(a, b dirEntryInfo) int {
		return strings.Compare(a.entry.Name(), b.entry.Name())
	})
	slices.SortFunc(regularFiles, func(a, b dirEntryInfo) int {
		return strings.Compare(a.entry.Name(), b.entry.Name())
	})

	for _, d := range dirs {
		entries = append(entries, fileEntry{
			path:         d.fullPath,
			name:         d.entry.Name(),
			isDir:        true,
			depth:        depth,
			expanded:     false,
			resolvedPath: d.resolvedPath,
		})
	}

	for _, f := range regularFiles {
		entries = append(entries, fileEntry{
			path:         f.fullPath,
			name:         f.entry.Name(),
			isDir:        false,
			isBinary:     sniffBinary(f.fullPath),
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
	entries []fileEntry, paths map[string]bool, exclude ExcludeFunc,
) []fileEntry {
	for i := 0; i < len(entries); i++ {
		if entries[i].isDir && paths[entries[i].path] {
			entries = expandDir(entries, i, exclude)
		}
	}
	return entries
}

// expandDir expands a directory entry and inserts its children.
func expandDir(entries []fileEntry, index int, exclude ExcludeFunc) []fileEntry {
	if index < 0 || index >= len(entries) || !entries[index].isDir {
		return entries
	}

	entry := &entries[index]
	entry.expanded = true

	var children []fileEntry
	children = scanDir(entry.path, entry.depth+1, children, exclude)

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
