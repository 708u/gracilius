# Claude Code Integration TUI Tool Plan

## Goal

Create a TUI tool for quickly providing feedback on implemented code
during a workflow where implementation is done with Claude Code.

### What We Want to Achieve

- Quickly browse code with a file tree and viewer
- Automatically notify Claude Code of selected sections
- Visually confirm git diff
- Communicate partial approval/rejection of changes to Claude Code
- Run in parallel with the terminal (assuming tmux/zellij)

### What We Will Not Achieve

- File editing (editing is left to Claude Code)
- LSP integration (diagnostic information, etc.)
- Rich content such as image display

---

## Technical Specification: Claude Code Integration Protocol

Based on analysis results of the VS Code extension.

### Communication Architecture

```txt
┌─────────────┐    WebSocket    ┌─────────────┐
│  TUI Tool   │◄──────────────►│ Claude Code │
│  (Server)   │    MCP/JSONRPC  │   (Client)  │
└─────────────┘                 └─────────────┘
```

### Discovery: Lock File

Claude Code scans lock files under `~/.claude/ide/` at startup
to discover connection targets.

**File path**: `~/.claude/ide/{port}.lock`

**Contents**:

```json
{
  "pid": 12345,
  "workspaceFolders": ["/path/to/project"],
  "ideName": "TUI Tool Name",
  "transport": "ws",
  "runningInWindows": false,
  "authToken": "uuid-v4-token"
}
```

### Authentication

The following header is sent during WebSocket connection:

```txt
x-claude-code-ide-authorization: {authToken}
```

### MCP Handshake

After connection, Claude Code sends an `initialize` request.

**Request**:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "initialize",
  "params": {
    "protocolVersion": "2025-11-25",
    "capabilities": {},
    "clientInfo": { "name": "claude-code", "version": "1.0.0" }
  }
}
```

**Response**:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "protocolVersion": "2025-11-25",
    "capabilities": {},
    "serverInfo": { "name": "tui-tool", "version": "1.0.0" }
  }
}
```

### IDE to Claude Code: Event Notifications

#### selection_changed

Sent when the user selects code.

```json
{
  "jsonrpc": "2.0",
  "method": "selection_changed",
  "params": {
    "selection": {
      "start": { "line": 10, "character": 0 },
      "end": { "line": 15, "character": 20 }
    },
    "text": "selected text",
    "filePath": "/absolute/path/to/file.go"
  }
}
```

### Claude Code to IDE: Tool Calls

List of tools provided by the VS Code extension, and
support status in this TUI:

| Tool | VS Code | TUI Support | Notes |
|------|---------|-------------|-------|
| openFile | Yes | Phase 2 | Open specified file |
| getDiagnostics | Yes | Not supported | LSP not needed |
| getCurrentSelection | Yes | Phase 2 | Return current selection |
| getOpenEditors | Yes | Phase 2 | List open tabs |
| getWorkspaceFolders | Yes | Phase 1 | Workspace path |
| openDiff | Yes | Phase 3 | Open diff view |
| closeAllDiffTabs | Yes | Not supported | - |
| checkDocumentDirty | Yes | Not supported | No editing |
| saveDocument | Yes | Not supported | No editing |
| executeCode | Yes | Not supported | Jupyter not needed |

---

## Design

### Technology Stack

- **Language**: Go
- **TUI Framework**: Bubbletea
- **Syntax Highlighting**: chroma
- **Git operations**: git command invocation
- **WebSocket**: gorilla/websocket

### Screen Layout

```txt
┌─[file1.go]─[file2.go]─────────────────────────────────────────┐
│                                                                │
│ ┌─Tree──────┐ ┌─Viewer─────────────────────────────────────┐  │
│ │ /src      │ │  10 │ func main() {                        │  │
│ │   main.go │ │  11 │     server := NewServer()            │  │
│ │   app/    │ │> 12 │     server.Run()  <- selected line   │  │
│ │     ...   │ │  13 │ }                                    │  │
│ └───────────┘ └─────────────────────────────────────────────┘  │
│                                                                │
│ ┌─Git Status / Diff────────────────────────────────────────┐  │
│ │ M main.go  (+10, -5)                                     │  │
│ │ A new.go   (+50)                                         │  │
│ └──────────────────────────────────────────────────────────┘  │
│                                                                │
│ [Status: Connected to Claude Code] [Workspace: /path/to/proj] │
└────────────────────────────────────────────────────────────────┘
```

### Layout Configuration

Since it depends on the user's environment (display size, font),
it should be flexibly configurable:

- Number of pane splits: user setting
- wrap: on/off, width specification
- Only provide minimum width warning

### Key Binding Proposal

| Key | Action |
|-----|--------|
| `j/k` | Cursor movement |
| `h/l` | Pane movement |
| `Enter` | Open file (new tab) |
| `o` | Open file (current tab) |
| `q` | Close tab |
| `gt/gT` | Tab switching |
| `v` | Start selection mode |
| `y` | Send selection to Claude Code |
| `d` | Show diff view |
| `?` | Help |

---

## Implementation Phases

### Phase 1: MVP

Minimum viable verification.

- File tree display
- File viewer (syntax highlighting)
- Line selection -> Send `selection_changed` to Claude Code
- Lock file creation, WebSocket server startup
- MCP handshake support

### Phase 2: Bidirectional Communication

Accept operations from Claude Code.

- Implement `openFile` tool
- Implement `getCurrentSelection` tool
- Implement `getOpenEditors` tool
- Tab functionality (open multiple files)

### Phase 3: Git Integration

Change visualization and feedback.

- git status display
- git diff display (hunk-level)
- Jump to changed files
- Real-time file change detection (fsnotify)

### Phase 4: Enhanced Feedback

Optimize the review workflow.

- Hunk-level approval/rejection marking
- Send mark information to Claude Code
- Multiple range selection
- Selection history/bookmarks

### Phase 5: Configuration and Extensions

Improve usability.

- Configuration file support
- Custom key bindings
- Color schemes
- Exclusion pattern settings

---

## Verification Methods

### Phase 1 Verification

1. Start the TUI tool
2. Start `CLAUDE_CODE_SSE_PORT={port} claude` in another terminal
3. Select a file in the TUI
4. Confirm that the selection is reflected in Claude Code's
   system-reminder

### Ongoing Verification

- Use in actual Claude Code development workflow
- Evaluate feedback loop time reduction by feel

---

## References

- Existing sample implementation: `tools/simple-ide-mcp/`
  (worktree: `708u/feat-simple-ide-mcp-sample`)
- VS Code extension analysis results:
  `docs/sessions/2026-01-28-claude-code-ide-integration-analysis.md`

---

## Open Items

- Tool name
- Repository location (within path or independent repository)
- Distribution method (go install, Homebrew, binary release)
