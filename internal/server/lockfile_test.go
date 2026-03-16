package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestLockFile_CreateAndRemove(t *testing.T) {
	t.Parallel()
	port := 19999
	lf, err := NewLockFile(port, []string{"/test"}, "test-token")
	if err != nil {
		t.Fatalf("NewLockFile failed: %v", err)
	}

	if err := lf.Create(); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if _, err := os.Stat(lf.Path()); err != nil {
		t.Fatalf("lock file should exist after Create: %v", err)
	}

	if err := lf.Remove(); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	if _, err := os.Stat(lf.Path()); !os.IsNotExist(err) {
		t.Fatalf("lock file should not exist after Remove, got err: %v", err)
	}
}

func TestLockFile_Content(t *testing.T) {
	t.Parallel()
	port := 19998
	folders := []string{"/workspace/project"}
	token := "test-auth-token"
	lf, err := NewLockFile(port, folders, token)
	if err != nil {
		t.Fatalf("NewLockFile failed: %v", err)
	}

	if err := lf.Create(); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	t.Cleanup(func() { os.Remove(lf.Path()) })

	data, err := os.ReadFile(lf.Path())
	if err != nil {
		t.Fatalf("failed to read lock file: %v", err)
	}

	var got LockFileData
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("failed to unmarshal lock file: %v", err)
	}

	if got.PID != os.Getpid() {
		t.Fatalf("expected PID %d, got %d", os.Getpid(), got.PID)
	}
	if len(got.WorkspaceFolders) != 1 || got.WorkspaceFolders[0] != "/workspace/project" {
		t.Fatalf("unexpected workspaceFolders: %v", got.WorkspaceFolders)
	}
	if got.IDEName != "gracilius" {
		t.Fatalf("expected ideName %q, got %q", "gracilius", got.IDEName)
	}
	if got.Transport != "ws" {
		t.Fatalf("expected transport %q, got %q", "ws", got.Transport)
	}
	if got.AuthToken != token {
		t.Fatalf("expected authToken %q, got %q", token, got.AuthToken)
	}

	wantWindows := runtime.GOOS == "windows"
	if got.RunningInWindows != wantWindows {
		t.Fatalf("expected runningInWindows=%v, got %v", wantWindows, got.RunningInWindows)
	}
}

func TestLockFile_AtomicWrite(t *testing.T) {
	t.Parallel()
	port := 19997
	lf, err := NewLockFile(port, []string{"/test"}, "test-token")
	if err != nil {
		t.Fatalf("NewLockFile failed: %v", err)
	}

	if err := lf.Create(); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	t.Cleanup(func() { os.Remove(lf.Path()) })

	tmpPath := lf.Path() + ".tmp"
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Fatalf(".tmp file should not remain after Create, got err: %v", err)
	}
}

func TestLockFile_Path(t *testing.T) {
	t.Parallel()
	port := 19996
	lf, err := NewLockFile(port, []string{"/test"}, "test-token")
	if err != nil {
		t.Fatalf("NewLockFile failed: %v", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("failed to get home dir: %v", err)
	}

	want := filepath.Join(homeDir, ".claude", "ide", "19996.lock")
	if lf.Path() != want {
		t.Fatalf("expected path %q, got %q", want, lf.Path())
	}
}
