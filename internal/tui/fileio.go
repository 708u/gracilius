package tui

import (
	"bufio"
	"bytes"
	"log"
	"os"
	"path/filepath"
)

// isBinary returns true if the content appears to be binary.
func isBinary(content []byte) bool {
	checkSize := min(len(content), 8192)
	for i := range checkSize {
		if content[i] == 0 {
			return true
		}
	}
	return false
}

// sniffBinary reads the first bytes of a file to detect binary content.
func sniffBinary(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()

	buf := make([]byte, 8192)
	n, _ := f.Read(buf)
	return isBinary(buf[:n])
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

	if isBinary(content) {
		t.lines = []string{"(Binary file)"}
		return nil
	}

	if m.watcher != nil {
		if err := m.watcher.Add(absPath); err != nil {
			log.Printf("Failed to watch file: %v", err)
		}
	}

	t.lines = splitLines(content)
	t.syncContent(t.lines)
	t.highlightedLines = highlightFile(absPath, string(content), m.theme)

	// Load persisted comments for this file.
	stored, err := m.commentStore.List(absPath, false)
	if err != nil {
		log.Printf("Failed to load comments for %s: %v", absPath, err)
	}
	for _, sc := range stored {
		t.comments = append(t.comments, comment{
			id:        sc.ID,
			startLine: sc.StartLine,
			endLine:   sc.EndLine,
			text:      sc.Text,
		})
	}

	return nil
}

// splitLines splits content into lines.
// Uses bufio.Scanner to handle \n, \r\n, and \r transparently.
func splitLines(content []byte) []string {
	scanner := bufio.NewScanner(bytes.NewReader(content))
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines
}
