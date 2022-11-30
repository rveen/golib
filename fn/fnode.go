package fn

import (
	"io/fs"
	"strings"

	"github.com/rveen/golib/document"
	"github.com/rveen/ogdl"
)

type FNode struct {
	Root     string
	RootFs   fs.FS
	Path     string
	Revision string
	Type     string
	Parts    []string
	N        int
	Data     *ogdl.Graph
	Document *document.Document
	Content  []byte
	Params   map[string]string
}

// Return the part of the path that hasn't been processed
// (from fn.N up to len(fn.Parts)
//
// It returns a relative path (not starting with /)
func (fn *FNode) remainingPath() string {

	path := ""
	for i := fn.N; i < len(fn.Parts); i++ {
		path += "/" + fn.Parts[i]
	}
	if path != "" {
		return path[1:]
	}
	return ""
}

// Return the part of the path that hasn't been processed
// (from fn.N up to len(fn.Parts). Use dots as separators
//
// It returns a relative path (not starting with /)
func (fn *FNode) remainingPathDot() string {

	path := ""
	for i := fn.N; i < len(fn.Parts); i++ {
		path += "." + fn.Parts[i]
	}
	if path != "" {
		return path[1:]
	}
	return ""
}

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
func (fn *FNode) document() {

	fn.Document, _ = document.New(string(fn.Content))

	// If the current part is "_" then we want the data view of this document.
	data := false
	if fn.Parts[fn.N] == "_" {
		fn.Data = fn.Document.Data()
		fn.Type = "data"
		fn.N++
		data = true
	} else {
		// fn.Type = "document"		Probably this has been set already
	}

	path := fn.remainingPathDot()
	if path == "" {
		return
	}

	if data {
		fn.Data = fn.Data.Get(path)
	} else {
		fn.Document = fn.Document.Part(path)
	}
}

// Return the type of a file either as data, document or file (blob).
func (fn *FNode) fileType() string {
	if strings.HasSuffix(fn.Path, ".md") {
		return "document"
	}
	if strings.HasSuffix(fn.Path, ".ogdl") {
		return "data"
	}
	return "file"
}
