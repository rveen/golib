package fn

import (
	"io/fs"
	"strings"

	"github.com/rveen/golib/document"
	"github.com/rveen/ogdl"
)

type FNode struct {
	Base     string
	Fs       fs.FS
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

// The file has been loaded into fn.Content
func (fn *FNode) data() {

	fn.Data = ogdl.FromBytes(fn.Content)

	path := ""
	for i := fn.N; i < len(fn.Parts); i++ {
		path += "." + fn.Parts[i]
	}
	if path != "" {
		fn.Data = fn.Data.Get(path[1:])
	}
}

// The file has been loaded into fn.Content
func (fn *FNode) document() {

	fn.Document, _ = document.New(string(fn.Content))

	if fn.N >= len(fn.Parts) {
		return
	}

	data := false
	if fn.Parts[fn.N] == "_" {
		fn.Data = fn.Document.Data()
		fn.Type = "data"
		fn.N++
		data = true
		if fn.N >= len(fn.Parts) {
			return
		}
	}

	path := ""
	for i := fn.N; i < len(fn.Parts); i++ {
		path += "." + fn.Parts[i]
	}

	if data {
		fn.Data = fn.Data.Get(path[1:])
	} else {
		fn.Document = fn.Document.Part(path[1:])
	}
}

func (fn *FNode) fileType() string {
	if strings.HasSuffix(fn.Path, ".md") {
		return "document"
	}
	if strings.HasSuffix(fn.Path, ".ogdl") {
		return "data"
	}
	return "file"
}
