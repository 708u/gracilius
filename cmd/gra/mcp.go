package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/708u/gracilius/internal/comment"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type listCommentsInput struct {
	FilePath        string `json:"filePath,omitempty" jsonschema:"filter by file path"`
	IncludeResolved bool   `json:"includeResolved,omitempty" jsonschema:"include resolved comments"`
}

type commentIDInput struct {
	ID string `json:"id" jsonschema:"comment ID"`
}

func runMCP() int {
	rootDir := "."
	if len(os.Args) > 2 {
		rootDir = os.Args[2]
	}

	absRootDir, err := filepath.Abs(rootDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to resolve root directory: %v\n", err)
		return exitErr
	}

	store, err := comment.NewRepository(absRootDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create comment repository: %v\n", err)
		return exitErr
	}

	server := mcp.NewServer(
		&mcp.Implementation{Name: "gracilius", Version: "0.1.0"},
		nil,
	)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_comments",
		Description: "List review comments from gracilius TUI",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input listCommentsInput) (*mcp.CallToolResult, any, error) {
		comments, err := store.List(input.FilePath, input.IncludeResolved)
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
		log.Printf("MCP server error: %v", err)
		return exitErr
	}

	return exitOK
}
