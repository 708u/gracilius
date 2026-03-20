package comment

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// newTestRepo creates a Repository backed by a temporary directory.
// Directly constructs the struct to avoid filesystem side effects
// from config.DataDir() (which ignores XDG_CONFIG_HOME on Darwin).
func newTestRepo(t *testing.T) *Repository {
	t.Helper()
	dir := t.TempDir()
	return &Repository{dir: dir, rootDir: "/test/project"}
}

func makeEntry(id, filePath string, startLine, endLine int, text string) Entry {
	return Entry{
		ID:        id,
		FilePath:  filePath,
		StartLine: startLine,
		EndLine:   endLine,
		Text:      text,
		Snippet:   "snippet",
		CreatedAt: time.Now().Truncate(time.Second),
	}
}

// --------------- NewRepository ---------------

func TestNewRepository_CreatesDirectory(t *testing.T) {
	tmp := t.TempDir()
	// On Darwin os.UserConfigDir uses $HOME/Library/Application Support;
	// on other Unix it uses $XDG_CONFIG_HOME.
	if runtime.GOOS == "darwin" {
		t.Setenv("HOME", tmp)
	} else {
		t.Setenv("XDG_CONFIG_HOME", tmp)
	}

	repo, err := NewRepository("/some/root")
	if err != nil {
		t.Fatalf("NewRepository: %v", err)
	}

	dir := filepath.Dir(repo.DataPath())
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("expected a directory")
	}
}

func TestNewRepository_ReturnsValidRepository(t *testing.T) {
	t.Parallel()
	repo := newTestRepo(t)
	if repo.DataPath() == "" {
		t.Fatal("DataPath should not be empty")
	}
	if !filepath.IsAbs(repo.DataPath()) {
		t.Fatalf("DataPath should be absolute, got %s", repo.DataPath())
	}
}

// --------------- Add / List ---------------

func TestAdd_ThenList_ReturnsSingleComment(t *testing.T) {
	t.Parallel()
	repo := newTestRepo(t)
	e := makeEntry("c1", "/a.go", 1, 5, "fix this")

	if err := repo.Add(e); err != nil {
		t.Fatalf("Add: %v", err)
	}

	got, err := repo.List("", true)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(got))
	}
	if got[0].ID != "c1" {
		t.Fatalf("expected id c1, got %s", got[0].ID)
	}
}

func TestAdd_MultipleComments_ListReturnsAll(t *testing.T) {
	t.Parallel()
	repo := newTestRepo(t)
	for _, id := range []string{"c1", "c2", "c3"} {
		if err := repo.Add(makeEntry(id, "/a.go", 1, 1, id)); err != nil {
			t.Fatalf("Add %s: %v", id, err)
		}
	}

	got, err := repo.List("", true)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 comments, got %d", len(got))
	}
}

func TestList_FilePathFilter(t *testing.T) {
	t.Parallel()
	repo := newTestRepo(t)
	_ = repo.Add(makeEntry("c1", "/a.go", 1, 1, "a"))
	_ = repo.Add(makeEntry("c2", "/b.go", 1, 1, "b"))
	_ = repo.Add(makeEntry("c3", "/a.go", 10, 10, "a2"))

	got, err := repo.List("/a.go", true)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 comments for /a.go, got %d", len(got))
	}
	for _, c := range got {
		if c.FilePath != "/a.go" {
			t.Fatalf("unexpected filePath %s", c.FilePath)
		}
	}
}

func TestList_ExcludesResolved(t *testing.T) {
	t.Parallel()
	repo := newTestRepo(t)
	_ = repo.Add(makeEntry("c1", "/a.go", 1, 1, "open"))

	resolved := makeEntry("c2", "/a.go", 5, 5, "resolved")
	resolved.ResolvedAt = time.Now().Truncate(time.Second)
	_ = repo.Add(resolved)

	got, err := repo.List("", false)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 unresolved comment, got %d", len(got))
	}
	if got[0].ID != "c1" {
		t.Fatalf("expected c1, got %s", got[0].ID)
	}
}

func TestList_IncludesResolved(t *testing.T) {
	t.Parallel()
	repo := newTestRepo(t)
	_ = repo.Add(makeEntry("c1", "/a.go", 1, 1, "open"))

	resolved := makeEntry("c2", "/a.go", 5, 5, "resolved")
	resolved.ResolvedAt = time.Now().Truncate(time.Second)
	_ = repo.Add(resolved)

	got, err := repo.List("", true)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 comments, got %d", len(got))
	}
}

// --------------- Replace ---------------

func TestReplace_ExistingComment(t *testing.T) {
	t.Parallel()
	repo := newTestRepo(t)
	_ = repo.Add(makeEntry("c1", "/a.go", 1, 1, "old text"))
	_ = repo.Add(makeEntry("c2", "/b.go", 1, 1, "other"))

	replacement := makeEntry("c1-new", "/a.go", 2, 3, "new text")
	if err := repo.Replace("c1", replacement); err != nil {
		t.Fatalf("Replace: %v", err)
	}

	got, err := repo.List("", true)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 comments after replace, got %d", len(got))
	}

	ids := map[string]bool{}
	for _, c := range got {
		ids[c.ID] = true
	}
	if ids["c1"] {
		t.Fatal("old comment c1 should have been removed")
	}
	if !ids["c1-new"] {
		t.Fatal("replacement c1-new should be present")
	}
	if !ids["c2"] {
		t.Fatal("unrelated comment c2 should still be present")
	}
}

func TestReplace_NonExistentID_AddsWithoutRemoving(t *testing.T) {
	t.Parallel()
	repo := newTestRepo(t)
	_ = repo.Add(makeEntry("c1", "/a.go", 1, 1, "existing"))

	newEntry := makeEntry("c2", "/b.go", 1, 1, "added")
	if err := repo.Replace("no-such-id", newEntry); err != nil {
		t.Fatalf("Replace: %v", err)
	}

	got, err := repo.List("", true)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 comments, got %d", len(got))
	}
}

// --------------- Resolve ---------------

func TestResolve_SetsResolvedAt(t *testing.T) {
	t.Parallel()
	repo := newTestRepo(t)
	_ = repo.Add(makeEntry("c1", "/a.go", 1, 1, "fix"))

	before := time.Now().Add(-time.Second)
	if err := repo.Resolve("c1"); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	after := time.Now().Add(time.Second)

	got, err := repo.List("", true)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(got))
	}
	if got[0].ResolvedAt.IsZero() {
		t.Fatal("ResolvedAt should be set")
	}
	if got[0].ResolvedAt.Before(before) || got[0].ResolvedAt.After(after) {
		t.Fatalf("ResolvedAt %v not in expected range [%v, %v]",
			got[0].ResolvedAt, before, after)
	}
}

func TestResolve_NonExistentID_ReturnsError(t *testing.T) {
	t.Parallel()
	repo := newTestRepo(t)

	err := repo.Resolve("no-such-id")
	if err == nil {
		t.Fatal("expected error for non-existent id")
	}
}

// --------------- Delete ---------------

func TestDelete_RemovesComment(t *testing.T) {
	t.Parallel()
	repo := newTestRepo(t)
	_ = repo.Add(makeEntry("c1", "/a.go", 1, 1, "to delete"))
	_ = repo.Add(makeEntry("c2", "/a.go", 5, 5, "to keep"))

	if err := repo.Delete("c1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	got, err := repo.List("", true)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(got))
	}
	if got[0].ID != "c2" {
		t.Fatalf("expected c2 to remain, got %s", got[0].ID)
	}
}

func TestDelete_NonExistentID_ReturnsError(t *testing.T) {
	t.Parallel()
	repo := newTestRepo(t)

	err := repo.Delete("no-such-id")
	if err == nil {
		t.Fatal("expected error for non-existent id")
	}
}

// --------------- DeleteByFile ---------------

func TestDeleteByFile_RemovesMatchingFile(t *testing.T) {
	t.Parallel()
	repo := newTestRepo(t)
	_ = repo.Add(makeEntry("c1", "/a.go", 1, 1, "a1"))
	_ = repo.Add(makeEntry("c2", "/a.go", 5, 5, "a2"))
	_ = repo.Add(makeEntry("c3", "/b.go", 1, 1, "b1"))

	if err := repo.DeleteByFile("/a.go"); err != nil {
		t.Fatalf("DeleteByFile: %v", err)
	}

	got, err := repo.List("", true)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(got))
	}
	if got[0].ID != "c3" {
		t.Fatalf("expected c3, got %s", got[0].ID)
	}
}

func TestDeleteByFile_LeavesOtherFilesIntact(t *testing.T) {
	t.Parallel()
	repo := newTestRepo(t)
	_ = repo.Add(makeEntry("c1", "/a.go", 1, 1, "a"))
	_ = repo.Add(makeEntry("c2", "/b.go", 1, 1, "b"))
	_ = repo.Add(makeEntry("c3", "/c.go", 1, 1, "c"))

	if err := repo.DeleteByFile("/b.go"); err != nil {
		t.Fatalf("DeleteByFile: %v", err)
	}

	got, err := repo.List("", true)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 comments, got %d", len(got))
	}
	ids := map[string]bool{}
	for _, c := range got {
		ids[c.ID] = true
	}
	if !ids["c1"] || !ids["c3"] {
		t.Fatalf("expected c1 and c3 to remain, got %v", ids)
	}
}

// --------------- Purge logic ---------------

func TestPurge_OldResolvedCommentsAreRemoved(t *testing.T) {
	t.Parallel()
	repo := newTestRepo(t)

	old := makeEntry("old", "/a.go", 1, 1, "old resolved")
	old.ResolvedAt = time.Now().Add(-31 * 24 * time.Hour)
	_ = repo.Add(old)

	// Trigger save via another Add, which runs purge logic.
	_ = repo.Add(makeEntry("new", "/a.go", 5, 5, "fresh"))

	got, err := repo.List("", true)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 comment after purge, got %d", len(got))
	}
	if got[0].ID != "new" {
		t.Fatalf("expected 'new' to survive purge, got %s", got[0].ID)
	}
}

func TestPurge_RecentResolvedCommentsAreKept(t *testing.T) {
	t.Parallel()
	repo := newTestRepo(t)

	recent := makeEntry("recent", "/a.go", 1, 1, "recently resolved")
	recent.ResolvedAt = time.Now().Add(-10 * 24 * time.Hour)
	_ = repo.Add(recent)

	// Trigger save via another Add.
	_ = repo.Add(makeEntry("other", "/a.go", 5, 5, "unresolved"))

	got, err := repo.List("", true)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 comments, got %d", len(got))
	}
}

func TestPurge_UnresolvedCommentsNeverPurged(t *testing.T) {
	t.Parallel()
	repo := newTestRepo(t)

	// An unresolved comment with an old CreatedAt should survive purge.
	old := makeEntry("ancient", "/a.go", 1, 1, "never resolved")
	old.CreatedAt = time.Now().Add(-365 * 24 * time.Hour)
	_ = repo.Add(old)

	// Trigger save via another Add.
	_ = repo.Add(makeEntry("new", "/b.go", 1, 1, "new"))

	got, err := repo.List("", true)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 comments (unresolved kept), got %d", len(got))
	}
	ids := map[string]bool{}
	for _, c := range got {
		ids[c.ID] = true
	}
	if !ids["ancient"] {
		t.Fatal("unresolved comment should not be purged")
	}
}

// --------------- JSON round-trip ---------------

func TestEntry_JSONRoundTrip_AllFields(t *testing.T) {
	t.Parallel()
	now := time.Now().Truncate(time.Second)
	original := Entry{
		ID:         "rt1",
		FilePath:   "/test/file.go",
		StartLine:  10,
		EndLine:    20,
		Text:       "review comment",
		Snippet:    "code snippet",
		ResolvedAt: now,
		CreatedAt:  now.Add(-time.Hour),
	}

	data, err := json.Marshal(&original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded Entry
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.ID != original.ID {
		t.Fatalf("ID mismatch: %s vs %s", decoded.ID, original.ID)
	}
	if decoded.FilePath != original.FilePath {
		t.Fatalf("FilePath mismatch: %s vs %s",
			decoded.FilePath, original.FilePath)
	}
	if decoded.StartLine != original.StartLine {
		t.Fatalf("StartLine mismatch: %d vs %d",
			decoded.StartLine, original.StartLine)
	}
	if decoded.EndLine != original.EndLine {
		t.Fatalf("EndLine mismatch: %d vs %d",
			decoded.EndLine, original.EndLine)
	}
	if decoded.Text != original.Text {
		t.Fatalf("Text mismatch: %s vs %s",
			decoded.Text, original.Text)
	}
	if decoded.Snippet != original.Snippet {
		t.Fatalf("Snippet mismatch: %s vs %s",
			decoded.Snippet, original.Snippet)
	}
	if !decoded.ResolvedAt.Equal(original.ResolvedAt) {
		t.Fatalf("ResolvedAt mismatch: %v vs %v",
			decoded.ResolvedAt, original.ResolvedAt)
	}
	if !decoded.CreatedAt.Equal(original.CreatedAt) {
		t.Fatalf("CreatedAt mismatch: %v vs %v",
			decoded.CreatedAt, original.CreatedAt)
	}
}

func TestEntry_JSON_ResolvedAtZeroOmitted(t *testing.T) {
	t.Parallel()
	e := Entry{
		ID:        "rt2",
		FilePath:  "/test.go",
		StartLine: 1,
		EndLine:   1,
		Text:      "text",
		CreatedAt: time.Now().Truncate(time.Second),
	}

	data, err := json.Marshal(&e)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal raw: %v", err)
	}
	if _, ok := raw["resolvedAt"]; ok {
		t.Fatal("resolvedAt should be omitted when zero")
	}
}

// --------------- Atomic write ---------------

func TestAtomicWrite_FileExistsAfterAdd(t *testing.T) {
	t.Parallel()
	repo := newTestRepo(t)
	_ = repo.Add(makeEntry("c1", "/a.go", 1, 1, "text"))

	data, err := os.ReadFile(repo.DataPath())
	if err != nil {
		t.Fatalf("read data file: %v", err)
	}

	var cf CommentsFile
	if err := json.Unmarshal(data, &cf); err != nil {
		t.Fatalf("unmarshal data file: %v", err)
	}
	if cf.Version != 1 {
		t.Fatalf("expected version 1, got %d", cf.Version)
	}
	if cf.RootDir != "/test/project" {
		t.Fatalf("expected rootDir /test/project, got %s", cf.RootDir)
	}
	if len(cf.Comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(cf.Comments))
	}
}

func TestAtomicWrite_NoTempFileLeftOver(t *testing.T) {
	t.Parallel()
	repo := newTestRepo(t)
	_ = repo.Add(makeEntry("c1", "/a.go", 1, 1, "text"))

	tmpPath := repo.DataPath() + ".tmp"
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Fatal("temp file should not remain after successful write")
	}
}

// --------------- Scope-aware operations ---------------

func makeScopedEntry(id, filePath string, startLine, endLine int, text string, sc DiffScope) Entry {
	e := makeEntry(id, filePath, startLine, endLine, text)
	e.Side = "new"
	e.Scope = sc
	return e
}

var workingScope = DiffScope{Kind: "working"}
var branchScope = DiffScope{Kind: "branch", Base: "main"}

func TestListByScope_FiltersByScope(t *testing.T) {
	repo := newTestRepo(t)
	_ = repo.Add(makeEntry("f1", "/a.go", 1, 1, "file comment"))
	_ = repo.Add(makeScopedEntry("d1", "/a.go", 1, 1, "working", workingScope))
	_ = repo.Add(makeScopedEntry("d2", "/a.go", 1, 1, "branch", branchScope))

	got, err := repo.ListByScope(workingScope, "", true)
	if err != nil {
		t.Fatalf("ListByScope: %v", err)
	}
	if len(got) != 1 || got[0].ID != "d1" {
		t.Fatalf("expected d1, got %v", got)
	}
}

func TestListByScope_FilePathFilter(t *testing.T) {
	repo := newTestRepo(t)
	_ = repo.Add(makeScopedEntry("d1", "/a.go", 1, 1, "a", workingScope))
	_ = repo.Add(makeScopedEntry("d2", "/b.go", 1, 1, "b", workingScope))

	got, err := repo.ListByScope(workingScope, "/a.go", true)
	if err != nil {
		t.Fatalf("ListByScope: %v", err)
	}
	if len(got) != 1 || got[0].ID != "d1" {
		t.Fatalf("expected d1, got %v", got)
	}
}

func TestList_ExcludesScopedComments(t *testing.T) {
	repo := newTestRepo(t)
	_ = repo.Add(makeEntry("f1", "/a.go", 1, 1, "file comment"))
	_ = repo.Add(makeScopedEntry("d1", "/a.go", 1, 1, "diff comment", workingScope))

	got, err := repo.List("", true)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 || got[0].ID != "f1" {
		t.Fatalf("expected only f1, got %v", got)
	}
}

func TestDeleteByScope(t *testing.T) {
	repo := newTestRepo(t)
	_ = repo.Add(makeEntry("f1", "/a.go", 1, 1, "file"))
	_ = repo.Add(makeScopedEntry("d1", "/a.go", 1, 1, "working", workingScope))
	_ = repo.Add(makeScopedEntry("d2", "/a.go", 1, 1, "branch", branchScope))

	if err := repo.DeleteByScope(workingScope); err != nil {
		t.Fatalf("DeleteByScope: %v", err)
	}

	all, _ := repo.List("", true)
	if len(all) != 1 || all[0].ID != "f1" {
		t.Fatalf("file comment should remain, got %v", all)
	}
	working, _ := repo.ListByScope(workingScope, "", true)
	if len(working) != 0 {
		t.Fatalf("working scope should be empty, got %v", working)
	}
	branch, _ := repo.ListByScope(branchScope, "", true)
	if len(branch) != 1 {
		t.Fatalf("branch scope should remain, got %v", branch)
	}
}

func TestDeleteByFileAndScope(t *testing.T) {
	repo := newTestRepo(t)
	_ = repo.Add(makeScopedEntry("d1", "/a.go", 1, 1, "a", workingScope))
	_ = repo.Add(makeScopedEntry("d2", "/b.go", 1, 1, "b", workingScope))
	_ = repo.Add(makeEntry("f1", "/a.go", 1, 1, "file"))

	if err := repo.DeleteByFileAndScope(workingScope, "/a.go"); err != nil {
		t.Fatalf("DeleteByFileAndScope: %v", err)
	}

	all, _ := repo.ListByScope(workingScope, "", true)
	if len(all) != 1 || all[0].ID != "d2" {
		t.Fatalf("expected d2, got %v", all)
	}
	files, _ := repo.List("", true)
	if len(files) != 1 || files[0].ID != "f1" {
		t.Fatalf("file comment should remain, got %v", files)
	}
}

func TestEntry_Scope_JSONRoundTrip(t *testing.T) {
	e := Entry{
		ID:        "s1",
		FilePath:  "/test.go",
		StartLine: 1,
		EndLine:   1,
		Text:      "text",
		Side:      "new",
		Scope:     DiffScope{Kind: "branch", Base: "main"},
		CreatedAt: time.Now().Truncate(time.Second),
	}

	data, err := json.Marshal(&e)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded Entry
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if decoded.Scope.Kind != "branch" || decoded.Scope.Base != "main" {
		t.Fatalf("expected scope branch/main, got %+v", decoded.Scope)
	}
}

func TestEntry_Scope_OmittedWhenZero(t *testing.T) {
	e := Entry{
		ID:        "s2",
		FilePath:  "/test.go",
		StartLine: 1,
		EndLine:   1,
		Text:      "text",
		CreatedAt: time.Now().Truncate(time.Second),
	}

	data, err := json.Marshal(&e)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal raw: %v", err)
	}
	if _, ok := raw["scope"]; ok {
		t.Fatal("scope should be omitted when zero")
	}
}

func TestEntry_Scope_BackwardCompatible(t *testing.T) {
	jsonData := `{"id":"bc1","filePath":"/a.go","startLine":1,"endLine":1,"text":"old","snippet":"s","createdAt":"2024-01-01T00:00:00Z"}`
	var e Entry
	if err := json.Unmarshal([]byte(jsonData), &e); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if e.Scope.Kind != "" {
		t.Fatalf("expected empty scope for backward compat, got %+v", e.Scope)
	}
}
