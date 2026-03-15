package tui

import (
	"log"
	"os"
	"path/filepath"

	"github.com/708u/gracilius/internal/fileutil"
)

// sniffBinary reads the first bytes of a file to detect binary content.
func sniffBinary(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()

	buf := make([]byte, 8192)
	n, _ := f.Read(buf)
	return fileutil.IsBinary(buf[:n])
}

// loadFileIntoTab reads a file and loads it into the given tab.
func (m *Model) loadFileIntoTab(t *tab, filePath string) error {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return err
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		return err
	}

	// Remove watch on previous file
	if t.filePath != "" && m.watcher != nil {
		if err := m.watcher.Remove(t.filePath); err != nil {
			log.Printf("Failed to remove watch: %v", err)
		}
	}

	t.filePath = absPath
	t.resetEditorState()

	if fileutil.IsBinary(content) {
		t.lines = []string{"(Binary file)"}
		return nil
	}

	if m.watcher != nil {
		if err := m.watcher.Add(absPath); err != nil {
			log.Printf("Failed to watch file: %v", err)
		}
	}

	t.lines = fileutil.SplitLines(content)
	t.syncContent(t.lines)
	t.highlightedLines = highlightFile(absPath, string(content), m.theme)

	// Load persisted comments for this file.
	stored, err := m.commentRepo.List(absPath, false)
	if err != nil {
		log.Printf("Failed to load comments for %s: %v", absPath, err)
	}
	t.comments = append(t.comments, stored...)

	return nil
}
