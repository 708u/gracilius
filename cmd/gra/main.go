package main

import (
	"context"
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
		fmt.Printf("Failed to resolve root directory: %v\n", err)
		os.Exit(1) //nolint:gocritic // defers are for cleanup; exit during init is safe
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
	defer func() { _ = watcher.Close() }()

	// Directory watcher
	dirWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Printf("Failed to create dir watcher: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = dirWatcher.Close() }()

	if err := tui.WatchDirRecursive(dirWatcher, absRootDir); err != nil {
		fmt.Printf("Failed to watch root dir: %v\n", err)
		os.Exit(1)
	}

	go func() {
		<-ctx.Done()
		srv.Stop()
	}()

	m := tui.NewModel(srv, ctx, rootDir, watcher, dirWatcher)
	p := tea.NewProgram(m,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	// Register callbacks
	srv.SetOpenDiffCallback(func(filePath string, contents string) {
		log.Printf("openDiff callback: %s", filePath)
		p.Send(tui.OpenDiffMsg{
			FilePath: filePath,
			Contents: contents,
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

	if _, err := p.Run(); err != nil {
		srv.Stop()
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	srv.Stop()
}
