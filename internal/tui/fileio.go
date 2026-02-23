package tui

import (
	"log"
	"os"
	"path/filepath"
	"strings"
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

// resetEditorState resets cursor, selection, highlight, comments, and input state.
func (m *Model) resetEditorState() {
	m.highlightedLines = nil
	m.cursorLine = 0
	m.cursorChar = 0
	m.anchorLine = 0
	m.anchorChar = 0
	m.scrollOffset = 0
	m.selecting = false
	m.comments = make(map[int]string)
	m.inputMode = false
	m.commentInput.Reset()
	m.commentInput.Blur()
}

// loadFile reads a file and updates the model state.
func (m *Model) loadFile(filePath string) error {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return err
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		return err
	}

	// Remove watch on previous file
	if m.filePath != "" && m.watcher != nil {
		if err := m.watcher.Remove(m.filePath); err != nil {
			log.Printf("Failed to remove watch: %v", err)
		}
	}

	m.filePath = absPath
	m.resetEditorState()

	if isBinary(content) {
		m.lines = []string{"(Binary file)"}
		return nil
	}

	if m.watcher != nil {
		if err := m.watcher.Add(absPath); err != nil {
			log.Printf("Failed to watch file: %v", err)
		}
	}

	m.lines = strings.Split(string(content), "\n")
	m.highlightedLines = highlightFile(absPath, string(content))

	return nil
}
