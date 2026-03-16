package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestLockFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		port    int
		folders []string
		token   string
		verify  func(t *testing.T, lf *LockFile)
	}{
		{
			name:    "CreateAndRemove",
			port:    19999,
			folders: []string{"/test"},
			token:   "test-token",
			verify: func(t *testing.T, lf *LockFile) {
				t.Helper()
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
			},
		},
		{
			name:    "Content",
			port:    19998,
			folders: []string{"/workspace/project"},
			token:   "test-auth-token",
			verify: func(t *testing.T, lf *LockFile) {
				t.Helper()
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
				if got.AuthToken != "test-auth-token" {
					t.Fatalf("expected authToken %q, got %q", "test-auth-token", got.AuthToken)
				}
				wantWindows := runtime.GOOS == "windows"
				if got.RunningInWindows != wantWindows {
					t.Fatalf("expected runningInWindows=%v, got %v", wantWindows, got.RunningInWindows)
				}
			},
		},
		{
			name:    "AtomicWrite",
			port:    19997,
			folders: []string{"/test"},
			token:   "test-token",
			verify: func(t *testing.T, lf *LockFile) {
				t.Helper()
				if err := lf.Create(); err != nil {
					t.Fatalf("Create failed: %v", err)
				}
				t.Cleanup(func() { os.Remove(lf.Path()) })

				tmpPath := lf.Path() + ".tmp"
				if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
					t.Fatalf(".tmp file should not remain after Create, got err: %v", err)
				}
			},
		},
		{
			name:    "Path",
			port:    19996,
			folders: []string{"/test"},
			token:   "test-token",
			verify: func(t *testing.T, lf *LockFile) {
				t.Helper()
				homeDir, err := os.UserHomeDir()
				if err != nil {
					t.Fatalf("failed to get home dir: %v", err)
				}
				want := filepath.Join(homeDir, ".claude", "ide", "19996.lock")
				if lf.Path() != want {
					t.Fatalf("expected path %q, got %q", want, lf.Path())
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			lf, err := NewLockFile(tt.port, tt.folders, tt.token)
			if err != nil {
				t.Fatalf("NewLockFile failed: %v", err)
			}
			tt.verify(t, lf)
		})
	}
}
