package tui

import "sync"

type selectionNotification struct {
	filePath                               string
	text                                   string
	startLine, startChar, endLine, endChar int
}

type mockServer struct {
	mu            sync.Mutex
	port          int
	notifications []selectionNotification
}

func (s *mockServer) Port() int { return s.port }

func (s *mockServer) NotifySelectionChanged(
	filePath, text string,
	startLine, startChar, endLine, endChar int,
) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.notifications = append(s.notifications, selectionNotification{
		filePath:  filePath,
		text:      text,
		startLine: startLine,
		startChar: startChar,
		endLine:   endLine,
		endChar:   endChar,
	})
}

func (s *mockServer) lastNotification() (selectionNotification, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.notifications) == 0 {
		return selectionNotification{}, false
	}
	return s.notifications[len(s.notifications)-1], true
}
