package comment

import (
	"fmt"
	"os"
	"path/filepath"
)

// DiffRepository manages diff comment persistence.
// Comments are stored per-context in {dir}/diff/{context.Key()}.json.
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

func (r *DiffRepository) dataPath(ctx DiffContext) string {
	return filepath.Join(r.DiffDir(), ctx.Key()+".json")
}

func (r *DiffRepository) load(ctx DiffContext) ([]Entry, error) {
	return loadJSON[DiffCommentsFile](r.dataPath(ctx))
}

func (r *DiffRepository) save(ctx DiffContext, comments []Entry) error {
	if err := os.MkdirAll(r.DiffDir(), 0700); err != nil {
		return fmt.Errorf("create diff directory: %w", err)
	}

	cf := DiffCommentsFile{
		RootDir:  r.rootDir,
		Version:  1,
		Context:  ctx,
		Comments: purgeResolved(comments),
	}
	return atomicWriteJSON(r.dataPath(ctx), cf)
}

// Add adds a comment to the diff store.
func (r *DiffRepository) Add(ctx DiffContext, c Entry) error {
	comments, err := r.load(ctx)
	if err != nil {
		return err
	}
	comments = append(comments, c)
	return r.save(ctx, comments)
}

// Replace removes the comment with oldID and adds c.
func (r *DiffRepository) Replace(ctx DiffContext, oldID string, c Entry) error {
	comments, err := r.load(ctx)
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
	return r.save(ctx, comments)
}

// Delete removes a comment from the diff store.
func (r *DiffRepository) Delete(ctx DiffContext, id string) error {
	comments, err := r.load(ctx)
	if err != nil {
		return err
	}
	for i := range comments {
		if comments[i].ID == id {
			comments = append(comments[:i], comments[i+1:]...)
			return r.save(ctx, comments)
		}
	}
	return fmt.Errorf("comment not found: %s", id)
}

// DeleteByFile removes all comments for a specific file.
func (r *DiffRepository) DeleteByFile(ctx DiffContext, filePath string) error {
	comments, err := r.load(ctx)
	if err != nil {
		return err
	}
	var kept []Entry
	for i := range comments {
		if comments[i].FilePath != filePath {
			kept = append(kept, comments[i])
		}
	}
	return r.save(ctx, kept)
}

// List returns comments filtered by file path and resolved status.
func (r *DiffRepository) List(ctx DiffContext, filePath string, includeResolved bool) ([]Entry, error) {
	comments, err := r.load(ctx)
	if err != nil {
		return nil, err
	}
	return filterComments(comments, filePath, includeResolved), nil
}

// DeleteContext removes the entire context file.
func (r *DiffRepository) DeleteContext(ctx DiffContext) error {
	path := r.dataPath(ctx)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete context file: %w", err)
	}
	return nil
}
