package protocol

import "encoding/json"

const (
	jsonrpcVersion = "2.0"

	// JSON-RPC 2.0 defined error codes.
	// See: https://www.jsonrpc.org/specification#error_object
	codeMethodNotFound = -32601
	codeInvalidParams  = -32602
)

// Request represents a JSON-RPC 2.0 request.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response represents a JSON-RPC 2.0 response.
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

// Notification represents a JSON-RPC 2.0 notification (no id).
type Notification struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

// Error represents a JSON-RPC 2.0 error object.
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// NewResponse creates a success response.
func NewResponse(id json.RawMessage, result any) *Response {
	return &Response{
		JSONRPC: jsonrpcVersion,
		ID:      id,
		Result:  result,
	}
}

// NewErrorResponse creates an error response.
func NewErrorResponse(id json.RawMessage, code int, message string) *Response {
	return &Response{
		JSONRPC: jsonrpcVersion,
		ID:      id,
		Error: &Error{
			Code:    code,
			Message: message,
		},
	}
}

// NewNotification creates a notification.
func NewNotification(method string, params any) *Notification {
	return &Notification{
		JSONRPC: jsonrpcVersion,
		Method:  method,
		Params:  params,
	}
}

// SelectionChangedParams represents the parameters for selection_changed notification.
type SelectionChangedParams struct {
	Text      string    `json:"text,omitempty"`
	FilePath  string    `json:"filePath,omitempty"`
	FileURL   string    `json:"fileUrl,omitempty"`
	Selection Selection `json:"selection"`
}

// Selection represents a text selection range.
type Selection struct {
	Start   Position `json:"start"`
	End     Position `json:"end"`
	IsEmpty bool     `json:"isEmpty,omitempty"`
}

// Position represents a position in a text document.
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}
