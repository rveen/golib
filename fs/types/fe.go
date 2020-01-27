package types

import (
	"mime"
	"os"
	"path/filepath"
	"time"

	"github.com/rveen/golib/jupyter"

	"github.com/rveen/markdown"
	"github.com/rveen/markdown/parser"

	"github.com/rveen/ogdl"
)

// FileEntry type
// TODO: interface extension os os.FileInfo?
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
}

var isTemplate = map[string]bool{
	".htm": true,
	".txt": true,
}

func (f *FileEntry) IsDir() bool {
	if f.Typ == "dir" {
		return true
	}
	return false
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
		// Process markdown
		//f.content = blackfriday.MarkdownCommon(f.content)

		// this in init() !!!
		extensions := parser.CommonExtensions | parser.AutoHeadingIDs
		p := parser.NewWithExtensions(extensions)

		f.Content = markdown.ToHTML(f.Content, p, nil)

		f.Template = ogdl.NewTemplate(string(f.Content))
		f.Mime = "text/html"
		f.Typ = "m"
	} else if ext == ".ipynb" {
		g, _ := jupyter.FromJupyter(f.Content)
		f.Content, _ = jupyter.ToHTML(g)
		f.Mime = "text/html"
		f.Typ = "nb"
	}
}
