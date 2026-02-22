# Syntax Highlight Implementation

## Purpose

The editor pane in gracilius displays file contents as plain text,
resulting in poor code readability.
Add syntax highlighting using `github.com/alecthomas/chroma/v2`
to improve the code review experience.

## Changes

### Approach: Token-based rendering (chroma v2)

Use `github.com/alecthomas/chroma/v2` to tokenize files and
apply ANSI color codes to each token.

To coexist with existing cursor/selection inverse video display,
tokens are held as structs (`styledRun`) and split at
cursor/selection positions for rendering.

Reasons for not adopting other approaches:

- Pre-render + strip ANSI: ANSI code parsing and re-insertion is fragile
- Two-pass ANSI manipulation: Requires a complete ANSI parser, too complex

### 1. New type: `styledRun`

```go
type styledRun struct {
    Text string // Raw text (tabs unexpanded)
    ANSI string // SGR prefix (e.g., "\033[38;5;148m"), empty = ""
}
```

Each line is represented as `[]styledRun`.
The entire file is held in the Model as `[][]styledRun`.

- `Text` is the raw string (used for cursor/selection rune position calculation)
- `ANSI` is the SGR code pre-resolved from the chroma style
- Tabs are expanded with `expandTabs()` at render time

### 2. Model changes (`model.go`)

```go
type Model struct {
    // Existing fields...
    highlightedLines [][]styledRun // nil = no highlighting
}
```

### 3. Function layout in `highlight.go`

Create a new file `internal/tui/highlight.go`.
All functions are package-private.

#### `highlightFile(filePath, source string) [][]styledRun`

1. Detect language with `lexers.Match(filePath)`. If nil, use `lexers.Fallback`
2. Merge adjacent same-type tokens with `chroma.Coalesce(lexer)`
3. Get style with `styles.Get("monokai")`
4. Get token iterator with `lexer.Tokenise(nil, source)`
5. Get `[]chroma.Token` with `iterator.Tokens()`
6. For each token, resolve ANSI code with `resolveANSI(style, token.Type)`
7. Split token `Value` by `\n` and build `[]styledRun` per line
8. Return `[][]styledRun`. Return `nil` on error

Handling of multi-line tokens (multi-line strings, comments):

```go
parts := strings.Split(token.Value, "\n")
for i, part := range parts {
    if part != "" {
        currentLine = append(currentLine, styledRun{
            Text: part, ANSI: ansi,
        })
    }
    if i < len(parts)-1 {
        result = append(result, currentLine)
        currentLine = nil
    }
}
```

#### `resolveANSI(style *chroma.Style, tokenType chroma.TokenType) string`

1. Get `StyleEntry` with `style.Get(tokenType)`
2. If `entry.IsZero()`, return `""`
3. Build SGR parameter list:
   - `entry.Colour.IsSet()`: Convert RGB to 256-color, add `38;5;{idx}`
   - `entry.Bold == chroma.Yes`: Add `1`
   - `entry.Italic == chroma.Yes`: Add `3`
   - `entry.Underline == chroma.Yes`: Add `4`
4. If parameter list is empty, return `""`
5. Return `fmt.Sprintf("\033[%sm", strings.Join(params, ";"))`

#### `rgbTo256(r, g, b uint8) int`

RGB to 256-color palette index conversion.

1. Check if close to a grayscale value (colors 232-255)
2. Map to 6x6x6 color cube (colors 16-231):
   `16 + 36*(r/51) + 6*(g/51) + (b/51)`
3. Compare distances to cube color and grayscale color, choose the closer one

#### `renderStyledLine(sb *strings.Builder, runs []styledRun)`

Normal syntax highlight rendering (no cursor/selection).

For each run:
If `run.ANSI` is non-empty, output `run.ANSI + expandTabs(run.Text) + \033[0m`;
if empty, output only `expandTabs(run.Text)`.

#### `renderStyledLineWithCursor(sb *strings.Builder, runs []styledRun, cursorChar int)`

Syntax highlight + cursor display.

Track cumulative rune position while scanning runs.
When the cursor position falls within a run's range, split that run:

1. Text before cursor: Render with syntax color
2. Cursor character: Render with `\033[7m` (inverse video).
   For tabs, use 4 inverse spaces
3. Text after cursor: Render with syntax color

If cursor is past the end of all runs (EOL),
append `\033[7m \033[0m`.

#### `renderStyledLineWithSelection(sb *strings.Builder, runs []styledRun, selStart, selEnd int)`

Syntax highlight + selection range display.

Track cumulative rune position while scanning runs.
Determine overlap between each run and the selection range `[selStart, selEnd)`:

1. Text before selection start: Render with syntax color
2. Text within selection: Render with `\033[7m` (inverse video)
3. Text after selection end: Render with syntax color

Runs that span selection boundaries are split at the boundary positions.

#### `writeStyledText(sb *strings.Builder, ansi, text string)`

Helper function.
If `ansi` is non-empty, output `ansi + text + \033[0m`; if empty, output only `text`.
Used commonly by all rendering functions.

#### `getHighlightedRuns(lineIdx int) []styledRun`

Model method.
If `m.highlightedLines` exists and the index is in range, return the corresponding line;
otherwise return `nil` (triggers fallback path).

### 4. `fileio.go` changes

Add after `m.lines = strings.Split(...)` in `loadFile()`:

```go
m.highlightedLines = highlightFile(absPath, string(content))
```

For binary files, the early return means `highlightedLines` remains
`nil`.

### 5. `update.go` changes

Add after `m.lines = msg.lines` in the `fileChangedMsg` handler:

```go
m.highlightedLines = highlightFile(
    m.filePath, strings.Join(msg.lines, "\n"),
)
```

### 6. `view.go` changes

In each of the 4 rendering branches in `renderEditor()`,
use styled rendering functions if `getHighlightedRuns(i)` returns non-nil;
otherwise maintain existing fallback rendering.

Branch 1: Cursor line + selection (view.go L193-206)

```go
if runs := m.getHighlightedRuns(i); runs != nil {
    renderStyledLineWithSelection(&sb, runs, sc, ec)
} else {
    m.renderLineWithCursorAndSelection(&sb, lineContent, sc, ec)
}
```

Branch 2: Cursor line (view.go L207-209)

```go
if runs := m.getHighlightedRuns(i); runs != nil {
    renderStyledLineWithCursor(&sb, runs, m.cursorChar)
} else {
    m.renderLineWithCursor(&sb, lineContent)
}
```

Branch 3: Non-cursor selected line (view.go L210-231)

```go
if runs := m.getHighlightedRuns(i); runs != nil {
    renderStyledLineWithSelection(&sb, runs, sc, ec)
} else {
    // Existing code unchanged
}
```

Branch 4: Normal line (view.go L232-233)

```go
if runs := m.getHighlightedRuns(i); runs != nil {
    renderStyledLine(&sb, runs)
} else {
    sb.WriteString(expandTabs(lineContent))
}
```

No changes to diff/preview mode (view.go L157-176).
Maintain green/red coloring to preserve change visibility.

No changes to comment markers (view.go L236-238).
`[C]` is appended after line content and is independent.

Existing `renderLineWithCursor` and `renderLineWithCursorAndSelection`
are kept as fallbacks.

### 7. Caching Strategy

`highlightedLines` is recomputed only in these 2 places:

- `loadFile()`: On file load
- `fileChangedMsg`: On external file change

No recomputation on cursor movement, selection changes, scrolling, or resizing.
Highlighting cost is incurred only on file (re)load.

### 8. Edge Cases

- Binary files: Early return in `loadFile`,
  `highlightedLines` is `nil`, fallback rendering
- Unknown extensions: `lexers.Match` returns nil, `lexers.Fallback` is used.
  All tokens have `TokenType` of `Other` -> empty ANSI code -> plain display
- Empty files: Empty `[][]styledRun`, rendered as empty lines
- Line count mismatch: `getHighlightedRuns` returns `nil` for out-of-range,
  fallback path
- Large files: Tokenization is synchronous. A few ms for ~10000 lines.
  Comparable latency to the file read itself

### 9. Dependency Addition

```bash
go get github.com/alecthomas/chroma/v2
```

Sub-packages used:

- `github.com/alecthomas/chroma/v2`
  (Token, Colour, Style, StyleEntry, Coalesce)
- `github.com/alecthomas/chroma/v2/lexers`
  (Match, Fallback)
- `github.com/alecthomas/chroma/v2/styles`
  (Get)

## Target Files

- @internal/tui/highlight.go (new)
- @internal/tui/highlight_test.go (new)
- @internal/tui/model.go
- @internal/tui/fileio.go
- @internal/tui/update.go
- @internal/tui/view.go
- @go.mod
- @go.sum

## Implementation Order

1. `go get github.com/alecthomas/chroma/v2`
2. Create `internal/tui/highlight.go`
3. Add field to `internal/tui/model.go`
4. Add `highlightFile` call to `internal/tui/fileio.go`
5. Add `highlightFile` call to `internal/tui/update.go`
6. Integrate styled rendering into `internal/tui/view.go`
7. Create `internal/tui/highlight_test.go`
8. Build and manual verification

## Completion Criteria

- [ ] Add chroma v2 dependency
- [ ] Implement `highlight.go`
  - [ ] `styledRun` type definition
  - [ ] `highlightFile` - Tokenization and line splitting
  - [ ] `resolveANSI` - Style to ANSI conversion
  - [ ] `rgbTo256` - RGB to 256-color conversion
  - [ ] `renderStyledLine` - Normal rendering
  - [ ] `renderStyledLineWithCursor` - Rendering with cursor
  - [ ] `renderStyledLineWithSelection` - Rendering with selection range
  - [ ] `writeStyledText` - Helper
  - [ ] `getHighlightedRuns` - Cache accessor
- [ ] Add `highlightedLines` field to `model.go`
- [ ] Call highlight in `loadFile()` in `fileio.go`
- [ ] Recompute highlight on `fileChangedMsg` in `update.go`
- [ ] Integrate styled rendering into the 4 branches in `view.go`
- [ ] Create and pass tests in `highlight_test.go`
  - [ ] `TestHighlightFile` - Go source tokenization verification
  - [ ] `TestRgbTo256` - RGB to 256-color conversion table test
  - [ ] `TestRenderStyledLine` - ANSI output verification
  - [ ] `TestRenderStyledLineWithCursor` - Inverse video verification
  - [ ] `TestRenderStyledLineWithSelection` - Selection range verification
  - [ ] `TestHighlightFileUnknownExtension` - Fallback verification
  - [ ] `TestHighlightFileMultilineToken` - Multi-line token verification
- [ ] `go test ./...` passes
- [ ] `go build -o gra ./cmd/gra/` passes
- [ ] Verify highlight display for various files (Go, JSON, YAML, etc.)
- [ ] Verify cursor movement and selection work correctly
- [ ] Verify diff preview mode works as before
- [ ] Verify unknown extension files display as plain text

## Verification Method

```bash
go test ./internal/tui/...
go build -o gra ./cmd/gra/
./gra .
```

- Open Go, JSON, YAML, Markdown, etc. files and verify highlighting
- Verify cursor movement and text selection work correctly
- Verify diff preview mode works as before
- Verify unknown extension files display as plain text
