package fs

import (
	"mime"
	"os"
	"path/filepath"

	"github.com/rveen/ogdl"
)

type FileSystem interface {
	Root() string
	Info(path, rev string) (os.FileInfo, error)
	Dir(path, rev string) ([]os.FileInfo, error)
	File(path, rev string) ([]byte, error)
	Revisions(path, rev string) (*ogdl.Graph, error)
	// Return the type of underlying file system
	Type() string
}

// FileEntry implements the os.FileInfo interface and can hold also metainfo and
// the content of the file.
type FileEntry interface {
	Name() string
	Size() int64
	Content() []byte
	Info() *ogdl.Graph
	Tree() *ogdl.Graph
	Type() string
	Mime() string
	Param() map[string]string
	Prepare()
}

var typeByExtension map[string]string

func init() {

	typeByExtension = make(map[string]string)
	typeByExtension[".md"] = "text/markdown"
	typeByExtension[".nb"] = "data/notebook"
	typeByExtension[".html"] = "text/html"
	typeByExtension[".htm"] = "text/html"
	typeByExtension[".xml"] = "data/xml"
	typeByExtension[".ogdl"] = "data/ogdl"
	typeByExtension[".json"] = "data/json"
	typeByExtension[".yml"] = "data/yaml"
	typeByExtension[".pdf"] = "pdf"
	typeByExtension[".odt"] = "ooffice"
	typeByExtension[".odp"] = "ooffice"
	typeByExtension[".ods"] = "ooffice"
}

// TypeByExtension returns the type of a file according to its extension.
func TypeByExtension(ext string) string {

	// Allows ext to be an extension or a path, and works with paths starting
	// with a dot.
	ext = filepath.Ext(ext)

	s := typeByExtension[ext]
	if s == "" {
		s = mime.TypeByExtension(ext)
		// Should filter out character sets
	}
	return s
}
