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

	"github.com/708u/gracilius/internal/server"
	"github.com/708u/gracilius/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/fsnotify/fsnotify"
	"github.com/google/uuid"
)

const defaultPort = 18765

func main() {
	// Log file setup
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("Failed to get home directory: %v\n", err)
		os.Exit(1)
	}
	logDir := filepath.Join(homeDir, ".gracilius", "logs")
	if err := os.MkdirAll(logDir, 0700); err != nil {
		fmt.Printf("Failed to create log directory: %v\n", err)
		os.Exit(1)
	}
	id, err := uuid.NewV7()
	if err != nil {
		fmt.Printf("Failed to generate UUIDv7: %v\n", err)
		os.Exit(1)
	}
	logPath := filepath.Join(logDir, id.String()+".log")
	logFile, err := os.Create(logPath)
	if err != nil {
		fmt.Printf("Failed to create log file: %v\n", err)
		os.Exit(1)
	}
	// Create latest symlink
	latestLink := filepath.Join(logDir, "latest")
	os.Remove(latestLink)
	if err := os.Symlink(logPath, latestLink); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to create latest symlink: %v\n", err)
	}
	defer logFile.Close()
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
		fmt.Printf("Failed to resolve root directory: %v\n", err)
		os.Exit(1)
	}
	srv, err := server.New(defaultPort, []string{absRootDir})
	if err != nil {
		fmt.Printf("Failed to create server: %v\n", err)
		os.Exit(1)
	}

	if err := srv.Listen(); err != nil {
		fmt.Printf("Failed to start server: %v\n", err)
		os.Exit(1)
	}

	go srv.Serve()

	// File watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Printf("Failed to create watcher: %v\n", err)
		os.Exit(1)
	}
	defer watcher.Close()

	// Directory watcher
	dirWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Printf("Failed to create dir watcher: %v\n", err)
		os.Exit(1)
	}
	defer dirWatcher.Close()

	if err := tui.WatchDirRecursive(dirWatcher, absRootDir); err != nil {
		fmt.Printf("Failed to watch root dir: %v\n", err)
		os.Exit(1)
	}

	m, err := tui.NewModel(srv, rootDir, watcher, dirWatcher)
	if err != nil {
		fmt.Printf("Failed to create TUI model: %v\n", err)
		os.Exit(1)
	}
	p := tea.NewProgram(m,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
		tea.WithContext(ctx),
	)

	// Register callbacks
	srv.SetOpenDiffCallback(func(filePath string, contents string) {
		log.Printf("openDiff callback: %s (ignored, diff not yet implemented)", filePath)
	})

	srv.SetCloseTabCallback(func() {
		log.Printf("close_tab callback (ignored, diff not yet implemented)")
	})

	srv.SetIdeConnectedCallback(func() {
		log.Printf("ide_connected callback: sending initial selection")
		p.Send(tui.IdeConnectedMsg{})
	})

	if _, err := p.Run(); err != nil {
		srv.Stop()
		if errors.Is(err, tea.ErrProgramKilled) {
			return
		}
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	srv.Stop()
}
