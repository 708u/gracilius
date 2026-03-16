package main

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/708u/gracilius/internal/comment"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type listCommentsInput struct {
	FilePath        string `json:"filePath,omitempty" jsonschema:"filter by file path"`
	IncludeResolved bool   `json:"includeResolved,omitempty" jsonschema:"include resolved comments"`
	Kind            string `json:"kind,omitempty" jsonschema:"diff scope kind: working, branch, or review. If omitted, returns file comments only."`
	Base            string `json:"base,omitempty" jsonschema:"base branch name (for branch kind)"`
	SessionID       string `json:"sessionId,omitempty" jsonschema:"session UUID (for review kind)"`
}

type commentIDInput struct {
	ID string `json:"id" jsonschema:"comment ID"`
}

type McpCmd struct {
	pathArg
}

func (c *McpCmd) Run() error {
	absRootDir, err := filepath.Abs(c.Path)
	if err != nil {
		return fmt.Errorf("failed to resolve root directory: %w", err)
	}

	store, err := comment.NewRepository(absRootDir)
	if err != nil {
		return fmt.Errorf("failed to create comment repository: %w", err)
	}

	server := mcp.NewServer(
		&mcp.Implementation{Name: "gracilius", Version: "0.1.0"},
		nil,
	)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_comments",
		Description: "List review comments from gracilius TUI. Without kind, returns file comments. With kind, returns diff comments for that scope.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input listCommentsInput) (*mcp.CallToolResult, any, error) {
		var comments []comment.Entry
		var err error
		if input.Kind != "" {
			sc := comment.DiffScope{Kind: input.Kind, Base: input.Base, SessionID: input.SessionID}
			comments, err = store.ListByScope(sc, input.FilePath, input.IncludeResolved)
		} else {
			comments, err = store.List(input.FilePath, input.IncludeResolved)
		}
		if err != nil {
			return nil, nil, fmt.Errorf("list comments: %w", err)
		}
		data, err := json.Marshal(comments)
		if err != nil {
			return nil, nil, fmt.Errorf("marshal comments: %w", err)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: string(data)},
			},
		}, nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "resolve_comment",
		Description: "Mark a review comment as resolved",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input commentIDInput) (*mcp.CallToolResult, any, error) {
		if err := store.Resolve(input.ID); err != nil {
			return nil, nil, fmt.Errorf("resolve comment: %w", err)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Resolved comment %s", input.ID)},
			},
		}, nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "delete_comment",
		Description: "Delete a review comment",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input commentIDInput) (*mcp.CallToolResult, any, error) {
		if err := store.Delete(input.ID); err != nil {
			return nil, nil, fmt.Errorf("delete comment: %w", err)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Deleted comment %s", input.ID)},
			},
		}, nil, nil
	})

	ctx := context.Background()
	if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil {
		return fmt.Errorf("MCP server error: %w", err)
	}

	return nil
}
