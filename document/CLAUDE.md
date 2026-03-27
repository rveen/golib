# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Run tests
go test -v ./...

# Run a single test
go test -v -run TestPart

# Run benchmarks
go test -bench=. -benchmem

```

## Architecture

This package parses an extended Markdown dialect into an [OGDL](https://github.com/rveen/ogdl) graph, then renders that graph to HTML or structured data.

### Data flow

```
Markdown string
  → block() in markdown.go  (emits events via EventHandler)
  → ogdl.Graph              (built in document.go::New)
  → Html() / Data() / Part()
```

### Key types and files

- **`Document`** (`document.go`) — the public API. Contains `g *ogdl.Graph` (the parsed tree) and `parts *ogdl.Graph` (cached header sections). Main methods: `New`, `Html`, `HtmlWithLinks`, `Data`, `Structure`, `Part`, `CompareStructure`.
- **`markdown.go`** — block-level parser (`block()` dispatches on first char of each line) and inline element regex transforms (`inLine()`). Produces the event stream.
- **`html.go`** — walks the OGDL graph and renders each node type to HTML. `headerToHtml`, `listToHtml`, `tableToHtml`, `textToHtml`, `codeToHtml`.
- **`data.go`** — extracts structured data from the graph: headers become hierarchical keys, tables become key-value structures.
- **`normalize.go`** — Unicode normalization and kebab-case conversion for anchor IDs.
- **`escape.go`** — parses OGDL escape sequences embedded in markdown.

### Block node types in the OGDL graph

| Node tag | Markdown element |
|----------|-----------------|
| `!h`     | Header (sub-nodes: level, text, key, type) |
| `!p`     | Paragraph |
| `!q`     | Block quote |
| `!pre`   | Fenced code block |
| `!ul`    | Unordered list |
| `!ol`    | Ordered list |
| `!li`    | List item |
| `!tb`    | Table |
| `!var`   | Variable substitution |
| `!g`     | Embedded OGDL data block |

### Extended markdown syntax

- Headers: `# Title`, `## Sub {#anchor}`, `### Section {!type}`
- Lists: `-` for UL, `+` for OL; nested by indentation
- Tables: `|` rows, `|---|` separator, `||` for column-header variant
- Commands on their own line: `.csv [hrow|hcol|hboth]`, `.var EXPR`, `.bp`
- Inline OGDL data blocks delimited by `{` / `}`

### Notes

- There are two rendering styles in `html.go`: an older event-stream iteration approach and a newer OGDL graph traversal. The graph traversal approach is preferred for new work.
