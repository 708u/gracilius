# CLAUDE.md

## Project Overview

gracilius is a TUI viewer for reviewing and providing feedback
on code produced by Claude Code.
It embeds a WebSocket server and communicates with the
Claude Code CLI via MCP (Model Context Protocol).

The command name is `gra`.

## Development Commands

```bash
# Build
go build -o gra ./cmd/gra/

# Run
go run ./cmd/gra/

# Run (specify directory)
go run ./cmd/gra/ /path/to/project

# Test
go test ./...
```

Connection test with Claude Code:

```bash
# Start Claude Code in another terminal
CLAUDE_CODE_SSE_PORT=18765 claude
```

## Architecture

### Overall Structure

```txt
┌─────────────┐   WebSocket   ┌──────────────┐
│ gracilius   │◄─────────────►│  Claude Code  │
│ (WS Server) │  MCP/JSONRPC  │   (Client)    │
└──────┬──────┘               └──────────────┘
       │
  Bubbletea TUI
  (Elm Architecture)
```

gracilius acts as the WebSocket server;
Claude Code acts as the client.

### Package Layout

```txt
cmd/gra/
  main.go          Entry point, wiring, callback registration

internal/
  config/
    config.go      DataDir() for data directory path
  tui/
    model.go       MCPServer interface, Model struct,
                   message types, NewModel
    update.go      Init(), Update(), helpers
    view.go        View(), renderTree, renderEditor,
                   renderLineWith*
    filetree.go    fileEntry, buildFileTree, scanDir,
                   expandDir, collapseDir,
                   WatchDirRecursive
    diff.go        diffLine, computeLineDiff
    display.go     displayWidth, isWideRune,
                   truncateString, padRight, expandTabs
    notify.go      notifySelectionChanged,
                   notifyClearSelection, notifyComment
    watch.go       watchFile, watchDir
    fileio.go      loadFile, isBinary
  protocol/
    jsonrpc.go     JSON-RPC 2.0 type definitions
    handler.go     MCP method handler
  server/
    server.go      WebSocket server, client management
    lockfile.go    Lock file management
```

### Dependency Graph

```txt
cmd/gra → internal/config   (DataDir)
cmd/gra → internal/tui      (Model, message types)
cmd/gra → internal/server   (Server creation, callbacks)
       internal/server  → internal/config
       internal/server  → internal/protocol
       internal/comment → internal/config
```

`tui` has zero dependencies on other internal packages.
There is no direct dependency between `tui` and `server`.

### cmd/gra/main.go

Responsible only for the entry point and wiring.

Startup flow:

1. Create log file (`gracilius.log`)
2. Get target directory from args (default `.`)
3. Signal handling (SIGINT/SIGTERM)
4. Create WebSocket server with `server.New()`,
   start with `StartAsync()`
5. Create two `fsnotify.Watcher` instances
   (file watcher / directory watcher)
6. Watch root directory recursively
   via `tui.WatchDirRecursive()`
7. Start goroutine to call `srv.Stop()` on ctx.Done()
8. Create TUI model with `tui.NewModel()`
9. Create `tea.NewProgram` (alt screen + mouse)
10. Register server callbacks to bridge events to TUI
    (openDiff, closeTab, ideConnected)
11. Run the TUI program; stop server on exit

### internal/tui

All TUI code lives in this package.
Defines the `MCPServer` interface so it does not directly
import the `server` package (`server.Server` satisfies it
via structural typing).

The TUI has a two-pane layout (file tree | editor) and
supports mouse interactions (click, drag, pane resize,
scroll).

### internal/protocol

Defines JSON-RPC 2.0 base types
(Request, Response, Notification, Error)
and dispatches MCP methods.

Supported methods:

| Method | Kind | Description |
| --- | --- | --- |
| `initialize` | Request | MCP handshake |
| `notifications/initialized` | Notification | Init complete |
| `tools/list` | Request | List tools |
| `tools/call` | Request | Execute a tool |
| `prompts/list` | Request | List prompts (empty) |
| `ide_connected` | Notification | Connection established |

Implemented tools:

| Tool | Listed in tools/list | Description |
| --- | :-: | --- |
| `getWorkspaceFolders` | o | Workspace folder list |
| `openDiff` | o | Show diff view |
| `getDiagnostics` | o | Diagnostics (stub) |
| `closeAllDiffTabs` | x | Close all diff tabs |
| `close_tab` | x | Close tab |

All 5 tools defined in the MCP Server spec are implemented.

### internal/server

WebSocket server based on `gorilla/websocket`.

- Bind: `127.0.0.1:{port}` (default 18765)
- Port retry: up to 10 attempts, incrementing port
- Auth: `x-claude-code-ide-authorization` header
- Auth token: persisted at `~/.config/gracilius/token` (UUID v4)
- Keepalive: 30s ping / 60s timeout
- Selection notification debounce: 100ms

Lock file:

- Path: `~/.claude/ide/{port}.lock`
- Atomic write (.tmp then rename)
- Duplicate workspace detection (with process liveness check)
- Removed on `Stop()`

### Key Dependencies

| Library | Purpose |
| --- | --- |
| `charmbracelet/bubbletea` | TUI framework |
| `charmbracelet/lipgloss` | TUI styling |
| `gorilla/websocket` | WebSocket communication |
| `fsnotify/fsnotify` | File change watching |
| `google/uuid` | Auth token generation |
| `sergi/go-diff` | Line-level diff computation |

## User Instructions

@.claude/user_instructions/index.md

Place personal markdown files in this directory for local instructions.
These files are gitignored and will not be committed to the repository.
Instructions in this directory take highest priority
over other project instructions.
