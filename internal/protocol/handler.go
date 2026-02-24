package protocol

import (
	"encoding/json"
	"net/url"
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

// Capabilities describes server capabilities (empty for MCP compatibility).
type Capabilities struct{}

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
type OpenDiffCallback func(filePath string, contents string)

// CloseTabCallback is called when close_tab is received.
type CloseTabCallback func()

// IdeConnectedCallback is called when ide_connected is received.
type IdeConnectedCallback func()

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

// Handler processes JSON-RPC messages.
type Handler struct {
	workspaceFolders []string
	onOpenDiff       OpenDiffCallback
	onCloseTab       CloseTabCallback
	onIdeConnected   IdeConnectedCallback
}

// NewHandler creates a new Handler.
func NewHandler(workspaceFolders []string) *Handler {
	return &Handler{
		workspaceFolders: workspaceFolders,
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

// HandleMessage processes a JSON-RPC request and returns a response.
func (h *Handler) HandleMessage(req *Request) *Response {
	switch req.Method {
	case "initialize":
		return h.handleInitialize(req)
	case "tools/call":
		return h.handleToolsCall(req)
	case "notifications/initialized":
		// Received initialized notification from client (no response needed)
		return nil
	case "prompts/list":
		// MCP prompts/list - return an empty list
		return NewResponse(req.ID, map[string][]any{"prompts": {}})
	case "tools/list":
		// MCP tools/list - return tool list
		return h.handleToolsList(req)
	case "ide_connected":
		// Claude Code notified that connection is established
		if h.onIdeConnected != nil {
			h.onIdeConnected()
		}
		return nil
	default:
		// If id is present, return a "method not found" error
		// If id is absent, it is a notification so no response is needed
		if len(req.ID) > 0 {
			return NewErrorResponse(req.ID, codeMethodNotFound, "Method not found: "+req.Method)
		}
		return nil
	}
}

func (h *Handler) handleInitialize(req *Request) *Response {
	var params InitializeParams
	if req.Params != nil {
		json.Unmarshal(req.Params, &params) //nolint:errcheck // use default values on parse failure
	}

	// Default protocol version if not provided
	protocolVersion := params.ProtocolVersion
	if protocolVersion == "" {
		protocolVersion = defaultProtocolVersion
	}

	result := InitializeResult{
		ProtocolVersion: protocolVersion,
		Capabilities:    Capabilities{},
		ServerInfo: ServerInfo{
			Name:    "gracilius",
			Version: "0.1.0",
		},
	}
	return NewResponse(req.ID, result)
}

func (h *Handler) handleToolsList(req *Request) *Response {
	// MCP-compliant tool list
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
	}
	return NewResponse(req.ID, map[string]any{"tools": tools})
}

// fileURI converts an absolute file path to a file:// URI using net/url.
func fileURI(path string) string {
	return (&url.URL{Scheme: "file", Path: path}).String()
}

func (h *Handler) handleToolsCall(req *Request) *Response {
	var params ToolCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, codeInvalidParams, "Invalid params")
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
				URI:  fileURI(path),
				Name: path,
				Path: path,
			}
		}
		// MCP-compliant response format
		result := WorkspaceFoldersResult{
			Success:  true,
			Folders:  folders,
			RootPath: rootPath,
		}
		resultJSON, _ := json.Marshal(result)
		return NewResponse(req.ID, NewMCPResult(string(resultJSON)))
	case "openDiff":
		var args OpenDiffArgs
		if err := json.Unmarshal(params.Arguments, &args); err != nil {
			return NewErrorResponse(req.ID, codeInvalidParams, "Invalid openDiff arguments")
		}
		if h.onOpenDiff != nil {
			h.onOpenDiff(args.NewFilePath, args.NewFileContents)
		}
		return NewResponse(req.ID, NewMCPResult("DIFF_SHOWN"))
	case "getDiagnostics":
		// MCP-compliant: return an empty content array
		return NewResponse(req.ID, NewMCPResultEmpty())
	case "closeAllDiffTabs":
		if h.onCloseTab != nil {
			h.onCloseTab()
		}
		return NewResponse(req.ID, NewMCPResult("CLOSED_DIFF_TABS"))
	case "close_tab":
		// close_tab is called on cancel, so clear the preview
		if h.onCloseTab != nil {
			h.onCloseTab()
		}
		return NewResponse(req.ID, NewMCPResult("TAB_CLOSED"))
	default:
		return NewErrorResponse(req.ID, codeMethodNotFound, "Method not found: "+params.Name)
	}
}
