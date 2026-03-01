package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand/v2"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/708u/gracilius/internal/protocol"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	maxPortRetries = 50    // maximum number of ports to try
	minPort        = 10000 // minimum port number for random selection
	maxPort        = 65535 // maximum port number for random selection
)

const commentPrefix = "[Comment]"

const debounceInterval = 100 * time.Millisecond

// wsClient represents a WebSocket client with keepalive tracking.
type wsClient struct {
	conn     *websocket.Conn
	lastPong time.Time
	mu       sync.Mutex // protects writes to conn
}

// Server is a WebSocket server for Claude Code integration.
type Server struct {
	port             int
	authToken        string
	workspaceFolders []string
	lockFile         *LockFile
	handler          *protocol.Handler
	clients          []*wsClient
	mu               sync.Mutex
	upgrader         websocket.Upgrader
	stopOnce         sync.Once

	// set by Listen, used by Serve/Stop
	mux      *http.ServeMux
	listener net.Listener
	httpSrv  *http.Server

	// debounce fields
	pendingNotify *selectionChange
	notifyTimer   *time.Timer

	// last sent selection for change detection
	lastSentSelection *selectionChange
}

// Keepalive constants
const (
	pingInterval     = 30 * time.Second // ping send interval
	keepaliveTimeout = 60 * time.Second // timeout threshold (pingInterval * 2)
	sleepThreshold   = 45 * time.Second // sleep wake detection (pingInterval * 1.5)
)

type selectionChange struct {
	filePath  string
	text      string
	startLine int
	startChar int
	endLine   int
	endChar   int
}

// loadOrCreateToken loads an existing token from ~/.gracilius/token or creates a new one.
func loadOrCreateToken() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Printf("Failed to get home directory, using new token: %v", err)
		return uuid.New().String()
	}

	graciliusDir := filepath.Join(homeDir, ".gracilius")
	tokenPath := filepath.Join(graciliusDir, "token")

	// Try to read existing token
	data, err := os.ReadFile(tokenPath)
	if err == nil {
		token := strings.TrimSpace(string(data))
		if token != "" {
			log.Printf("Loaded existing token from %s", tokenPath)
			return token
		}
	}

	// Create new token
	token := uuid.New().String()

	// Ensure directory exists
	if err := os.MkdirAll(graciliusDir, 0700); err != nil {
		log.Printf("Failed to create directory %s: %v", graciliusDir, err)
		return token
	}

	// Save token
	if err := os.WriteFile(tokenPath, []byte(token), 0600); err != nil {
		log.Printf("Failed to save token to %s: %v", tokenPath, err)
	} else {
		log.Printf("Created new token at %s", tokenPath)
	}

	return token
}

// New creates a new Server instance.
func New(workspaceFolders []string) (*Server, error) {
	authToken := loadOrCreateToken()

	return &Server{
		authToken:        authToken,
		workspaceFolders: workspaceFolders,
		handler:          protocol.NewHandler(workspaceFolders),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}, nil
}

// Listen binds a random port and creates the lock file.
// Selects a random port in the range [minPort, maxPort] up to maxPortRetries attempts.
// Call Serve after Listen to start accepting connections.
func (s *Server) Listen() error {
	s.mux = http.NewServeMux()
	s.mux.HandleFunc("/", s.handleWebSocket)

	// Find an available port by random selection
	var err error

	for range maxPortRetries {
		tryPort := minPort + rand.IntN(maxPort-minPort+1)
		addr := "127.0.0.1:" + strconv.Itoa(tryPort)
		s.listener, err = net.Listen("tcp", addr)
		if err == nil {
			s.port = tryPort
			break
		}
		log.Printf("Port %d in use, trying next...", tryPort)
	}

	if s.listener == nil {
		return fmt.Errorf("failed to find available port after %d attempts: %w",
			maxPortRetries, err)
	}

	// Create lock file after port is determined
	lockFile, err := NewLockFile(s.port, s.workspaceFolders, s.authToken)
	if err != nil {
		_ = s.listener.Close()
		return err
	}
	s.lockFile = lockFile
	if err := s.lockFile.Create(); err != nil {
		_ = s.listener.Close()
		return err
	}

	log.Printf("Lock file created: %s", s.lockFile.Path())
	log.Printf("Listening on %s", s.getAddr())
	return nil
}

// Serve starts accepting connections (blocking).
// Listen must be called before Serve.
// Call Stop from another goroutine to shut down.
func (s *Server) Serve() {
	s.httpSrv = &http.Server{Handler: s.mux}
	if err := s.httpSrv.Serve(s.listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Printf("Server error: %v", err)
	}
}

// Stop gracefully shuts down the server and removes the lock file.
func (s *Server) Stop() {
	s.stopOnce.Do(func() {
		if s.httpSrv != nil {
			_ = s.httpSrv.Shutdown(context.Background())
		}

		s.mu.Lock()
		for _, client := range s.clients {
			_ = client.conn.Close()
		}
		s.clients = nil
		s.mu.Unlock()

		if err := s.lockFile.Remove(); err != nil {
			log.Printf("Failed to remove lock file: %v", err)
		} else {
			log.Println("Lock file removed")
		}
	})
}

// Port returns the server port.
func (s *Server) Port() int {
	return s.port
}

// LockFilePath returns the lock file path for cleanup on panic.
func (s *Server) LockFilePath() string {
	return s.lockFile.Path()
}

// SetOpenDiffCallback sets the callback for openDiff events.
func (s *Server) SetOpenDiffCallback(cb protocol.OpenDiffCallback) {
	s.handler.SetOpenDiffCallback(cb)
}

// SetCloseTabCallback sets the callback for close_tab events.
func (s *Server) SetCloseTabCallback(cb protocol.CloseTabCallback) {
	s.handler.SetCloseTabCallback(cb)
}

// SetIdeConnectedCallback sets the callback for ide_connected events.
func (s *Server) SetIdeConnectedCallback(cb protocol.IdeConnectedCallback) {
	s.handler.SetIdeConnectedCallback(cb)
}

// hasSelectionChanged checks if the selection has changed from the last sent state.
func (s *Server) hasSelectionChanged(filePath, text string, startLine, startChar, endLine, endChar int) bool {
	if s.lastSentSelection == nil {
		return true
	}
	last := s.lastSentSelection
	if last.filePath != filePath {
		return true
	}
	if last.text != text {
		return true
	}
	if last.startLine != startLine || last.startChar != startChar {
		return true
	}
	if last.endLine != endLine || last.endChar != endChar {
		return true
	}
	return false
}

// NotifySelectionChanged sends a selection_changed notification to all connected clients.
// Uses debounce (100ms) to prevent flooding with rapid cursor movements.
// Only sends if the selection has actually changed.
// Comment notifications (text starts with "[Comment]") are sent immediately.
func (s *Server) NotifySelectionChanged(filePath, text string, startLine, startChar, endLine, endChar int) {
	sel := &selectionChange{
		filePath:  filePath,
		text:      text,
		startLine: startLine,
		startChar: startChar,
		endLine:   endLine,
		endChar:   endChar,
	}

	// Comment notifications are sent immediately
	if strings.HasPrefix(text, commentPrefix) {
		s.mu.Lock()
		if s.notifyTimer != nil {
			s.notifyTimer.Stop()
		}
		s.pendingNotify = nil
		s.broadcastSelection(sel)
		s.mu.Unlock()
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.hasSelectionChanged(filePath, text, startLine, startChar, endLine, endChar) {
		return
	}

	s.pendingNotify = sel

	if s.notifyTimer != nil {
		s.notifyTimer.Stop()
	}

	s.notifyTimer = time.AfterFunc(debounceInterval, func() {
		s.mu.Lock()
		defer s.mu.Unlock()

		if s.pendingNotify == nil {
			return
		}
		n := s.pendingNotify
		s.pendingNotify = nil
		s.broadcastSelection(n)
		s.lastSentSelection = n
	})
}

// broadcastSelection sends a selection_changed notification to all connected clients.
// The caller must hold s.mu.
func (s *Server) broadcastSelection(sel *selectionChange) {
	params := protocol.SelectionChangedParams{
		Text:     sel.text,
		FilePath: sel.filePath,
		FileURL:  (&url.URL{Scheme: "file", Path: sel.filePath}).String(),
		Selection: protocol.Selection{
			Start: protocol.Position{
				Line:      sel.startLine,
				Character: sel.startChar,
			},
			End: protocol.Position{
				Line:      sel.endLine,
				Character: sel.endChar,
			},
			IsEmpty: sel.startLine == sel.endLine && sel.startChar == sel.endChar,
		},
	}

	notification := protocol.NewNotification("selection_changed", params)
	data, err := json.Marshal(notification)
	if err != nil {
		log.Printf("Error marshaling selection_changed: %v", err)
		return
	}

	for i, client := range s.clients {
		client.mu.Lock()
		err := client.conn.WriteMessage(websocket.TextMessage, data)
		client.mu.Unlock()
		if err != nil {
			log.Printf("Error sending selection_changed to client %d: %v", i, err)
		} else {
			log.Printf("Sent selection_changed to client %d", i)
		}
	}
}

func (s *Server) getAddr() string {
	return "127.0.0.1:" + strconv.Itoa(s.port)
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	log.Printf("Received WebSocket request from %s", r.RemoteAddr)
	authHeader := r.Header.Get("x-claude-code-ide-authorization")
	if authHeader != s.authToken {
		log.Printf("Auth failed: token mismatch from %s", r.RemoteAddr)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	log.Println("Client connected with valid auth")

	// Create wsClient wrapper
	client := &wsClient{
		conn:     conn,
		lastPong: time.Now(),
	}

	// Reply with pong when ping is received
	conn.SetPingHandler(func(appData string) error {
		log.Println("Received ping, sending pong")
		client.mu.Lock()
		defer client.mu.Unlock()
		return conn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(time.Second))
	})

	// Update lastPong when pong is received (ReadDeadline is not used)
	conn.SetPongHandler(func(string) error {
		client.mu.Lock()
		client.lastPong = time.Now()
		client.mu.Unlock()
		log.Println("Received pong, updated lastPong")
		return nil
	})

	// Do not set ReadDeadline
	// Timeout detection is based on lastPong

	// Periodically send pings from the server and check for timeouts
	pingTicker := time.NewTicker(pingInterval)
	pingDone := make(chan struct{})
	lastTickTime := time.Now()

	go func() {
		defer pingTicker.Stop()
		for {
			select {
			case <-pingTicker.C:
				now := time.Now()
				elapsed := now.Sub(lastTickTime)

				// Detect wake from sleep
				if elapsed > sleepThreshold {
					log.Printf("Detected potential wake from sleep (%.1fs elapsed), resetting keepalive timer", elapsed.Seconds())
					client.mu.Lock()
					client.lastPong = now
					client.mu.Unlock()
				}

				lastTickTime = now

				// Timeout check (based on lastPong)
				client.mu.Lock()
				timeSincePong := now.Sub(client.lastPong)
				client.mu.Unlock()

				if timeSincePong > keepaliveTimeout {
					log.Printf("Client keepalive timeout (%.1fs idle), closing connection", timeSincePong.Seconds())
					_ = conn.Close()
					return
				}

				// Send ping
				client.mu.Lock()
				err := conn.WriteControl(websocket.PingMessage, []byte("ping"), time.Now().Add(time.Second))
				client.mu.Unlock()
				if err != nil {
					log.Printf("Ping failed: %v", err)
					return
				}
			case <-pingDone:
				return
			}
		}
	}()

	s.mu.Lock()
	s.clients = append(s.clients, client)
	s.mu.Unlock()

	defer func() {
		close(pingDone)
		_ = conn.Close()
		s.mu.Lock()
		for i, c := range s.clients {
			if c == client {
				s.clients = append(s.clients[:i], s.clients[i+1:]...)
				break
			}
		}
		s.mu.Unlock()
		log.Println("Client disconnected")
	}()

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// Update lastPong on message receipt as well (proof of activity)
		client.mu.Lock()
		client.lastPong = time.Now()
		client.mu.Unlock()

		s.handleMessage(client, message)
	}
}

func (s *Server) handleMessage(client *wsClient, message []byte) {
	var req protocol.Request
	if err := json.Unmarshal(message, &req); err != nil {
		log.Printf("Error parsing message: %v", err)
		return
	}

	log.Printf("Received: %s", string(message))

	resp := s.handler.HandleMessage(&req)

	if resp != nil {
		data, err := json.Marshal(resp)
		if err != nil {
			log.Printf("Error marshaling response: %v", err)
			return
		}
		client.mu.Lock()
		err = client.conn.WriteMessage(websocket.TextMessage, data)
		client.mu.Unlock()
		if err != nil {
			log.Printf("Error sending response: %v", err)
			return
		}
		log.Printf("Sent response for: %s", req.Method)
	}

	// Note: The initialized notification is sent by the client (Claude Code).
	// The server only receives notifications/initialized.
}
