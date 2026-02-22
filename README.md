# gracilius

A TUI code review viewer for
[Claude Code](https://docs.anthropic.com/en/docs/claude-code).

Connects to the Claude Code CLI via WebSocket/MCP and
provides a terminal-based interface for reviewing diffs.

> Work in progress.

## Build

```bash
go build -o gra ./cmd/gra/
```

## Usage

```bash
# Start gracilius
./gra

# Specify a project directory
./gra /path/to/project
```

In another terminal, start Claude Code pointing to the
same port:

```bash
CLAUDE_CODE_SSE_PORT=18765 claude
```
