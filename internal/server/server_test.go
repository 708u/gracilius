package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/708u/gracilius/internal/protocol"
	"github.com/gorilla/websocket"
)

func setupServer(t *testing.T) *Server {
	t.Helper()
	srv, err := New([]string{"/test"})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	if err := srv.Listen(); err != nil {
		t.Fatalf("Listen failed: %v", err)
	}
	go srv.Serve()
	t.Cleanup(func() { srv.Stop() })
	return srv
}

func connectClient(t *testing.T, srv *Server) *websocket.Conn {
	t.Helper()
	url := fmt.Sprintf("ws://127.0.0.1:%d/", srv.Port())
	header := http.Header{}
	header.Set("x-claude-code-ide-authorization", srv.authToken)
	conn, _, err := websocket.DefaultDialer.Dial(url, header)
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func TestNew(t *testing.T) {
	srv, err := New([]string{"/test"})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	if srv == nil {
		t.Fatal("New returned nil")
	}
	if srv.authToken == "" {
		t.Fatal("authToken should not be empty")
	}
}

func TestListenAndStop(t *testing.T) {
	srv, err := New([]string{"/test"})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	if err := srv.Listen(); err != nil {
		t.Fatalf("Listen failed: %v", err)
	}

	if srv.Port() <= 0 {
		t.Fatalf("expected positive port, got %d", srv.Port())
	}

	lockPath := srv.LockFilePath()

	go srv.Serve()

	srv.Stop()

	if fileExists(lockPath) {
		t.Fatal("lock file should be removed after Stop")
	}
}

func TestWebSocketAuth_ValidToken(t *testing.T) {
	srv := setupServer(t)
	conn := connectClient(t, srv)

	// Send an initialize request
	req := protocol.Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "initialize",
		Params:  json.RawMessage(`{"protocolVersion":"2025-11-25"}`),
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		t.Fatalf("failed to send message: %v", err)
	}

	// Read response with deadline
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}

	var resp protocol.Response
	if err := json.Unmarshal(msg, &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error in response: %v", resp.Error)
	}
	if resp.Result == nil {
		t.Fatal("expected non-nil result in initialize response")
	}
}

func TestWebSocketAuth_InvalidToken(t *testing.T) {
	srv := setupServer(t)

	url := fmt.Sprintf("ws://127.0.0.1:%d/", srv.Port())
	header := http.Header{}
	header.Set("x-claude-code-ide-authorization", "wrong-token")

	_, resp, err := websocket.DefaultDialer.Dial(url, header)
	if err == nil {
		t.Fatal("expected connection to fail with invalid token")
	}
	if resp != nil && resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", resp.StatusCode)
	}
}

func TestNotifySelectionChanged_Debounce(t *testing.T) {
	srv := setupServer(t)
	conn := connectClient(t, srv)

	// Wait briefly for the client to be registered
	time.Sleep(20 * time.Millisecond)

	// Send multiple rapid notifications
	for i := range 5 {
		srv.NotifySelectionChanged(
			"/test/file.go",
			fmt.Sprintf("text-%d", i),
			i, 0, i, 5,
		)
	}

	// Wait for debounce to fire (100ms interval + margin)
	time.Sleep(200 * time.Millisecond)

	// Read messages -- should receive only the last one
	conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))

	var notifications []protocol.Notification
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}
		var n protocol.Notification
		if err := json.Unmarshal(msg, &n); err == nil && n.Method == "selection_changed" {
			notifications = append(notifications, n)
		}
	}

	if len(notifications) != 1 {
		t.Fatalf("expected 1 debounced notification, got %d", len(notifications))
	}

	// Verify it is the last notification (text-4)
	paramsData, err := json.Marshal(notifications[0].Params)
	if err != nil {
		t.Fatalf("failed to marshal params: %v", err)
	}
	var params protocol.SelectionChangedParams
	if err := json.Unmarshal(paramsData, &params); err != nil {
		t.Fatalf("failed to unmarshal params: %v", err)
	}
	if params.Text != "text-4" {
		t.Fatalf("expected last notification text %q, got %q", "text-4", params.Text)
	}
}

func TestNotifySelectionChanged_CommentImmediate(t *testing.T) {
	srv := setupServer(t)
	conn := connectClient(t, srv)

	// Wait briefly for the client to be registered
	time.Sleep(20 * time.Millisecond)

	srv.NotifySelectionChanged(
		"/test/file.go",
		"[Comment] /test/file.go:10\nsome comment",
		10, 0, 10, 0,
	)

	// Comment should arrive immediately, no need to wait for debounce
	conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))

	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("expected to receive comment notification: %v", err)
	}

	var n protocol.Notification
	if err := json.Unmarshal(msg, &n); err != nil {
		t.Fatalf("failed to unmarshal notification: %v", err)
	}
	if n.Method != "selection_changed" {
		t.Fatalf("expected selection_changed, got %q", n.Method)
	}

	paramsData, err := json.Marshal(n.Params)
	if err != nil {
		t.Fatalf("failed to marshal params: %v", err)
	}
	var params protocol.SelectionChangedParams
	if err := json.Unmarshal(paramsData, &params); err != nil {
		t.Fatalf("failed to unmarshal params: %v", err)
	}
	if params.Text != "[Comment] /test/file.go:10\nsome comment" {
		t.Fatalf("unexpected text: %q", params.Text)
	}
}

func TestHasSelectionChanged(t *testing.T) {
	srv := setupServer(t)

	// --- Phase 1: first notification is received ---
	conn1 := connectClient(t, srv)
	time.Sleep(20 * time.Millisecond)

	srv.NotifySelectionChanged("/test/file.go", "hello", 1, 0, 1, 5)
	time.Sleep(200 * time.Millisecond)

	conn1.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	_, _, err := conn1.ReadMessage()
	if err != nil {
		t.Fatalf("expected to receive first notification: %v", err)
	}

	// --- Phase 2: same args -- no notification ---
	srv.NotifySelectionChanged("/test/file.go", "hello", 1, 0, 1, 5)
	time.Sleep(200 * time.Millisecond)

	conn1.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	_, _, err = conn1.ReadMessage()
	if err == nil {
		t.Fatal("expected no notification for unchanged selection")
	}
	// Close conn1 to avoid stale deadline state
	conn1.Close()

	// --- Phase 3: different args -- notification expected ---
	conn2 := connectClient(t, srv)
	time.Sleep(20 * time.Millisecond)

	srv.NotifySelectionChanged("/test/file.go", "world", 2, 0, 2, 5)
	time.Sleep(200 * time.Millisecond)

	conn2.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	_, msg, err := conn2.ReadMessage()
	if err != nil {
		t.Fatalf("expected notification for changed selection: %v", err)
	}

	var n protocol.Notification
	if err := json.Unmarshal(msg, &n); err != nil {
		t.Fatalf("failed to unmarshal notification: %v", err)
	}

	paramsData, _ := json.Marshal(n.Params)
	var params protocol.SelectionChangedParams
	json.Unmarshal(paramsData, &params)
	if params.Text != "world" {
		t.Fatalf("expected text %q, got %q", "world", params.Text)
	}
}

func TestGetLatestSelection(t *testing.T) {
	srv, err := New([]string{"/test"})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Before any notification, GetLatestSelection returns nil
	if result := srv.GetLatestSelection(); result != nil {
		t.Fatalf("expected nil before any notification, got %+v", result)
	}

	// Simulate a selection by directly setting lastSentSelection
	// (broadcastSelection sets it, but requires connected clients to
	// actually send). We test via NotifySelectionChanged with a
	// connected client to exercise the full path.
	if err := srv.Listen(); err != nil {
		t.Fatalf("Listen failed: %v", err)
	}
	go srv.Serve()
	t.Cleanup(func() { srv.Stop() })

	conn := connectClient(t, srv)
	_ = conn

	time.Sleep(20 * time.Millisecond)

	srv.NotifySelectionChanged("/test/main.go", "func main()", 5, 0, 5, 11)

	// Wait for debounce
	time.Sleep(200 * time.Millisecond)

	result := srv.GetLatestSelection()
	if result == nil {
		t.Fatal("expected non-nil result after notification")
	}
	if !result.Success {
		t.Fatal("expected success=true")
	}
	if result.Text != "func main()" {
		t.Fatalf("expected text %q, got %q", "func main()", result.Text)
	}
	if result.FilePath != "/test/main.go" {
		t.Fatalf("expected filePath %q, got %q", "/test/main.go", result.FilePath)
	}
	if result.Selection == nil {
		t.Fatal("expected non-nil selection")
	}
	if result.Selection.Start.Line != 5 || result.Selection.Start.Character != 0 {
		t.Fatalf("unexpected start position: %+v", result.Selection.Start)
	}
	if result.Selection.End.Line != 5 || result.Selection.End.Character != 11 {
		t.Fatalf("unexpected end position: %+v", result.Selection.End)
	}
}

// fileExists returns true if the file at path exists.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
