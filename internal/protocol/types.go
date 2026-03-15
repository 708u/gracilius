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

// FileURI converts an absolute file path to a file:// URI using net/url.
func FileURI(path string) string {
	return (&url.URL{Scheme: "file", Path: path}).String()
}
