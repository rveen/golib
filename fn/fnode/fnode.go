package fn

import (
	"strings"

	"github.com/rveen/golib/document"
	"github.com/rveen/ogdl"
)

type FNode interface {
	Get(string) error
	Put() error

	Base() string
	Path() string
	Revision() string
	Type() string
	Parts() []string
	N() int
	Data() *ogdl.Graph
	SetData(*ogdl.Graph)
	Document() *document.Document
	Content() []byte
}

func fileType(path string) string {
	if strings.HasSuffix(path, ".md") {
		return "document"
	}
	if strings.HasSuffix(path, ".ogdl") {
		return "data"
	}
	return "file"
}
