package tui

import (
	"log"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/fsnotify/fsnotify"
)

// watchFile returns a tea.Cmd that watches the current file for changes.
func (m *Model) watchFile() tea.Cmd {
	w := m.watcher
	return func() tea.Msg {
		if w == nil {
			return nil
		}
		for {
			select {
			case event, ok := <-w.Events:
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
					return fileChangedMsg{lines: splitLines(content)}
				}
			case err, ok := <-w.Errors:
				if !ok {
					return nil
				}
				log.Printf("Watcher error: %v", err)
			}
		}
	}
}

// watchComments returns a tea.Cmd that watches comments.json for changes.
func (m *Model) watchComments() tea.Cmd {
	w := m.commentWatcher
	dataPath := m.commentRepo.DataPath()
	return func() tea.Msg {
		if w == nil {
			return nil
		}
		for {
			select {
			case event, ok := <-w.Events:
				if !ok {
					return nil
				}
				if event.Name != dataPath {
					continue
				}
				if event.Op&(fsnotify.Write|fsnotify.Create) != 0 {
					log.Printf("Comments file changed: %s (%s)", event.Name, event.Op)
					return commentsChangedMsg{}
				}
			case err, ok := <-w.Errors:
				if !ok {
					return nil
				}
				log.Printf("Comment watcher error: %v", err)
			}
		}
	}
}

// watchDir returns a tea.Cmd that watches directories for changes.
func (m *Model) watchDir() tea.Cmd {
	w := m.dirWatcher
	return func() tea.Msg {
		if w == nil {
			return nil
		}
		for {
			select {
			case event, ok := <-w.Events:
				if !ok {
					return nil
				}
				if event.Op&(fsnotify.Create|fsnotify.Remove|fsnotify.Rename) != 0 {
					log.Printf("Directory changed: %s (%s)", event.Name, event.Op)
					if event.Op&fsnotify.Create != 0 {
						if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
							if !isHiddenEntry(info.Name()) {
								if err := WatchDirRecursive(w, event.Name); err != nil {
									log.Printf("Failed to watch new dir: %v", err)
								}
							}
						}
					}
					return treeChangedMsg{}
				}
			case err, ok := <-w.Errors:
				if !ok {
					return nil
				}
				log.Printf("Dir watcher error: %v", err)
			}
		}
	}
}
