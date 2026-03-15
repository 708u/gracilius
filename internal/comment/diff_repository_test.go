package comment

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newTestDiffRepo(t *testing.T) *DiffRepository {
	t.Helper()
	dir := t.TempDir()
	repo := &Repository{dir: dir, rootDir: "/test/project"}
	return NewDiffRepository(repo)
}

var workingCtx = DiffContext{Kind: "working"}
var branchCtx = DiffContext{Kind: "branch", Base: "main"}
var reviewCtx = DiffContext{Kind: "review", SessionID: "abc-123"}

// --------------- DiffContext.Key ---------------

func TestDiffContext_Key_Working(t *testing.T) {
	if got := workingCtx.Key(); got != "working" {
		t.Fatalf("expected 'working', got %q", got)
	}
}

func TestDiffContext_Key_Branch(t *testing.T) {
	if got := branchCtx.Key(); got != "branch-main" {
		t.Fatalf("expected 'branch-main', got %q", got)
	}
}

func TestDiffContext_Key_BranchNoBase(t *testing.T) {
	ctx := DiffContext{Kind: "branch"}
	if got := ctx.Key(); got != "branch-main" {
		t.Fatalf("expected 'branch-main', got %q", got)
	}
}

func TestDiffContext_Key_Review(t *testing.T) {
	if got := reviewCtx.Key(); got != "review-abc-123" {
		t.Fatalf("expected 'review-abc-123', got %q", got)
	}
}

// --------------- Add / List ---------------

func TestDiffRepo_Add_ThenList(t *testing.T) {
	repo := newTestDiffRepo(t)
	e := makeEntry("d1", "/a.go", 1, 5, "fix this")
	e.Side = "new"

	if err := repo.Add(workingCtx, e); err != nil {
		t.Fatalf("Add: %v", err)
	}

	got, err := repo.List(workingCtx, "", true)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1, got %d", len(got))
	}
	if got[0].Side != "new" {
		t.Fatalf("expected side 'new', got %q", got[0].Side)
	}
}

func TestDiffRepo_List_FilePathFilter(t *testing.T) {
	repo := newTestDiffRepo(t)
	_ = repo.Add(workingCtx, makeEntry("d1", "/a.go", 1, 1, "a"))
	_ = repo.Add(workingCtx, makeEntry("d2", "/b.go", 1, 1, "b"))

	got, err := repo.List(workingCtx, "/a.go", true)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1, got %d", len(got))
	}
}

func TestDiffRepo_ContextIsolation(t *testing.T) {
	repo := newTestDiffRepo(t)
	_ = repo.Add(workingCtx, makeEntry("d1", "/a.go", 1, 1, "working"))
	_ = repo.Add(branchCtx, makeEntry("d2", "/a.go", 1, 1, "branch"))

	w, _ := repo.List(workingCtx, "", true)
	b, _ := repo.List(branchCtx, "", true)

	if len(w) != 1 || w[0].ID != "d1" {
		t.Fatalf("working: expected d1, got %v", w)
	}
	if len(b) != 1 || b[0].ID != "d2" {
		t.Fatalf("branch: expected d2, got %v", b)
	}
}

// --------------- Replace ---------------

func TestDiffRepo_Replace(t *testing.T) {
	repo := newTestDiffRepo(t)
	_ = repo.Add(workingCtx, makeEntry("d1", "/a.go", 1, 1, "old"))

	replacement := makeEntry("d1-new", "/a.go", 2, 3, "new")
	if err := repo.Replace(workingCtx, "d1", replacement); err != nil {
		t.Fatalf("Replace: %v", err)
	}

	got, _ := repo.List(workingCtx, "", true)
	if len(got) != 1 || got[0].ID != "d1-new" {
		t.Fatalf("expected d1-new, got %v", got)
	}
}

// --------------- Delete ---------------

func TestDiffRepo_Delete(t *testing.T) {
	repo := newTestDiffRepo(t)
	_ = repo.Add(workingCtx, makeEntry("d1", "/a.go", 1, 1, "to delete"))
	_ = repo.Add(workingCtx, makeEntry("d2", "/a.go", 5, 5, "to keep"))

	if err := repo.Delete(workingCtx, "d1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	got, _ := repo.List(workingCtx, "", true)
	if len(got) != 1 || got[0].ID != "d2" {
		t.Fatalf("expected d2 to remain, got %v", got)
	}
}

func TestDiffRepo_Delete_NonExistent(t *testing.T) {
	repo := newTestDiffRepo(t)
	if err := repo.Delete(workingCtx, "no-such-id"); err == nil {
		t.Fatal("expected error")
	}
}

// --------------- DeleteByFile ---------------

func TestDiffRepo_DeleteByFile(t *testing.T) {
	repo := newTestDiffRepo(t)
	_ = repo.Add(workingCtx, makeEntry("d1", "/a.go", 1, 1, "a"))
	_ = repo.Add(workingCtx, makeEntry("d2", "/b.go", 1, 1, "b"))

	if err := repo.DeleteByFile(workingCtx, "/a.go"); err != nil {
		t.Fatalf("DeleteByFile: %v", err)
	}

	got, _ := repo.List(workingCtx, "", true)
	if len(got) != 1 || got[0].ID != "d2" {
		t.Fatalf("expected d2, got %v", got)
	}
}

// --------------- DeleteContext ---------------

func TestDiffRepo_DeleteContext(t *testing.T) {
	repo := newTestDiffRepo(t)
	_ = repo.Add(reviewCtx, makeEntry("d1", "/a.go", 1, 1, "review"))

	if err := repo.DeleteContext(reviewCtx); err != nil {
		t.Fatalf("DeleteContext: %v", err)
	}

	got, _ := repo.List(reviewCtx, "", true)
	if len(got) != 0 {
		t.Fatalf("expected 0, got %d", len(got))
	}
}

// --------------- Side field serialization ---------------

func TestEntry_Side_JSONRoundTrip(t *testing.T) {
	e := Entry{
		ID:        "s1",
		FilePath:  "/test.go",
		StartLine: 1,
		EndLine:   1,
		Text:      "text",
		Side:      "old",
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
	if decoded.Side != "old" {
		t.Fatalf("expected side 'old', got %q", decoded.Side)
	}
}

func TestEntry_Side_OmittedWhenEmpty(t *testing.T) {
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
	if _, ok := raw["side"]; ok {
		t.Fatal("side should be omitted when empty")
	}
}

func TestEntry_Side_BackwardCompatible(t *testing.T) {
	// Existing JSON without side field should deserialize with Side = "".
	jsonData := `{"id":"bc1","filePath":"/a.go","startLine":1,"endLine":1,"text":"old","snippet":"s","createdAt":"2024-01-01T00:00:00Z"}`
	var e Entry
	if err := json.Unmarshal([]byte(jsonData), &e); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if e.Side != "" {
		t.Fatalf("expected empty side for backward compat, got %q", e.Side)
	}
}

// --------------- DiffCommentsFile structure ---------------

func TestDiffRepo_FileStructure(t *testing.T) {
	repo := newTestDiffRepo(t)
	e := makeEntry("d1", "/a.go", 1, 1, "text")
	e.Side = "new"
	_ = repo.Add(workingCtx, e)

	path := filepath.Join(repo.DiffDir(), "working.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	var cf DiffCommentsFile
	if err := json.Unmarshal(data, &cf); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if cf.Version != 1 {
		t.Fatalf("expected version 1, got %d", cf.Version)
	}
	if cf.Context.Kind != "working" {
		t.Fatalf("expected kind 'working', got %q", cf.Context.Kind)
	}
	if cf.RootDir != "/test/project" {
		t.Fatalf("expected rootDir '/test/project', got %q", cf.RootDir)
	}
	if len(cf.Comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(cf.Comments))
	}
	if cf.Comments[0].Side != "new" {
		t.Fatalf("expected side 'new', got %q", cf.Comments[0].Side)
	}
}
