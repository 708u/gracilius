package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/708u/gracilius/internal/comment"
)

// mockCommentRepository is a no-op CommentRepository for tests.
type mockCommentRepository struct {
	comments []comment.Entry
}

func (s *mockCommentRepository) List(string, bool) ([]comment.Entry, error) { return s.comments, nil }
func (s *mockCommentRepository) Add(c comment.Entry) error {
	s.comments = append(s.comments, c)
	return nil
}
func (s *mockCommentRepository) Replace(oldID string, c comment.Entry) error {
	for i := range s.comments {
		if s.comments[i].ID == oldID {
			s.comments = append(s.comments[:i], s.comments[i+1:]...)
			break
		}
	}
	s.comments = append(s.comments, c)
	return nil
}
func (s *mockCommentRepository) Delete(id string) error {
	for i := range s.comments {
		if s.comments[i].ID == id {
			s.comments = append(s.comments[:i], s.comments[i+1:]...)
			return nil
		}
	}
	return nil
}
func (s *mockCommentRepository) DeleteByFile(string) error { s.comments = nil; return nil }
func (s *mockCommentRepository) DataPath() string          { return "" }

// newTestModel creates a minimal Model with mock server and temp directory.
func newTestModel(t *testing.T) *Model {
	t.Helper()
	tmpDir := t.TempDir()
	srv := &mockServer{port: 18765}
	m := &Model{
		server:         srv,
		commentRepo:    &mockCommentRepository{},
		rootDir:        tmpDir,
		tabs:           []*tab{},
		treeWidth:      30,
		sidebarVisible: true,
		keys:           newKeyMap(),
		iconMode:       iconSymbol,
		openFile:       newOpenFileOverlay(iconSymbol, darkTheme),
		width:          120,
		height:         40,
	}
	return m
}

// newTestModelWithFile creates a Model with a file tab loaded.
func newTestModelWithFile(t *testing.T, content string) *Model {
	t.Helper()
	m := newTestModel(t)

	filePath := filepath.Join(m.rootDir, "test.go")
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	ft := newFileTab()
	ft.filePath = filePath
	ft.lines = strings.Split(content, "\n")
	ft.highlightedLines = highlightFile(filePath, content, m.theme)

	m.tabs = append(m.tabs, ft)
	m.activeTab = 0
	m.focusPane = paneEditor
	return m
}
