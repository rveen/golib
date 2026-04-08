package fn

import (
	"io/fs"
	"strings"

	"github.com/rveen/golib/document"
	"github.com/rveen/ogdl"
)

type FNode struct {
	Root   string
	RootFs fs.FS

	Path     string
	Revision string
	Type     string
	Data     *ogdl.Graph
	Document *document.Document
	Content  []byte
	Params   map[string]string

	parts []string
	n     int
}

// joinParts joins fn.parts[fn.n:] with sep. Returns "" if there are no remaining parts.
func (fn *FNode) joinParts(sep string) string {
	if fn.n >= len(fn.parts) {
		return ""
	}
	var b strings.Builder
	for i := fn.n; i < len(fn.parts); i++ {
		if i > fn.n {
			b.WriteString(sep)
		}
		b.WriteString(fn.parts[i])
	}
	return b.String()
}

// remainingPath returns the unprocessed portion of the path (fn.parts[fn.n:]) joined by "/".
func (fn *FNode) remainingPath() string { return fn.joinParts("/") }

// remainingPathDot returns the unprocessed portion of the path (fn.parts[fn.n:]) joined by ".".
func (fn *FNode) remainingPathDot() string { return fn.joinParts(".") }

// Return the path as data. Start with fn.Content and process remaining
// parts of path. If there are no remaining parts, the whole data file is returned.
//
// The file is what is loaded into fn.Content
func (fn *FNode) data() {

	fn.Data = ogdl.FromBytes(fn.Content)

	path := fn.remainingPathDot()
	if path != "" {
		fn.Data = fn.Data.Get(path)
	}
}

// Return the path as a document. Start with fn.Content and process remaining
// parts of path. If there are no remaining parts, the whole document file is returned.
//
// The file is what is loaded into fn.Content
func (fn *FNode) document() error {

	var err error
	fn.Document, err = document.New(string(fn.Content))
	if err != nil {
		return err
	}

	// If the current part is "_" then we want the data view of this document.
	data := false
	if fn.n < len(fn.parts) && fn.parts[fn.n] == "_" {
		fn.Data = fn.Document.Data()
		fn.Type = "data"
		fn.n++
		data = true
	}

	path := fn.remainingPathDot()
	if path == "" {
		return nil
	}

	if data {
		fn.Data = fn.Data.Get(path)
	} else {
		fn.Document = fn.Document.Part(path)
	}
	return nil
}

func (fn *FNode) fileType() string {
	return fileType(fn.Path)
}

// fileType returns the type of a file: "data" (.ogdl), "document" (.md), or "file" (anything else).
func fileType(path string) string {
	if strings.HasSuffix(path, ".md") {
		return "document"
	}
	if strings.HasSuffix(path, ".ogdl") {
		return "data"
	}
	return "file"
}
