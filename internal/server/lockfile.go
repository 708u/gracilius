package server

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
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
