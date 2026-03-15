package protocol

import "encoding/json"

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
