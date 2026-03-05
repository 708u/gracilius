package comment

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Entry represents a review comment attached to a file.
type Entry struct {
	ID         string
	FilePath   string
	StartLine  int
	EndLine    int
	Text       string
	Snippet    string
	ResolvedAt time.Time // zero value = unresolved
	CreatedAt  time.Time
}

// commentJSON is the JSON serialization format for Entry.
type commentJSON struct {
	ID         string `json:"id"`
	FilePath   string `json:"filePath"`
	StartLine  int    `json:"startLine"`
	EndLine    int    `json:"endLine"`
	Text       string `json:"text"`
	Snippet    string `json:"snippet"`
	ResolvedAt string `json:"resolvedAt,omitempty"`
	CreatedAt  string `json:"createdAt"`
}

// MarshalJSON implements json.Marshaler.
func (c *Entry) MarshalJSON() ([]byte, error) {
	j := commentJSON{
		ID:        c.ID,
		FilePath:  c.FilePath,
		StartLine: c.StartLine,
		EndLine:   c.EndLine,
		Text:      c.Text,
		Snippet:   c.Snippet,
		CreatedAt: c.CreatedAt.Format(time.RFC3339),
	}
	if !c.ResolvedAt.IsZero() {
		j.ResolvedAt = c.ResolvedAt.Format(time.RFC3339)
	}
	return json.Marshal(j)
}

// UnmarshalJSON implements json.Unmarshaler.
func (c *Entry) UnmarshalJSON(data []byte) error {
	var j commentJSON
	if err := json.Unmarshal(data, &j); err != nil {
		return err
	}
	c.ID = j.ID
	c.FilePath = j.FilePath
	c.StartLine = j.StartLine
	c.EndLine = j.EndLine
	c.Text = j.Text
	c.Snippet = j.Snippet
	if j.CreatedAt != "" {
		t, err := time.Parse(time.RFC3339, j.CreatedAt)
		if err != nil {
			return fmt.Errorf("parse createdAt: %w", err)
		}
		c.CreatedAt = t
	}
	if j.ResolvedAt != "" {
		t, err := time.Parse(time.RFC3339, j.ResolvedAt)
		if err != nil {
			return fmt.Errorf("parse resolvedAt: %w", err)
		}
		c.ResolvedAt = t
	}
	return nil
}

// CommentsFile is the top-level structure for the comments JSON file.
type CommentsFile struct {
	RootDir  string  `json:"rootDir"`
	Version  int     `json:"version"`
	Comments []Entry `json:"comments"`
}

const purgeAge = 30 * 24 * time.Hour

// Repository manages comment persistence.
type Repository struct {
	dir     string
	rootDir string
}

// NewRepository creates a new Repository for the given rootDir.
// The store directory is ~/.gracilius/projects/{basename-hash8}/
func NewRepository(rootDir string) (*Repository, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("get home directory: %w", err)
	}

	base := filepath.Base(rootDir)
	hash := sha256.Sum256([]byte(rootDir))
	name := fmt.Sprintf("%s-%x", base, hash[:4])

	dir := filepath.Join(homeDir, ".gracilius", "projects", name)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("create project directory: %w", err)
	}

	return &Repository{dir: dir, rootDir: rootDir}, nil
}

// DataPath returns the path to the comments JSON file.
func (s *Repository) DataPath() string {
	return filepath.Join(s.dir, "comments.json")
}

func (s *Repository) load() ([]Entry, error) {
	data, err := os.ReadFile(s.DataPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var cf CommentsFile
	if err := json.Unmarshal(data, &cf); err != nil {
		return nil, fmt.Errorf("decode comments: %w", err)
	}
	return cf.Comments, nil
}

func (s *Repository) save(comments []Entry) error {
	now := time.Now()
	var kept []Entry
	for i := range comments {
		if !comments[i].ResolvedAt.IsZero() && now.Sub(comments[i].ResolvedAt) > purgeAge {
			continue
		}
		kept = append(kept, comments[i])
	}

	cf := CommentsFile{
		RootDir:  s.rootDir,
		Version:  1,
		Comments: kept,
	}

	data, err := json.MarshalIndent(cf, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal comments: %w", err)
	}
	data = append(data, '\n')

	tmp := s.DataPath() + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}

	if err := os.Rename(tmp, s.DataPath()); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename temp file: %w", err)
	}

	return nil
}

// Add adds a comment to the store.
func (s *Repository) Add(c Entry) error {
	comments, err := s.load()
	if err != nil {
		return err
	}
	comments = append(comments, c)
	return s.save(comments)
}

// Replace removes the comment with oldID and adds c in a single operation.
func (s *Repository) Replace(oldID string, c Entry) error {
	comments, err := s.load()
	if err != nil {
		return err
	}
	for i := range comments {
		if comments[i].ID == oldID {
			comments = append(comments[:i], comments[i+1:]...)
			break
		}
	}
	comments = append(comments, c)
	return s.save(comments)
}

// Resolve marks a comment as resolved.
func (s *Repository) Resolve(id string) error {
	comments, err := s.load()
	if err != nil {
		return err
	}
	for i := range comments {
		if comments[i].ID == id {
			comments[i].ResolvedAt = time.Now()
			return s.save(comments)
		}
	}
	return fmt.Errorf("comment not found: %s", id)
}

// Delete removes a comment from the store.
func (s *Repository) Delete(id string) error {
	comments, err := s.load()
	if err != nil {
		return err
	}
	for i := range comments {
		if comments[i].ID == id {
			comments = append(comments[:i], comments[i+1:]...)
			return s.save(comments)
		}
	}
	return fmt.Errorf("comment not found: %s", id)
}

// DeleteByFile removes all comments for a specific file.
func (s *Repository) DeleteByFile(filePath string) error {
	comments, err := s.load()
	if err != nil {
		return err
	}
	var kept []Entry
	for i := range comments {
		if comments[i].FilePath != filePath {
			kept = append(kept, comments[i])
		}
	}
	return s.save(kept)
}

// List returns comments filtered by file path and resolved status.
func (s *Repository) List(filePath string, includeResolved bool) ([]Entry, error) {
	comments, err := s.load()
	if err != nil {
		return nil, err
	}
	var result []Entry
	for i := range comments {
		if filePath != "" && comments[i].FilePath != filePath {
			continue
		}
		if !includeResolved && !comments[i].ResolvedAt.IsZero() {
			continue
		}
		result = append(result, comments[i])
	}
	return result, nil
}
