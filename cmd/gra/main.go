package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	tea "charm.land/bubbletea/v2"
	"github.com/708u/gracilius/internal/server"
	"github.com/708u/gracilius/internal/tui"
	"github.com/fsnotify/fsnotify"
	"github.com/google/uuid"
)

const (
	exitOK  = 0
	exitErr = 1
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "mcp" {
		os.Exit(runMCP())
		return
	}
	os.Exit(run())
}

func run() int {
	// Log file setup
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get home directory: %v\n", err)
		return exitErr
	}
	logDir := filepath.Join(homeDir, ".gracilius", "logs")
	if err := os.MkdirAll(logDir, 0700); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create log directory: %v\n", err)
		return exitErr
	}
	id, err := uuid.NewV7()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate UUIDv7: %v\n", err)
		return exitErr
	}
	logPath := filepath.Join(logDir, id.String()+".log")
	logFile, err := os.Create(logPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create log file: %v\n", err)
		return exitErr
	}
	// Create latest symlink
	latestLink := filepath.Join(logDir, "latest")
	_ = os.Remove(latestLink)
	if err := os.Symlink(logPath, latestLink); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to create latest symlink: %v\n", err)
	}
	defer func() { _ = logFile.Close() }()
	log.SetOutput(logFile)
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.LUTC)

	// Get directory argument
	rootDir := "."
	if len(os.Args) > 1 {
		rootDir = os.Args[1]
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	absRootDir, err := filepath.Abs(rootDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to resolve root directory: %v\n", err)
		return exitErr
	}
	srv, err := server.New([]string{absRootDir})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create server: %v\n", err)
		return exitErr
	}

	if err := srv.Listen(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start server: %v\n", err)
		return exitErr
	}

	go srv.Serve()
	defer srv.Stop()

	// File watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create watcher: %v\n", err)
		return exitErr
	}
	defer func() { _ = watcher.Close() }()

	// Directory watcher
	dirWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create dir watcher: %v\n", err)
		return exitErr
	}
	defer func() { _ = dirWatcher.Close() }()

	if err := tui.WatchDirRecursive(dirWatcher, absRootDir); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to watch root dir: %v\n", err)
		return exitErr
	}

	m, err := tui.NewModel(srv, rootDir, watcher, dirWatcher)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create TUI model: %v\n", err)
		return exitErr
	}
	p := tea.NewProgram(m,
		tea.WithContext(ctx),
	)

	// Register callbacks
	srv.SetOpenDiffCallback(func(filePath, contents, tabName string, accept func(string), reject func()) {
		log.Printf("openDiff callback: %s", filePath)
		p.Send(tui.OpenDiffMsg{
			FilePath: filePath,
			Contents: contents,
			Accept: func(newContents string) {
				log.Printf("diff accepted: %s", filePath)
				accept(newContents)
			},
			Reject: func() {
				log.Printf("diff rejected: %s", filePath)
				reject()
			},
		})
	})

	srv.SetCloseTabCallback(func() {
		log.Printf("close_tab callback")
		p.Send(tui.CloseDiffMsg{})
	})

	srv.SetIdeConnectedCallback(func() {
		log.Printf("ide_connected callback: sending initial selection")
		p.Send(tui.IdeConnectedMsg{})
	})

	if _, err := p.Run(); err != nil && !errors.Is(err, tea.ErrProgramKilled) {
		fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
		return exitErr
	}

	return exitOK
}
