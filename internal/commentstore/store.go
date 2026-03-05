package commentstore

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// Comment represents a review comment attached to a file.
type Comment struct {
	ID         string
	FilePath   string
	StartLine  int
	EndLine    int
	Text       string
	Snippet    string
	ResolvedAt time.Time // zero value = unresolved
	CreatedAt  time.Time
}

// commentJSON is the JSON serialization format for Comment.
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
func (c Comment) MarshalJSON() ([]byte, error) {
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
func (c *Comment) UnmarshalJSON(data []byte) error {
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
	RootDir  string    `json:"rootDir"`
	Version  int       `json:"version"`
	Comments []Comment `json:"comments"`
}

const purgeAge = 30 * 24 * time.Hour

// Store manages comment persistence.
type Store struct {
	dir     string
	rootDir string
}

// NewStore creates a new Store for the given rootDir.
// The store directory is ~/.gracilius/projects/{basename-hash8}/
func NewStore(rootDir string) (*Store, error) {
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

	return &Store{dir: dir, rootDir: rootDir}, nil
}

// DataPath returns the path to the comments JSON file.
func (s *Store) DataPath() string {
	return filepath.Join(s.dir, "comments.json")
}

func (s *Store) lockPath() string {
	return filepath.Join(s.dir, "comments.lock")
}

// withLock acquires a file lock and runs fn.
// exclusive=true for writes, false for reads.
func (s *Store) withLock(exclusive bool, fn func() error) error {
	f, err := os.OpenFile(s.lockPath(), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return fmt.Errorf("open lock file: %w", err)
	}
	defer f.Close()

	flag := syscall.LOCK_SH
	if exclusive {
		flag = syscall.LOCK_EX
	}
	if err := syscall.Flock(int(f.Fd()), flag); err != nil {
		return fmt.Errorf("flock: %w", err)
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)

	return fn()
}

// loadRaw reads comments from the data file without locking.
func (s *Store) loadRaw() ([]Comment, error) {
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

// saveRaw writes comments to the data file atomically without locking.
// Old resolved comments (>30 days) are auto-purged.
func (s *Store) saveRaw(comments []Comment) error {
	now := time.Now()
	var kept []Comment
	for _, c := range comments {
		if !c.ResolvedAt.IsZero() && now.Sub(c.ResolvedAt) > purgeAge {
			continue
		}
		kept = append(kept, c)
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
		os.Remove(tmp)
		return fmt.Errorf("rename temp file: %w", err)
	}

	return nil
}

// Load reads comments from the store.
func (s *Store) Load() ([]Comment, error) {
	var comments []Comment
	err := s.withLock(false, func() error {
		var loadErr error
		comments, loadErr = s.loadRaw()
		return loadErr
	})
	return comments, err
}

// Save writes comments to the store.
func (s *Store) Save(comments []Comment) error {
	return s.withLock(true, func() error {
		return s.saveRaw(comments)
	})
}

// Add adds a comment to the store.
func (s *Store) Add(c Comment) error {
	return s.withLock(true, func() error {
		comments, err := s.loadRaw()
		if err != nil {
			return err
		}
		comments = append(comments, c)
		return s.saveRaw(comments)
	})
}

// Replace removes the comment with oldID and adds c in a single operation.
func (s *Store) Replace(oldID string, c Comment) error {
	return s.withLock(true, func() error {
		comments, err := s.loadRaw()
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
		return s.saveRaw(comments)
	})
}

// Resolve marks a comment as resolved.
func (s *Store) Resolve(id string) error {
	return s.withLock(true, func() error {
		comments, err := s.loadRaw()
		if err != nil {
			return err
		}
		for i := range comments {
			if comments[i].ID == id {
				comments[i].ResolvedAt = time.Now()
				return s.saveRaw(comments)
			}
		}
		return fmt.Errorf("comment not found: %s", id)
	})
}

// Delete removes a comment from the store.
func (s *Store) Delete(id string) error {
	return s.withLock(true, func() error {
		comments, err := s.loadRaw()
		if err != nil {
			return err
		}
		for i := range comments {
			if comments[i].ID == id {
				comments = append(comments[:i], comments[i+1:]...)
				return s.saveRaw(comments)
			}
		}
		return fmt.Errorf("comment not found: %s", id)
	})
}

// DeleteByFile removes all comments for a specific file.
func (s *Store) DeleteByFile(filePath string) error {
	return s.withLock(true, func() error {
		comments, err := s.loadRaw()
		if err != nil {
			return err
		}
		var kept []Comment
		for _, c := range comments {
			if c.FilePath != filePath {
				kept = append(kept, c)
			}
		}
		return s.saveRaw(kept)
	})
}

// List returns comments filtered by file path and resolved status.
func (s *Store) List(filePath string, includeResolved bool) ([]Comment, error) {
	var result []Comment
	err := s.withLock(false, func() error {
		comments, err := s.loadRaw()
		if err != nil {
			return err
		}
		for _, c := range comments {
			if filePath != "" && c.FilePath != filePath {
				continue
			}
			if !includeResolved && !c.ResolvedAt.IsZero() {
				continue
			}
			result = append(result, c)
		}
		return nil
	})
	return result, err
}
