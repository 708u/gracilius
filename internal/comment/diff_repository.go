package comment

import (
	"fmt"
	"os"
	"path/filepath"
)

// DiffRepository manages diff comment persistence.
// Comments are stored per-scope in {dir}/diff/{scope.Key()}.json.
type DiffRepository struct {
	dir     string
	rootDir string
}

// NewDiffRepository creates a new DiffRepository sharing the same
// project directory as the regular Repository.
func NewDiffRepository(repo *Repository) *DiffRepository {
	return &DiffRepository{dir: repo.dir, rootDir: repo.rootDir}
}

// DiffDir returns the path to the diff comments directory.
func (r *DiffRepository) DiffDir() string {
	return filepath.Join(r.dir, "diff")
}

func (r *DiffRepository) dataPath(sc DiffScope) string {
	return filepath.Join(r.DiffDir(), sc.Key()+".json")
}

func (r *DiffRepository) load(sc DiffScope) ([]Entry, error) {
	return loadJSON[DiffCommentsFile](r.dataPath(sc))
}

func (r *DiffRepository) save(sc DiffScope, comments []Entry) error {
	if err := os.MkdirAll(r.DiffDir(), 0700); err != nil {
		return fmt.Errorf("create diff directory: %w", err)
	}

	cf := DiffCommentsFile{
		RootDir:  r.rootDir,
		Version:  1,
		Scope:    sc,
		Comments: purgeResolved(comments),
	}
	return atomicWriteJSON(r.dataPath(sc), cf)
}

// Add adds a comment to the diff store.
func (r *DiffRepository) Add(sc DiffScope, c Entry) error {
	comments, err := r.load(sc)
	if err != nil {
		return err
	}
	comments = append(comments, c)
	return r.save(sc, comments)
}

// Replace removes the comment with oldID and adds c.
func (r *DiffRepository) Replace(sc DiffScope, oldID string, c Entry) error {
	comments, err := r.load(sc)
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
	return r.save(sc, comments)
}

// Delete removes a comment from the diff store.
func (r *DiffRepository) Delete(sc DiffScope, id string) error {
	comments, err := r.load(sc)
	if err != nil {
		return err
	}
	for i := range comments {
		if comments[i].ID == id {
			comments = append(comments[:i], comments[i+1:]...)
			return r.save(sc, comments)
		}
	}
	return fmt.Errorf("comment not found: %s", id)
}

// DeleteByFile removes all comments for a specific file.
func (r *DiffRepository) DeleteByFile(sc DiffScope, filePath string) error {
	comments, err := r.load(sc)
	if err != nil {
		return err
	}
	var kept []Entry
	for i := range comments {
		if comments[i].FilePath != filePath {
			kept = append(kept, comments[i])
		}
	}
	return r.save(sc, kept)
}

// List returns comments filtered by file path and resolved status.
func (r *DiffRepository) List(sc DiffScope, filePath string, includeResolved bool) ([]Entry, error) {
	comments, err := r.load(sc)
	if err != nil {
		return nil, err
	}
	return filterComments(comments, filePath, includeResolved), nil
}

// DeleteScope removes the entire scope file.
func (r *DiffRepository) DeleteScope(sc DiffScope) error {
	path := r.dataPath(sc)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete scope file: %w", err)
	}
	return nil
}
