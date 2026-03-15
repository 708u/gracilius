package protocol

import (
	"encoding/json"
	"sync"
	"testing"
	"time"
)

func testID() json.RawMessage {
	return json.RawMessage(`1`)
}

// collectSend returns a send function and a channel that receives
// responses passed to it. Useful for verifying HandleMessage output.
func collectSend() (func(*Response), <-chan *Response) {
	ch := make(chan *Response, 8)
	return func(r *Response) { ch <- r }, ch
}

func TestHandleInitialize_Capabilities(t *testing.T) {
	h := NewHandler([]string{"/workspace"})
	req := &Request{
		JSONRPC: "2.0",
		ID:      testID(),
		Method:  "initialize",
		Params:  json.RawMessage(`{"protocolVersion":"2025-11-25"}`),
	}

	send, ch := collectSend()
	h.HandleMessage(req, send)

	select {
	case resp := <-ch:
		if resp == nil {
			t.Fatal("initialize should return a response")
		}
		data, err := json.Marshal(resp.Result)
		if err != nil {
			t.Fatalf("failed to marshal result: %v", err)
		}
		var result map[string]json.RawMessage
		if err := json.Unmarshal(data, &result); err != nil {
			t.Fatalf("failed to unmarshal result: %v", err)
		}
		capsData, ok := result["capabilities"]
		if !ok {
			t.Fatal("capabilities field missing")
		}
		var caps map[string]json.RawMessage
		if err := json.Unmarshal(capsData, &caps); err != nil {
			t.Fatalf("failed to unmarshal capabilities: %v", err)
		}
		if _, ok := caps["tools"]; !ok {
			t.Fatal("capabilities should contain 'tools' field")
		}
	default:
		t.Fatal("expected a response from initialize")
	}
}

func TestHandleOpenDiff_Blocking(t *testing.T) {
	h := NewHandler([]string{"/workspace"})

	var cbCalled bool
	h.SetOpenDiffCallback(func(filePath, contents, tabName string, accept func(string), reject func()) {
		cbCalled = true
	})

	args, _ := json.Marshal(OpenDiffArgs{
		OldFilePath:     "/workspace/file.go",
		NewFilePath:     "/workspace/file.go",
		NewFileContents: "new content",
		TabName:         "file.go",
	})
	params, _ := json.Marshal(ToolCallParams{
		Name:      "openDiff",
		Arguments: args,
	})

	req := &Request{
		JSONRPC: "2.0",
		ID:      testID(),
		Method:  "tools/call",
		Params:  params,
	}

	send, ch := collectSend()
	h.HandleMessage(req, send)

	// openDiff should not send an immediate response
	select {
	case <-ch:
		t.Fatal("openDiff should not return immediate response")
	default:
	}
	if !cbCalled {
		t.Fatal("openDiff callback should have been called")
	}
}

func TestDiffResponder_Accept(t *testing.T) {
	send, ch := collectSend()
	r := &DiffResponder{
		send: send,
		id:   testID(),
	}

	r.Accept("saved content")

	select {
	case resp := <-ch:
		if resp == nil {
			t.Fatal("expected non-nil response")
		}
		data, _ := json.Marshal(resp.Result)
		var mcpResult MCPResult
		if err := json.Unmarshal(data, &mcpResult); err != nil {
			t.Fatalf("failed to unmarshal MCP result: %v", err)
		}
		if len(mcpResult.Content) < 2 {
			t.Fatalf("expected 2 content items, got %d", len(mcpResult.Content))
		}
		if mcpResult.Content[0].Text != diffResultAccepted {
			t.Fatalf("expected FILE_SAVED, got %v", mcpResult.Content[0].Text)
		}
		if mcpResult.Content[1].Text != "saved content" {
			t.Fatalf("expected saved content, got %v", mcpResult.Content[1].Text)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for response")
	}
}

func TestDiffResponder_Reject(t *testing.T) {
	send, ch := collectSend()
	r := &DiffResponder{
		send:    send,
		id:      testID(),
		tabName: "test.go",
	}

	r.Reject()

	select {
	case resp := <-ch:
		if resp == nil {
			t.Fatal("expected non-nil response")
		}
		data, _ := json.Marshal(resp.Result)
		var mcpResult MCPResult
		if err := json.Unmarshal(data, &mcpResult); err != nil {
			t.Fatalf("failed to unmarshal MCP result: %v", err)
		}
		if len(mcpResult.Content) < 2 {
			t.Fatalf("expected 2 content items, got %d", len(mcpResult.Content))
		}
		if mcpResult.Content[0].Text != diffResultRejected {
			t.Fatalf("expected DIFF_REJECTED, got %v", mcpResult.Content[0].Text)
		}
		if mcpResult.Content[1].Text != "test.go" {
			t.Fatalf("expected tab name, got %v", mcpResult.Content[1].Text)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for response")
	}
}

func TestDiffResponder_DoubleCall(t *testing.T) {
	send, ch := collectSend()
	r := &DiffResponder{
		send: send,
		id:   testID(),
	}

	r.Accept("content")
	r.Accept("content") // should be safe (no-op)
	r.Reject()          // should be safe (no-op)

	select {
	case resp := <-ch:
		data, _ := json.Marshal(resp.Result)
		var mcpResult MCPResult
		json.Unmarshal(data, &mcpResult)
		if len(mcpResult.Content) == 0 || mcpResult.Content[0].Text != diffResultAccepted {
			t.Fatalf("first call should win, got %v", mcpResult.Content)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out")
	}

	// send should have been called only once
	select {
	case <-ch:
		t.Fatal("should not receive second response")
	default:
		// expected
	}
}

func TestPendingDiffs_RejectAll(t *testing.T) {
	h := NewHandler([]string{"/workspace"})

	var chs []<-chan *Response
	h.SetOpenDiffCallback(func(filePath, contents, tabName string, accept func(string), reject func()) {
	})

	// Create multiple pending diffs
	for i := range 3 {
		id := json.RawMessage(`"` + string(rune('a'+i)) + `"`)
		args, _ := json.Marshal(OpenDiffArgs{
			OldFilePath:     "/workspace/file.go",
			NewFilePath:     "/workspace/file.go",
			NewFileContents: "content",
		})
		params, _ := json.Marshal(ToolCallParams{
			Name:      "openDiff",
			Arguments: args,
		})
		req := &Request{
			JSONRPC: "2.0",
			ID:      id,
			Method:  "tools/call",
			Params:  params,
		}
		send, ch := collectSend()
		h.HandleMessage(req, send)
		chs = append(chs, ch)
	}

	h.RejectAllPendingDiffs()

	// All channels should have received DIFF_REJECTED
	for i, ch := range chs {
		select {
		case resp := <-ch:
			data, _ := json.Marshal(resp.Result)
			var mcpResult MCPResult
			json.Unmarshal(data, &mcpResult)
			if len(mcpResult.Content) == 0 || mcpResult.Content[0].Text != diffResultRejected {
				t.Fatalf("channel %d: expected DIFF_REJECTED, got %v", i, mcpResult.Content)
			}
		case <-time.After(time.Second):
			t.Fatalf("channel %d: timed out", i)
		}
	}
}

func TestCloseTab_RejectsPending(t *testing.T) {
	h := NewHandler([]string{"/workspace"})

	h.SetOpenDiffCallback(func(filePath, contents, tabName string, accept func(string), reject func()) {
	})
	h.SetCloseTabCallback(func() {})

	// Create a pending diff
	args, _ := json.Marshal(OpenDiffArgs{
		OldFilePath:     "/workspace/file.go",
		NewFilePath:     "/workspace/file.go",
		NewFileContents: "content",
	})
	params, _ := json.Marshal(ToolCallParams{
		Name:      "openDiff",
		Arguments: args,
	})
	req := &Request{
		JSONRPC: "2.0",
		ID:      testID(),
		Method:  "tools/call",
		Params:  params,
	}
	diffSend, diffCh := collectSend()
	h.HandleMessage(req, diffSend)

	// Call close_tab
	closeParams, _ := json.Marshal(ToolCallParams{Name: "close_tab"})
	closeReq := &Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`2`),
		Method:  "tools/call",
		Params:  closeParams,
	}
	closeSend, closeCh := collectSend()
	h.HandleMessage(closeReq, closeSend)

	// close_tab should return immediate response
	select {
	case resp := <-closeCh:
		if resp == nil {
			t.Fatal("close_tab should return immediate response")
		}
	default:
		t.Fatal("close_tab should return immediate response")
	}

	// Pending diff should have been rejected via diffSend
	select {
	case r := <-diffCh:
		data, _ := json.Marshal(r.Result)
		var mcpResult MCPResult
		json.Unmarshal(data, &mcpResult)
		if len(mcpResult.Content) == 0 || mcpResult.Content[0].Text != diffResultRejected {
			t.Fatalf("expected DIFF_REJECTED, got %v", mcpResult.Content)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for reject")
	}
}

func TestCloseAllDiffTabs_RejectsPending(t *testing.T) {
	h := NewHandler([]string{"/workspace"})

	h.SetOpenDiffCallback(func(filePath, contents, tabName string, accept func(string), reject func()) {
	})
	h.SetCloseTabCallback(func() {})

	args, _ := json.Marshal(OpenDiffArgs{
		OldFilePath:     "/workspace/file.go",
		NewFilePath:     "/workspace/file.go",
		NewFileContents: "content",
	})
	params, _ := json.Marshal(ToolCallParams{
		Name:      "openDiff",
		Arguments: args,
	})
	req := &Request{
		JSONRPC: "2.0",
		ID:      testID(),
		Method:  "tools/call",
		Params:  params,
	}
	diffSend, diffCh := collectSend()
	h.HandleMessage(req, diffSend)

	// Call closeAllDiffTabs
	closeParams, _ := json.Marshal(ToolCallParams{Name: "closeAllDiffTabs"})
	closeReq := &Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`2`),
		Method:  "tools/call",
		Params:  closeParams,
	}
	closeSend, closeCh := collectSend()
	h.HandleMessage(closeReq, closeSend)

	select {
	case resp := <-closeCh:
		if resp == nil {
			t.Fatal("closeAllDiffTabs should return immediate response")
		}
	default:
		t.Fatal("closeAllDiffTabs should return immediate response")
	}

	select {
	case r := <-diffCh:
		data, _ := json.Marshal(r.Result)
		var mcpResult MCPResult
		json.Unmarshal(data, &mcpResult)
		if len(mcpResult.Content) == 0 || mcpResult.Content[0].Text != diffResultRejected {
			t.Fatalf("expected DIFF_REJECTED, got %v", mcpResult.Content)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for reject")
	}
}

func TestToolsList_IncludesSelectionTools(t *testing.T) {
	h := NewHandler([]string{"/workspace"})
	req := &Request{
		JSONRPC: "2.0",
		ID:      testID(),
		Method:  "tools/list",
	}

	send, ch := collectSend()
	h.HandleMessage(req, send)

	resp := <-ch
	data, _ := json.Marshal(resp.Result)
	var result struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to unmarshal tools/list result: %v", err)
	}

	want := map[string]bool{
		"getCurrentSelection": false,
		"getLatestSelection":  false,
	}
	for _, tool := range result.Tools {
		if _, ok := want[tool.Name]; ok {
			want[tool.Name] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("tools/list should include %s", name)
		}
	}
}

func TestGetCurrentSelection_NoCallback(t *testing.T) {
	h := NewHandler([]string{"/workspace"})

	params, _ := json.Marshal(ToolCallParams{Name: "getCurrentSelection"})
	req := &Request{
		JSONRPC: "2.0",
		ID:      testID(),
		Method:  "tools/call",
		Params:  params,
	}

	send, ch := collectSend()
	h.HandleMessage(req, send)

	resp := <-ch
	data, _ := json.Marshal(resp.Result)
	var mcpResult MCPResult
	if err := json.Unmarshal(data, &mcpResult); err != nil {
		t.Fatalf("failed to unmarshal MCP result: %v", err)
	}
	if len(mcpResult.Content) == 0 {
		t.Fatal("expected content in response")
	}

	var selResult SelectionResult
	if err := json.Unmarshal([]byte(mcpResult.Content[0].Text), &selResult); err != nil {
		t.Fatalf("failed to unmarshal selection result: %v", err)
	}
	if selResult.Success {
		t.Fatal("expected success=false when no callback is set")
	}
	if selResult.Message != "No active editor found" {
		t.Fatalf("expected fallback message, got %q", selResult.Message)
	}
}

func TestGetCurrentSelection_WithSelection(t *testing.T) {
	h := NewHandler([]string{"/workspace"})

	expected := &SelectionResult{
		Success:  true,
		Text:     "selected text",
		FilePath: "/workspace/main.go",
		FileURL:  "file:///workspace/main.go",
		Selection: &Selection{
			Start: Position{Line: 10, Character: 5},
			End:   Position{Line: 10, Character: 18},
		},
	}
	h.SetGetSelectionCallback(func() *SelectionResult {
		return expected
	})

	params, _ := json.Marshal(ToolCallParams{Name: "getCurrentSelection"})
	req := &Request{
		JSONRPC: "2.0",
		ID:      testID(),
		Method:  "tools/call",
		Params:  params,
	}

	send, ch := collectSend()
	h.HandleMessage(req, send)

	resp := <-ch
	data, _ := json.Marshal(resp.Result)
	var mcpResult MCPResult
	if err := json.Unmarshal(data, &mcpResult); err != nil {
		t.Fatalf("failed to unmarshal MCP result: %v", err)
	}

	var selResult SelectionResult
	if err := json.Unmarshal([]byte(mcpResult.Content[0].Text), &selResult); err != nil {
		t.Fatalf("failed to unmarshal selection result: %v", err)
	}
	if !selResult.Success {
		t.Fatal("expected success=true")
	}
	if selResult.Text != "selected text" {
		t.Fatalf("expected 'selected text', got %q", selResult.Text)
	}
	if selResult.FilePath != "/workspace/main.go" {
		t.Fatalf("expected '/workspace/main.go', got %q", selResult.FilePath)
	}
	if selResult.Selection.Start.Line != 10 || selResult.Selection.Start.Character != 5 {
		t.Fatalf("unexpected start position: %+v", selResult.Selection.Start)
	}
	if selResult.Selection.End.Line != 10 || selResult.Selection.End.Character != 18 {
		t.Fatalf("unexpected end position: %+v", selResult.Selection.End)
	}
}

func TestGetLatestSelection_NoSelection(t *testing.T) {
	h := NewHandler([]string{"/workspace"})

	h.SetGetSelectionCallback(func() *SelectionResult {
		return nil
	})

	params, _ := json.Marshal(ToolCallParams{Name: "getLatestSelection"})
	req := &Request{
		JSONRPC: "2.0",
		ID:      testID(),
		Method:  "tools/call",
		Params:  params,
	}

	send, ch := collectSend()
	h.HandleMessage(req, send)

	resp := <-ch
	data, _ := json.Marshal(resp.Result)
	var mcpResult MCPResult
	if err := json.Unmarshal(data, &mcpResult); err != nil {
		t.Fatalf("failed to unmarshal MCP result: %v", err)
	}

	var selResult SelectionResult
	if err := json.Unmarshal([]byte(mcpResult.Content[0].Text), &selResult); err != nil {
		t.Fatalf("failed to unmarshal selection result: %v", err)
	}
	if selResult.Success {
		t.Fatal("expected success=false when callback returns nil")
	}
	if selResult.Message != "No selection available" {
		t.Fatalf("expected fallback message, got %q", selResult.Message)
	}
}

func TestDiffResponder_ConcurrentSafety(t *testing.T) {
	send, ch := collectSend()
	r := &DiffResponder{
		send: send,
		id:   testID(),
	}

	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			r.Accept("content")
		})
		wg.Go(func() {
			r.Reject()
		})
	}
	wg.Wait()

	// Exactly one response should have been sent
	select {
	case <-ch:
	default:
		t.Fatal("expected exactly one response")
	}
	select {
	case <-ch:
		t.Fatal("should not have second response")
	default:
	}
}
