package fs

import (
	"golib/jupyter"
	"mime"
	"os"
	"path/filepath"
	"strings"

	// "github.com/russross/blackfriday"
	"github.com/rveen/markdown"
	"github.com/rveen/markdown/parser"
	"github.com/rveen/ogdl"
)

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

// Type examines the path and returns its type. Path should be an existing directory
// or file in the file system.
func Type(fs FileSystem, path, rev string) (string, error) {

	f, err := fs.Info(path, rev)
	if err != nil {
		return "", err
	}

	if !f.IsDir() {
		// return its type by looking at the extension
		s := TypeByExtension(f.Name())
		if s == "" {
			return "file", nil
		}
		return s, nil
	}

	ff, err := fs.Dir(path, rev)
	if err != nil {
		return "", err
	}

	// Git
	sscore := 0
	gscore := 0
	for _, f := range ff {

		switch f.Name() {
		case "format":
			sscore++
			if sscore > 1 {
				return "svn", nil
			}
		case "hooks":
			sscore++
			gscore++
			if sscore > 1 {
				return "svn", nil
			}
			if gscore > 1 {
				return "git", nil
			}
		case "HEAD":
			gscore++
			if gscore > 1 {
				return "svn", nil
			}
		}
	}
	return "dir", nil
}

// Index checks if there are index.* files, and the dir info (list).
//
// - index.ogdl -> graph
// - index.* -> string (if there are several, take highest in the list (htm, md, ...)
// - dir info -> graph.dir (only if index.nolist is not found)
func Index(fs FileSystem, path, rev string) (string, *ogdl.Graph, *ogdl.Graph) {

	// Read the directory
	ff, err := fs.Dir(path, rev)

	if err != nil {
		return "", nil, nil
	}

	var g *ogdl.Graph
	indexFile := ""
	nodir := false

	// Read any index.* files
	for _, f := range ff {
		name := f.Name()

		if name == "index.link" {
			continue
		}

		if name == "index.nolist" {
			nodir = true
		}

		if name == "index.ogdl" {
			b, err := fs.File(path+"/index.ogdl", rev)
			if err != nil {
				return "", nil, nil
			}
			g = ogdl.FromString(string(b))
			continue
		}

		if strings.HasPrefix(name, "index.") {
			indexFile = path + "/" + name
		}
	}

	if nodir {
		return indexFile, g, nil
	}

	// Read dir info

	dir := ogdl.New(nil)

	// Add directoryes to the list, but not those starting with . or _
	for _, f := range ff {
		name := f.Name()

		// TODO optimize :-|
		// SVN and git: do not set mode, because Lstat will not work
		if (f.IsDir() || f.Mode()&os.ModeSymlink != 0) && name[0] != '_' && name[0] != '.' {
			// If a symlink, we want the info of the object where it points to
			if f.Mode()&os.ModeSymlink != 0 {
				f, err = os.Lstat(path + "/" + name + "/")
				if err != nil || !f.IsDir() {
					continue
				}
			}
			d := dir.Add("-")
			d.Add("name").Add(name)
			d.Add("type").Add("d")

		}
	}

	// Add regular files to the list, but not those starting with . or _
	// SVN and git: do not set mode, because Lstat will not work
	for _, f := range ff {
		name := f.Name()
		if !f.IsDir() && name[0] != '_' && name[0] != '.' {
			// If a symlink, we want the info of the object where it points to
			if f.Mode()&os.ModeSymlink != 0 {
				f, err = os.Lstat(path + "/" + name + "/")
				if err != nil || f.IsDir() {
					continue
				}
			}
			d := dir.Add("-")
			d.Add("type").Add(TypeByExtension(filepath.Ext(name)))
			d.Add("name").Add(name)
		}

		if strings.HasPrefix(strings.ToLower(name), "readme.") {
			indexFile = path + "/" + name
		}
	}

	return indexFile, g, dir
}
