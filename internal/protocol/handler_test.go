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

func TestHandleInitialize_Capabilities(t *testing.T) {
	h := NewHandler([]string{"/workspace"})
	req := &Request{
		JSONRPC: "2.0",
		ID:      testID(),
		Method:  "initialize",
		Params:  json.RawMessage(`{"protocolVersion":"2025-11-25"}`),
	}

	resp, deferred := h.HandleMessage(req)
	if deferred != nil {
		t.Fatal("initialize should not return deferred channel")
	}
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
}

func TestHandleOpenDiff_Blocking(t *testing.T) {
	h := NewHandler([]string{"/workspace"})

	var cbCalled bool
	h.SetOpenDiffCallback(func(filePath, contents, tabName string, responder *DiffResponder) {
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

	resp, deferred := h.HandleMessage(req)
	if resp != nil {
		t.Fatal("openDiff should not return immediate response")
	}
	if deferred == nil {
		t.Fatal("openDiff should return deferred channel")
	}
	if !cbCalled {
		t.Fatal("openDiff callback should have been called")
	}
}

func TestDiffResponder_Accept(t *testing.T) {
	ch := make(chan *Response, 1)
	r := &DiffResponder{
		ch: ch,
		id: testID(),
	}

	r.Accept()

	select {
	case resp := <-ch:
		if resp == nil {
			t.Fatal("expected non-nil response")
		}
		data, _ := json.Marshal(resp.Result)
		if string(data) == "" {
			t.Fatal("expected result data")
		}
		var mcpResult MCPResult
		if err := json.Unmarshal(data, &mcpResult); err != nil {
			t.Fatalf("failed to unmarshal MCP result: %v", err)
		}
		if len(mcpResult.Content) == 0 || mcpResult.Content[0].Text != diffResultAccepted {
			t.Fatalf("expected FILE_SAVED, got %v", mcpResult.Content)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for response")
	}
}

func TestDiffResponder_Reject(t *testing.T) {
	ch := make(chan *Response, 1)
	r := &DiffResponder{
		ch: ch,
		id: testID(),
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
		if len(mcpResult.Content) == 0 || mcpResult.Content[0].Text != diffResultRejected {
			t.Fatalf("expected DIFF_REJECTED, got %v", mcpResult.Content)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for response")
	}
}

func TestDiffResponder_DoubleCall(t *testing.T) {
	ch := make(chan *Response, 1)
	r := &DiffResponder{
		ch: ch,
		id: testID(),
	}

	r.Accept()
	r.Accept() // should be safe (no-op)
	r.Reject() // should be safe (no-op)

	select {
	case resp := <-ch:
		data, _ := json.Marshal(resp.Result)
		var mcpResult MCPResult
		json.Unmarshal(data, &mcpResult) //nolint:errcheck
		if len(mcpResult.Content) == 0 || mcpResult.Content[0].Text != diffResultAccepted {
			t.Fatalf("first call should win, got %v", mcpResult.Content)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out")
	}

	// Channel should have only one message
	select {
	case <-ch:
		t.Fatal("should not receive second response")
	default:
		// expected
	}
}

func TestPendingDiffs_RejectAll(t *testing.T) {
	h := NewHandler([]string{"/workspace"})

	var responders []*DiffResponder
	h.SetOpenDiffCallback(func(filePath, contents, tabName string, responder *DiffResponder) {
		responders = append(responders, responder)
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
		h.HandleMessage(req)
	}

	if len(responders) != 3 {
		t.Fatalf("expected 3 responders, got %d", len(responders))
	}

	h.RejectAllPendingDiffs()

	// All responders should have sent DIFF_REJECTED
	for i, r := range responders {
		select {
		case resp := <-r.ch:
			data, _ := json.Marshal(resp.Result)
			var mcpResult MCPResult
			json.Unmarshal(data, &mcpResult) //nolint:errcheck
			if len(mcpResult.Content) == 0 || mcpResult.Content[0].Text != diffResultRejected {
				t.Fatalf("responder %d: expected DIFF_REJECTED, got %v", i, mcpResult.Content)
			}
		case <-time.After(time.Second):
			t.Fatalf("responder %d: timed out", i)
		}
	}
}

func TestCloseTab_RejectsPending(t *testing.T) {
	h := NewHandler([]string{"/workspace"})

	var responder *DiffResponder
	h.SetOpenDiffCallback(func(filePath, contents, tabName string, r *DiffResponder) {
		responder = r
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
	h.HandleMessage(req)

	// Call close_tab
	closeParams, _ := json.Marshal(ToolCallParams{Name: "close_tab"})
	closeReq := &Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`2`),
		Method:  "tools/call",
		Params:  closeParams,
	}
	resp, _ := h.HandleMessage(closeReq)
	if resp == nil {
		t.Fatal("close_tab should return immediate response")
	}

	// Responder should have been rejected
	select {
	case r := <-responder.ch:
		data, _ := json.Marshal(r.Result)
		var mcpResult MCPResult
		json.Unmarshal(data, &mcpResult) //nolint:errcheck
		if len(mcpResult.Content) == 0 || mcpResult.Content[0].Text != diffResultRejected {
			t.Fatalf("expected DIFF_REJECTED, got %v", mcpResult.Content)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for reject")
	}
}

func TestCloseAllDiffTabs_RejectsPending(t *testing.T) {
	h := NewHandler([]string{"/workspace"})

	var responder *DiffResponder
	h.SetOpenDiffCallback(func(filePath, contents, tabName string, r *DiffResponder) {
		responder = r
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
	h.HandleMessage(req)

	// Call closeAllDiffTabs
	closeParams, _ := json.Marshal(ToolCallParams{Name: "closeAllDiffTabs"})
	closeReq := &Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`2`),
		Method:  "tools/call",
		Params:  closeParams,
	}
	resp, _ := h.HandleMessage(closeReq)
	if resp == nil {
		t.Fatal("closeAllDiffTabs should return immediate response")
	}

	select {
	case r := <-responder.ch:
		data, _ := json.Marshal(r.Result)
		var mcpResult MCPResult
		json.Unmarshal(data, &mcpResult) //nolint:errcheck
		if len(mcpResult.Content) == 0 || mcpResult.Content[0].Text != diffResultRejected {
			t.Fatalf("expected DIFF_REJECTED, got %v", mcpResult.Content)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for reject")
	}
}

func TestDiffResponder_ConcurrentSafety(t *testing.T) {
	ch := make(chan *Response, 1)
	r := &DiffResponder{
		ch: ch,
		id: testID(),
	}

	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r.Accept()
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			r.Reject()
		}()
	}
	wg.Wait()

	// Exactly one response should be in the channel
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
