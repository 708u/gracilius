package tui

import (
	"log"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/fsnotify/fsnotify"
)

// watchFile returns a tea.Cmd that watches the current file for changes.
func (m *Model) watchFile() tea.Cmd {
	return func() tea.Msg {
		if m.watcher == nil {
			return nil
		}
		for {
			select {
			case event, ok := <-m.watcher.Events:
				if !ok {
					return nil
				}
				if event.Op&(fsnotify.Write|fsnotify.Create) != 0 {
					log.Printf("File changed: %s (%s)", event.Name, event.Op)
					content, err := os.ReadFile(event.Name)
					if err != nil {
						log.Printf("Error reading file: %v", err)
						continue
					}
					return fileChangedMsg{lines: strings.Split(string(content), "\n")}
				}
			case err, ok := <-m.watcher.Errors:
				if !ok {
					return nil
				}
				log.Printf("Watcher error: %v", err)
			}
		}
	}
}

// watchDir returns a tea.Cmd that watches directories for changes.
func (m *Model) watchDir() tea.Cmd {
	return func() tea.Msg {
		if m.dirWatcher == nil {
			return nil
		}
		for {
			select {
			case event, ok := <-m.dirWatcher.Events:
				if !ok {
					return nil
				}
				if event.Op&(fsnotify.Create|fsnotify.Remove|fsnotify.Rename) != 0 {
					log.Printf("Directory changed: %s (%s)", event.Name, event.Op)
					if event.Op&fsnotify.Create != 0 {
						if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
							if !isHiddenEntry(info.Name()) {
								if err := WatchDirRecursive(m.dirWatcher, event.Name); err != nil {
									log.Printf("Failed to watch new dir: %v", err)
								}
							}
						}
					}
					return treeChangedMsg{}
				}
			case err, ok := <-m.dirWatcher.Errors:
				if !ok {
					return nil
				}
				log.Printf("Dir watcher error: %v", err)
			}
		}
	}
}
