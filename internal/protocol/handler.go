package protocol

import (
	"encoding/json"
	"net/url"
	"sync"
)

const defaultProtocolVersion = "2025-11-25"

type toolDefinition struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema inputSchema `json:"inputSchema"`
}

type inputSchema struct {
	Type       string                    `json:"type"`
	Properties map[string]propertySchema `json:"properties"`
	Required   []string                  `json:"required,omitempty"`
}

type propertySchema struct {
	Type string `json:"type"`
}

// ClientInfo describes the connecting client.
type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// InitializeParams represents parameters for initialize request.
type InitializeParams struct {
	ProtocolVersion string          `json:"protocolVersion"`
	Capabilities    json.RawMessage `json:"capabilities,omitempty"`
	ClientInfo      *ClientInfo     `json:"clientInfo,omitempty"`
}

// InitializeResult is the response to an initialize request.
type InitializeResult struct {
	ProtocolVersion string       `json:"protocolVersion"`
	Capabilities    Capabilities `json:"capabilities"`
	ServerInfo      ServerInfo   `json:"serverInfo"`
}

// ToolsCapability signals that the server supports tools.
type ToolsCapability struct{}

// Capabilities describes server capabilities.
type Capabilities struct {
	Tools *ToolsCapability `json:"tools,omitempty"`
}

// ServerInfo describes the server.
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ToolCallParams represents parameters for tools/call request.
type ToolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

// WorkspaceFolder represents a workspace folder.
type WorkspaceFolder struct {
	URI  string `json:"uri"`
	Name string `json:"name"`
	Path string `json:"path"`
}

// WorkspaceFoldersResult represents the result of getWorkspaceFolders.
type WorkspaceFoldersResult struct {
	Success  bool              `json:"success"`
	Folders  []WorkspaceFolder `json:"folders"`
	RootPath string            `json:"rootPath"`
}

// OpenDiffArgs represents arguments for openDiff tool.
type OpenDiffArgs struct {
	OldFilePath     string `json:"old_file_path"`
	NewFilePath     string `json:"new_file_path"`
	NewFileContents string `json:"new_file_contents"`
	TabName         string `json:"tab_name"`
}

// OpenDiffCallback is called when openDiff is received.
// accept and reject are bound to the DiffResponder's methods.
type OpenDiffCallback func(filePath, contents, tabName string, accept func(string), reject func())

// CloseTabCallback is called when close_tab is received.
type CloseTabCallback func()

// IdeConnectedCallback is called when ide_connected is received.
type IdeConnectedCallback func()

// GetSelectionCallback returns the current selection state.
// Returns nil when no selection is available.
type GetSelectionCallback func() *SelectionResult

// SelectionResult represents the result of getCurrentSelection / getLatestSelection.
type SelectionResult struct {
	Success   bool       `json:"success"`
	Text      string     `json:"text,omitempty"`
	FilePath  string     `json:"filePath,omitempty"`
	FileURL   string     `json:"fileUrl,omitempty"`
	Selection *Selection `json:"selection,omitempty"`
	Message   string     `json:"message,omitempty"`
}

// MCPContent represents a content item in MCP response.
type MCPContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// MCPResult represents an MCP-compliant tools/call result.
type MCPResult struct {
	Content []MCPContent `json:"content"`
}

// NewMCPResult creates an MCP-compliant result with a single text content.
func NewMCPResult(text string) MCPResult {
	return MCPResult{
		Content: []MCPContent{
			{Type: "text", Text: text},
		},
	}
}

// NewMCPResultEmpty creates an MCP-compliant result with empty content.
func NewMCPResultEmpty() MCPResult {
	return MCPResult{
		Content: []MCPContent{},
	}
}

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

func (h *Handler) handleToolsList(req *Request) *Response {
	tools := []toolDefinition{
		{
			Name:        "getWorkspaceFolders",
			Description: "Get the workspace folders",
			InputSchema: inputSchema{
				Type:       "object",
				Properties: map[string]propertySchema{},
			},
		},
		{
			Name:        "openDiff",
			Description: "Open a diff view",
			InputSchema: inputSchema{
				Type: "object",
				Properties: map[string]propertySchema{
					"old_file_path":     {Type: "string"},
					"new_file_path":     {Type: "string"},
					"new_file_contents": {Type: "string"},
					"tab_name":          {Type: "string"},
				},
				Required: []string{"old_file_path", "new_file_path", "new_file_contents"},
			},
		},
		{
			Name:        "getDiagnostics",
			Description: "Get diagnostics for the workspace",
			InputSchema: inputSchema{
				Type:       "object",
				Properties: map[string]propertySchema{},
			},
		},
		{
			Name:        "getCurrentSelection",
			Description: "Get the current selection in the active editor",
			InputSchema: inputSchema{
				Type:       "object",
				Properties: map[string]propertySchema{},
			},
		},
		{
			Name:        "getLatestSelection",
			Description: "Get the latest selection from any editor",
			InputSchema: inputSchema{
				Type:       "object",
				Properties: map[string]propertySchema{},
			},
		},
	}
	return NewResponse(req.ID, map[string]any{"tools": tools})
}

// FileURI converts an absolute file path to a file:// URI using net/url.
func FileURI(path string) string {
	return (&url.URL{Scheme: "file", Path: path}).String()
}

func (h *Handler) handleGetSelection(fallbackMsg string) MCPResult {
	var result *SelectionResult
	if h.onGetSelection != nil {
		result = h.onGetSelection()
	}
	if result == nil {
		result = &SelectionResult{Message: fallbackMsg}
	}
	resultJSON, _ := json.Marshal(result)
	return NewMCPResult(string(resultJSON))
}

func (h *Handler) handleToolsCall(req *Request, send func(*Response)) {
	var params ToolCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		send(NewErrorResponse(req.ID, codeInvalidParams, "Invalid params"))
		return
	}

	switch params.Name {
	case "getWorkspaceFolders":
		folders := make([]WorkspaceFolder, len(h.workspaceFolders))
		rootPath := ""
		for i, path := range h.workspaceFolders {
			if i == 0 {
				rootPath = path
			}
			folders[i] = WorkspaceFolder{
				URI:  FileURI(path),
				Name: path,
				Path: path,
			}
		}
		result := WorkspaceFoldersResult{
			Success:  true,
			Folders:  folders,
			RootPath: rootPath,
		}
		resultJSON, _ := json.Marshal(result)
		send(NewResponse(req.ID, NewMCPResult(string(resultJSON))))
	case "openDiff":
		// Unlike other tools, openDiff does not call send here.
		// send is stored in DiffResponder and called later when the
		// user accepts or rejects the diff in the TUI. This blocks
		// Claude Code until the user makes a decision.
		var args OpenDiffArgs
		if err := json.Unmarshal(params.Arguments, &args); err != nil {
			send(NewErrorResponse(req.ID, codeInvalidParams, "Invalid openDiff arguments"))
			return
		}

		idKey := string(req.ID)
		responder := &DiffResponder{
			send:    send,
			id:      req.ID,
			tabName: args.TabName,
			cleanup: func() {
				h.diffMu.Lock()
				delete(h.pendingDiffs, idKey)
				h.diffMu.Unlock()
			},
		}

		h.diffMu.Lock()
		h.pendingDiffs[idKey] = responder
		h.diffMu.Unlock()

		if h.onOpenDiff != nil {
			h.onOpenDiff(args.NewFilePath, args.NewFileContents, args.TabName, responder.Accept, responder.Reject)
		} else {
			responder.Reject()
		}
	case "getDiagnostics":
		send(NewResponse(req.ID, NewMCPResultEmpty()))
	case "getCurrentSelection":
		send(NewResponse(req.ID, h.handleGetSelection("No active editor found")))
	case "getLatestSelection":
		send(NewResponse(req.ID, h.handleGetSelection("No selection available")))
	case "closeAllDiffTabs":
		h.RejectAllPendingDiffs()
		if h.onCloseTab != nil {
			h.onCloseTab()
		}
		send(NewResponse(req.ID, NewMCPResult("CLOSED_DIFF_TABS")))
	case "close_tab":
		h.RejectAllPendingDiffs()
		if h.onCloseTab != nil {
			h.onCloseTab()
		}
		send(NewResponse(req.ID, NewMCPResult("TAB_CLOSED")))
	default:
		send(NewErrorResponse(req.ID, codeMethodNotFound, "Method not found: "+params.Name))
	}
}
