package svnfs

import (
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rveen/golib/jupyter"

	"github.com/rveen/markdown"
	"github.com/rveen/markdown/parser"

	"github.com/rveen/ogdl"
)

// fileEntry fullfils the fsutil.FileEntry and os.FileInfo interfaces
type fileEntry struct {
	name    string
	size    int64
	content []byte
	tree    *ogdl.Graph
	info    *ogdl.Graph
	typ     string
	mime    string
	time    time.Time
	param   map[string]string
}

// Name returns the base name of the file
func (f *fileEntry) Name() string {

	// Do not return a release number
	i := strings.LastIndex(f.name, "@")
	if i == -1 {
		return f.name
	}
	return f.name[0:i]
}

func (f *fileEntry) Size() int64        { return f.size }
func (f *fileEntry) Mode() os.FileMode  { return 0 }
func (f *fileEntry) ModTime() time.Time { return f.time }

func (f *fileEntry) IsDir() bool {
	if f.typ == "dir" {
		return true
	}
	return false
}

func (f *fileEntry) Sys() interface{}         { return nil }
func (f *fileEntry) Content() []byte          { return f.content }
func (f *fileEntry) Info() *ogdl.Graph        { return f.info }
func (f *fileEntry) Type() string             { return f.typ }
func (f *fileEntry) Mime() string             { return f.mime }
func (f *fileEntry) Param() map[string]string { return f.param }
func (f *fileEntry) Tree() *ogdl.Graph        { return f.tree }

var isTemplate = map[string]bool{
	".htm": true,
	".txt": true,
}

// Prepare preprocesses some types of files: markdown, templates.
func (f *fileEntry) Prepare() {

	// set MIME type
	ext := filepath.Ext(f.name)
	f.mime = mime.TypeByExtension(ext)

	// Pre-process template or markdown
	if isTemplate[ext] {
		f.tree = ogdl.NewTemplate(string(f.content))
		f.typ = "t"

	} else if ext == ".md" {
		// Process markdown
		//f.content = blackfriday.MarkdownCommon(f.content)

		// this in init() !!!
		extensions := parser.CommonExtensions | parser.AutoHeadingIDs
		p := parser.NewWithExtensions(extensions)

		f.content = markdown.ToHTML(f.content, p, nil)

		f.tree = ogdl.NewTemplate(string(f.content))
		f.mime = "text/html"
		f.typ = "m"
	} else if ext == ".ipynb" {
		g, _ := jupyter.FromJupyter(f.content)
		f.content, _ = jupyter.ToHTML(g)
		f.mime = "text/html"
		f.typ = "nb"
	}
}
