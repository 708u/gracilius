package server

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"syscall"
)

// LockFileData represents the lock file content.
type LockFileData struct {
	PID              int      `json:"pid"`
	WorkspaceFolders []string `json:"workspaceFolders"`
	IDEName          string   `json:"ideName"`
	Transport        string   `json:"transport"`
	RunningInWindows bool     `json:"runningInWindows"`
	AuthToken        string   `json:"authToken"`
}

// LockFile manages the IDE lock file.
type LockFile struct {
	port     int
	data     LockFileData
	filePath string
}

// NewLockFile creates a new LockFile instance.
func NewLockFile(port int, workspaceFolders []string, authToken string) (*LockFile, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}
	filePath := filepath.Join(homeDir, ".claude", "ide", strconv.Itoa(port)+".lock")

	return &LockFile{
		port:     port,
		filePath: filePath,
		data: LockFileData{
			PID:              os.Getpid(),
			WorkspaceFolders: workspaceFolders,
			IDEName:          "gracilius",
			Transport:        "ws",
			RunningInWindows: runtime.GOOS == "windows",
			AuthToken:        authToken,
		},
	}, nil
}

// Create creates the lock file atomically.
func (l *LockFile) Create() error {
	dir := filepath.Dir(l.filePath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(l.data, "", "  ")
	if err != nil {
		return err
	}

	tmpPath := l.filePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o600); err != nil {
		return err
	}

	return os.Rename(tmpPath, l.filePath)
}

// Remove removes the lock file.
func (l *LockFile) Remove() error {
	return os.Remove(l.filePath)
}

// Path returns the lock file path.
func (l *LockFile) Path() string {
	return l.filePath
}

// CheckDuplicateWorkspace checks if another gracilius instance is running
// with the same workspaceFolders. Returns an error if a duplicate is found.
func CheckDuplicateWorkspace(workspaceFolders []string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil // allow continuation if unable to check
	}

	lockDir := filepath.Join(homeDir, ".claude", "ide")
	entries, err := os.ReadDir(lockDir)
	if err != nil {
		return nil // allow continuation if directory does not exist
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".lock" {
			continue
		}

		lockPath := filepath.Join(lockDir, entry.Name())
		data, err := os.ReadFile(lockPath)
		if err != nil {
			continue
		}

		var lockData LockFileData
		if err := json.Unmarshal(data, &lockData); err != nil {
			continue
		}

		// Only check gracilius lock files
		if lockData.IDEName != "gracilius" {
			continue
		}

		// Check if workspaceFolders match
		if !slices.Equal(workspaceFolders, lockData.WorkspaceFolders) {
			continue
		}

		// Check if the process is still alive
		if isProcessAlive(lockData.PID) {
			return fmt.Errorf("another gracilius instance is already running for this directory (PID: %d, port: %s)",
				lockData.PID, strings.TrimSuffix(entry.Name(), ".lock"))
		}

		// Remove lock file if the process is dead
		os.Remove(lockPath)
	}

	return nil
}

// isProcessAlive checks if a process with the given PID is still running.
// This implementation uses POSIX signal(0) and is only valid on
// Unix-like systems (macOS, Linux).
func isProcessAlive(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// Send signal 0 to check if the process exists
	err = process.Signal(syscall.Signal(0))
	return err == nil
}
