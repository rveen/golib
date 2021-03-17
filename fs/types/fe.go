package types

import (
	"mime"
	"os"
	"path/filepath"
	"time"

	"github.com/rveen/golib/document"
	// "github.com/rveen/golib/jupyter"
	// "github.com/rveen/markdown"
	// "github.com/rveen/markdown/parser"
	"github.com/rveen/ogdl"
)

// FileEntry type
// TODO: interface extension of os.FileInfo?
/*
type FileInfo interface {
    Name() string       // base name of the file
    Size() int64        // length in bytes for regular files; system-dependent for others
    Mode() FileMode     // file mode bits
    ModTime() time.Time // modification time
    IsDir() bool        // abbreviation for Mode().IsDir()
    Sys() interface{}   // underlying data source (can return nil)
}
*/
type FileEntry struct {
	Name     string
	Size     int64
	Content  []byte
	Template *ogdl.Graph
	Data     *ogdl.Graph
	Info     *ogdl.Graph
	Typ      string
	Mime     string
	Time     time.Time
	Param    map[string]string
	Mode     os.FileMode
	Dir      []os.FileInfo
	Doc      *document.Document
}

// TODO remove template support?
// Template preprocessing makes sense if caching is used
var isTemplate = map[string]bool{
	".htm": true,
	".txt": true,
}

// TODO use mode bit
func (f *FileEntry) IsDir() bool {
	return f.Typ == "dir"
}

// Prepare preprocesses some types of files: markdown, templates.
func (f *FileEntry) Prepare() {

	// set MIME type
	ext := filepath.Ext(f.Name)
	f.Mime = mime.TypeByExtension(ext)

	// Pre-process template or markdown
	if isTemplate[ext] {
		f.Template = ogdl.NewTemplate(string(f.Content))
		f.Typ = "t"

	} else if ext == ".md" {

		doc, _ := document.New(string(f.Content))
		f.Content = []byte(doc.Html())

		f.Template = ogdl.NewTemplate(string(f.Content))
		f.Mime = "text/html"
		f.Typ = "m"
		f.Doc = doc

	}
}
