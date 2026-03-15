package protocol

import (
	"encoding/json"
	"sync"
)

// Handler processes JSON-RPC messages.
type Handler struct {
	workspaceFolders []string
	onOpenDiff       OpenDiffCallback
	onCloseTab       CloseTabCallback
	onIdeConnected   IdeConnectedCallback
	onGetSelection   GetSelectionCallback

	pendingDiffs map[string]*DiffResponder
	diffMu       sync.Mutex
}

// NewHandler creates a new Handler.
func NewHandler(workspaceFolders []string) *Handler {
	return &Handler{
		workspaceFolders: workspaceFolders,
		pendingDiffs:     make(map[string]*DiffResponder),
	}
}

// SetOpenDiffCallback sets the callback for openDiff events.
func (h *Handler) SetOpenDiffCallback(cb OpenDiffCallback) {
	h.onOpenDiff = cb
}

// SetCloseTabCallback sets the callback for close_tab events.
func (h *Handler) SetCloseTabCallback(cb CloseTabCallback) {
	h.onCloseTab = cb
}

// SetIdeConnectedCallback sets the callback for ide_connected events.
func (h *Handler) SetIdeConnectedCallback(cb IdeConnectedCallback) {
	h.onIdeConnected = cb
}

// SetGetSelectionCallback sets the callback for getCurrentSelection/getLatestSelection.
func (h *Handler) SetGetSelectionCallback(cb GetSelectionCallback) {
	h.onGetSelection = cb
}

// RejectAllPendingDiffs rejects all pending diff responses.
func (h *Handler) RejectAllPendingDiffs() {
	h.diffMu.Lock()
	pending := h.pendingDiffs
	h.pendingDiffs = make(map[string]*DiffResponder)
	h.diffMu.Unlock()

	for _, r := range pending {
		r.Reject()
	}
}

// HandleMessage processes a JSON-RPC request.
// send is called with the response when it is ready.
// For blocking operations (openDiff), send is called asynchronously.
func (h *Handler) HandleMessage(req *Request, send func(*Response)) {
	switch req.Method {
	case "initialize":
		send(h.handleInitialize(req))
	case "tools/call":
		h.handleToolsCall(req, send)
	case "notifications/initialized":
		// no response
	case "prompts/list":
		send(NewResponse(req.ID, map[string][]any{"prompts": {}}))
	case "tools/list":
		send(h.handleToolsList(req))
	case "ide_connected":
		if h.onIdeConnected != nil {
			h.onIdeConnected()
		}
	default:
		if len(req.ID) > 0 {
			send(NewErrorResponse(req.ID, codeMethodNotFound, "Method not found: "+req.Method))
		}
	}
}

func (h *Handler) handleInitialize(req *Request) *Response {
	var params InitializeParams
	if req.Params != nil {
		json.Unmarshal(req.Params, &params) //nolint:errcheck // use default values on parse failure
	}

	protocolVersion := params.ProtocolVersion
	if protocolVersion == "" {
		protocolVersion = defaultProtocolVersion
	}

	result := InitializeResult{
		ProtocolVersion: protocolVersion,
		Capabilities: Capabilities{
			Tools: &ToolsCapability{},
		},
		ServerInfo: ServerInfo{
			Name:    "gracilius",
			Version: "0.1.0",
		},
	}
	return NewResponse(req.ID, result)
}
