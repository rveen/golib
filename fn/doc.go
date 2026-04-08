// Package fn provides a unified path-navigation abstraction over heterogeneous
// file systems. Its central premise is that a URL-style path can address not
// only files and directories on the local OS filesystem, but also structured
// data inside files, named sections inside documents, and versioned content
// inside SVN repositories — all through a single, consistent API.
//
// # Rationale
//
// Web applications that serve file-backed content routinely need to resolve a
// request path to one of several things: a binary asset, a Markdown page, a
// structured data record, or a historical revision of any of the above. Without
// a unifying layer each concern requires its own resolution logic, leading to
// scattered path handling spread across the application.
//
// Package fn collapses that complexity into one call. The caller creates an
// [FNode] rooted at a directory (or embedded [fs.FS]) and calls [FNode.Get]
// with a path string. The package walks the filesystem, detects the type of
// each segment, and populates the node with whatever is appropriate for that
// type — raw bytes, a parsed data graph, a structured document, or a directory
// listing.
//
// # The FNode
//
// [FNode] is the single type in this package. Before a call to [FNode.Get] only
// Root (for OS paths) or RootFs (for embedded filesystems) needs to be set.
// After the call the following fields are populated according to the resolved
// type:
//
//   - Type "file"     — Content holds the raw bytes of the file.
//   - Type "data"     — Data holds the parsed [ogdl.Graph] of an .ogdl file.
//   - Type "document" — Document holds the parsed [document.Document] of a
//     .md file; sub-paths address named sections within it.
//   - Type "dir"      — Data holds a graph describing the directory listing
//     (name, type, size, modification time for each entry).
//
// Path, Type, and Params are always set after a successful Get, regardless of
// the resolved type. Params captures any named wildcards matched during
// navigation (see Generic segments below).
//
// # Path navigation
//
// The path is split on "/" and each segment is resolved left to right against
// the filesystem. Several special behaviours apply:
//
// Extension inference
//
// If a segment has no extension and does not exist as-is, the package probes
// for .html, .htm, .md, and .ogdl suffixes in that order, resolving to the
// first match. This allows clean URLs to address content files without exposing
// extensions to callers.
//
// Unique IDs
//
// Segments that look like unique IDs (as determined by [id.IsUniqueID]) are
// never subjected to extension inference; they are treated as a definitive
// not-found.
//
// Index files
//
// When navigation lands on a directory, the package looks for an index.* or
// readme.* file inside it and, if found, treats it as the target of the path.
// This mirrors the convention used by HTTP servers.
//
// Sub-path navigation into structured files
//
// Once a .ogdl data file is reached, remaining path segments are joined with
// "." and passed to the graph's Get method, allowing callers to address
// individual nodes within the file:
//
//	fn.Get("config.ogdl/database/host")  // returns the "database.host" node
//
// For .md document files, remaining segments address named sections (as parsed
// by the document package):
//
//	fn.Get("manual.md/introduction")     // returns the "introduction" section
//
// Appending "/_" to a document path switches to the data view of that document
// (the structured metadata embedded in the Markdown front-matter or headers):
//
//	fn.Get("manual.md/_")                // returns document.Data()
//
// Generic (wildcard) segments
//
// A directory whose name starts with "_" acts as a named wildcard. When a path
// segment does not match any real entry, the package checks for a "_name"
// directory. If found, the actual segment value is captured into
// FNode.Params["name"] and navigation continues inside that directory.
//
// A directory named "_name_end" captures the entire remaining path (not just
// the current segment) into Params["name"], consuming all further segments.
// This is useful for routing patterns such as user profiles or catch-all
// handlers.
//
// Tilde routing
//
// The special segment "~" triggers tilde routing. If the current directory
// contains an entry named "_tilde", navigation is redirected into that
// directory and the rest of the path (after the "~") is stored in
// Params["tilde"]. This provides a conventional mechanism for user-scoped
// paths (e.g. /~username/page).
//
// # SVN repositories
//
// When navigation encounters a bare SVN repository directory, it seamlessly
// delegates to SVN-aware logic using the svnlook and svn command-line tools.
// The same structured navigation rules (data sub-paths, document sections,
// index files, directory listings) apply to content inside SVN.
//
// Revisions are specified inline in the path with "@":
//
//	fn.Get("repo/trunk/file.md@42")   // revision 42
//	fn.Get("repo/trunk/file.md@")     // list all revisions (log)
//
// File content is size-checked before loading to prevent out-of-memory
// conditions for large binary assets (limit: 100 MB).
//
// # Filesystem backends
//
// Two backends are supported:
//
//   - OS filesystem: created with [New](root), where root is an absolute path.
//   - Embedded filesystem: created with [NewFS](fsys), where fsys is any
//     [fs.FS] value (e.g. from go:embed). The same navigation and type
//     detection logic applies to both.
//
// # Retrieval variants
//
//   - [FNode.Get] — full resolution: reads file content and parses it.
//   - [FNode.GetRaw] — reads the raw bytes without parsing structured types;
//     always sets Type to "file".
//   - [FNode.GetMeta] — resolves the path and sets Path, Type, and Params
//     without reading file content. Useful when the caller intends to stream
//     the file directly.
package fn
