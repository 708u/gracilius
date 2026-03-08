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
	"github.com/708u/gracilius/internal/comment"
	"github.com/708u/gracilius/internal/server"
	"github.com/708u/gracilius/internal/tui"
	"github.com/alecthomas/kong"
	"github.com/fsnotify/fsnotify"
	"github.com/google/uuid"
)

type CLI struct {
	View ViewCmd `cmd:"" default:"withargs" help:"Start TUI viewer"`
	Mcp  McpCmd  `cmd:"" help:"Start MCP server"`
}

type ViewCmd struct {
	Path string `arg:"" optional:"" default:"." help:"Target directory"`
}

func main() {
	var cli CLI
	cmd := kong.Parse(&cli,
		kong.Name("gra"),
		kong.Description("TUI viewer for reviewing code from Claude Code"),
	)
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func (c *ViewCmd) Run() error {
	// Log file setup
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}
	logDir := filepath.Join(homeDir, ".gracilius", "logs")
	if err := os.MkdirAll(logDir, 0700); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}
	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("failed to generate UUIDv7: %w", err)
	}
	logPath := filepath.Join(logDir, id.String()+".log")
	logFile, err := os.Create(logPath)
	if err != nil {
		return fmt.Errorf("failed to create log file: %w", err)
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

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	absRootDir, err := filepath.Abs(c.Path)
	if err != nil {
		return fmt.Errorf("failed to resolve root directory: %w", err)
	}
	srv, err := server.New([]string{absRootDir})
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	if err := srv.Listen(); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	go srv.Serve()
	defer srv.Stop()

	// File watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}
	defer func() { _ = watcher.Close() }()

	// Directory watcher
	dirWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create dir watcher: %w", err)
	}
	defer func() { _ = dirWatcher.Close() }()

	if err := tui.WatchDirRecursive(dirWatcher, absRootDir); err != nil {
		return fmt.Errorf("failed to watch root dir: %w", err)
	}

	// Comment repository
	store, err := comment.NewRepository(absRootDir)
	if err != nil {
		return fmt.Errorf("failed to create comment repository: %w", err)
	}

	// Comment file watcher
	commentWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create comment watcher: %w", err)
	}
	defer func() { _ = commentWatcher.Close() }()

	commentDir := filepath.Dir(store.DataPath())
	if err := commentWatcher.Add(commentDir); err != nil {
		return fmt.Errorf("failed to watch comment directory: %w", err)
	}

	m, err := tui.NewModel(srv, store, c.Path, watcher, dirWatcher, commentWatcher)
	if err != nil {
		return fmt.Errorf("failed to create TUI model: %w", err)
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
		return fmt.Errorf("TUI error: %w", err)
	}

	return nil
}
