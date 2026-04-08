# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Run all tests
go test ./...

# Run a single test
go test -run TestGetDir

# Build (no binary, library only)
go build ./...
```

Note: tests reference hardcoded paths under `/files/go/src/github.com/rveen/golib/fn/test/`, which is the expected working environment.

## Architecture

Package `fn` provides a unified path-navigation abstraction over heterogeneous file systems. The central type is `FNode`, which is populated by calling `Get`, `GetRaw`, or `GetMeta` with a path string.

### FNode lifecycle

1. Create with `New(root string)` (OS filesystem) or `NewFS(fs.FS)` (embedded/io.FS).
2. Call `fn.Get(path)` — this walks the path and populates `fn.Path`, `fn.Type`, `fn.Content`, `fn.Data`, `fn.Document`, and `fn.Params`.

### File types

| Type | Extension | Populated field |
|------|-----------|-----------------|
| `data` | `.ogdl` | `fn.Data` (`*ogdl.Graph`) |
| `document` | `.md` | `fn.Document` (`*document.Document`) |
| `file` | anything else | `fn.Content` (`[]byte`) |
| `dir` | directory | `fn.Data` (directory listing as ogdl graph) |
| `svn` | SVN repo dir | delegated to `svnGet` |

### Path navigation rules (get.go)

The path is split into slash-separated parts and consumed one by one:

- **Extension inference**: if a path segment has no extension and no exact match, the code tries `.html`, `.htm`, `.md`, `.ogdl` automatically.
- **`~` segment**: triggers tilde routing — looks for a `_tilde` entry in the current directory listing, stores remaining path in `fn.Params["tilde"]`.
- **`_token` / `_token_end` directories**: generic/parameterized path capture. A directory named `_foo` acts as a wildcard; the matched segment is stored in `fn.Params["foo"]`. Suffix `_end` captures the entire remaining path.
- **`_` segment inside a document path**: switches from document to data view (`fn.Document.Data()`).
- **Sub-path inside data/document**: remaining parts after the file are treated as dot-separated keys into the ogdl graph (`fn.Data.Get(...)`) or document section (`fn.Document.Part(...)`).
- **`index.*` / `readme.*`**: automatically selected when navigating into a directory.

### SVN support (svn.go)

When a directory is identified as an SVN repository (`dirType()` returns `"svn"`), navigation delegates to `svnGet`. It shells out to `svnlook` and `svn` CLI tools. Revision is specified with `@rev` anywhere in the path; `@` at the end requests a log listing.

### Key dependencies

- `github.com/rveen/ogdl` — graph/data format used for structured data files and directory listings.
- `github.com/rveen/golib/document` — Markdown document parser supporting section extraction (`Part`) and data extraction (`Data`).
- `github.com/rveen/golib/id` — used to detect unique IDs so extension inference is skipped for them.
- `github.com/rveen/ogdl/io/gxml` — XML-to-ogdl converter, used to parse SVN XML output.
