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
	Doc      *document.Document
}

var isTemplate = map[string]bool{
	".htm": true,
	".txt": true,
}

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

		// Task check marks to Unicode
		// TODO move this to the document package
		for i := 0; i < len(f.Content); i++ {

			if i+3 >= len(f.Content) {
				break
			}

			if f.Content[i] == '[' && f.Content[i+2] == ']' {
				switch f.Content[i+1] {
				case 'x':
					f.Content[i+2] = 0x92
					f.Content[i] = 0xE2
					f.Content[i+1] = 0x98
				default:
					f.Content[i+2] = 0x90
					f.Content[i] = 0xE2
					f.Content[i+1] = 0x98
				case '/':
					f.Content[i+2] = 0x91
					f.Content[i] = 0xE2
					f.Content[i+1] = 0x98
				}
				i += 3
			}
		}

		doc, _ := document.New(string(f.Content))
		f.Content = []byte(doc.Html())

		f.Template = ogdl.NewTemplate(string(f.Content))
		f.Mime = "text/html"
		f.Typ = "m"
		f.Doc = doc

	} /*else if ext == ".md" {
		// Process markdown
		//f.content = blackfriday.MarkdownCommon(f.content)

		// this in init() !!!
		extensions := parser.CommonExtensions | parser.AutoHeadingIDs
		p := parser.NewWithExtensions(extensions)

		// Task check marks to Unicode
		for i := 0; i < len(f.Content); i++ {

			if i+3 >= len(f.Content) {
				break
			}

			if f.Content[i] == '[' && f.Content[i+2] == ']' {
				switch f.Content[i+1] {
				case 'x':
					f.Content[i+2] = 0x92
					f.Content[i] = 0xE2
					f.Content[i+1] = 0x98
				default:
					f.Content[i+2] = 0x90
					f.Content[i] = 0xE2
					f.Content[i+1] = 0x98
				case '/':
					f.Content[i+2] = 0x91
					f.Content[i] = 0xE2
					f.Content[i+1] = 0x98
				}
				i += 3
			}
		}

		f.Content = markdown.ToHTML(f.Content, p, nil)

		f.Template = ogdl.NewTemplate(string(f.Content))
		f.Mime = "text/html"
		f.Typ = "m"
	} else if ext == ".ipynb" {
		g, _ := jupyter.FromJupyter(f.Content)
		f.Content, _ = jupyter.ToHTML(g)
		f.Mime = "text/html"
		f.Typ = "nb"
	} */
}
