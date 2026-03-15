package protocol

import (
	"encoding/json"
	"sync"
)

const (
	diffResultAccepted = "FILE_SAVED"
	diffResultRejected = "DIFF_REJECTED"
)

// DiffResponder holds the send function and request ID for a pending
// diff response. Accept or Reject sends the response exactly once.
type DiffResponder struct {
	send    func(*Response)
	id      json.RawMessage
	tabName string
	once    sync.Once
	cleanup func()
}

// respond sends a two-element MCP result exactly once, then runs cleanup.
func (r *DiffResponder) respond(status, payload string) {
	r.once.Do(func() {
		result := MCPResult{
			Content: []MCPContent{
				{Type: "text", Text: status},
				{Type: "text", Text: payload},
			},
		}
		r.send(NewResponse(r.id, result))
		if r.cleanup != nil {
			r.cleanup()
		}
	})
}

// Accept sends a FILE_SAVED response with saved file contents.
func (r *DiffResponder) Accept(savedContents string) {
	r.respond(diffResultAccepted, savedContents)
}

// Reject sends a DIFF_REJECTED response with the tab name.
func (r *DiffResponder) Reject() {
	r.respond(diffResultRejected, r.tabName)
}
