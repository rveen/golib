package fn

import (
	"errors"
	"io/fs"

	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/rveen/ogdl"
)

// Create and emtpy FNode with the root set to the given system path.
func New(root string) *FNode {
	return &FNode{Root: root}
}

// Create an empty FNode with the root set to a io.FS filesystem. This is
// used for example with go.embed file systems.
func NewFS(fs fs.FS) *FNode {
	return &FNode{RootFs: fs}
}

func (fn *FNode) Put(path string, content []byte) error {
	return nil
}

// Get returns an FNode (it updates its receiving object).
//
// What should fn contain to start with ?????
//
// Revision rules
// - 1 rev per path
// - @ at the end means log()
// - @rev means a specific revision (at any point in the path that has revisions)
func (fn *FNode) Get(path string) error {
	return fn.get(path, false)
}

func (fn *FNode) GetRaw(path string) error {
	err := fn.get(path, true)
	fn.Type = "file"
	return err
}

func (fn *FNode) index() bool {

	if fn.Data == nil || fn.Data.Out == nil {
		log.Println("requesting fn.index with empty data")
		return false
	}

	for _, entry := range fn.Data.Out {
		name := entry.ThisString()

		if strings.HasPrefix(name, "index.") || strings.HasPrefix(name, "readme.") {
			fn.Path += "/" + name
			fn.Type = fn.fileType()
			// log.Println("fn.index: index found", fn.Path, fn.Type)
			return true
		}
	}
	return false
}

// Read a file into fn.Content (a byte array).
func (fn *FNode) file() error {

	var err error

	if fn.RootFs != nil {
		fn.Content, err = fs.ReadFile(fn.RootFs, ioPathClean(fn.Path))
	} else {
		fn.Content, err = os.ReadFile(fn.Path)
	}

	return err
}

func (fn *FNode) stat(path string) (fs.FileInfo, error) {

	if fn.RootFs == nil {
		return os.Stat(path)
	}

	// io.fs (possibly embedded)
	f, err := fn.RootFs.Open(ioPathClean(path))
	defer f.Close()

	if err != nil {
		return nil, err
	}

	return f.Stat()

}

// Read fn.Path or fn.RootFs+fn.Path as a directory and build a data structure
// in fn.Data.
//
// TODO: should be fn.Root+fn.Path
func (fn *FNode) dir() error {

	// If io.FS is set as root, use io.ReadDir. Else use regular os.ReadDir

	var dir []fs.DirEntry
	var err error

	if fn.RootFs != nil {
		dir, err = fs.ReadDir(fn.RootFs, ioPathClean(fn.Path))
	} else {
		dir, err = os.ReadDir(fn.Path)
	}

	if err != nil {
		return err
	}

	// Now build the data structure with the directory info

	g := ogdl.New(nil)

	for _, entry := range dir {
		fi, err := entry.Info()
		if err != nil {
			continue
		}

		// entries starting with '.' are not shown
		if fi.Name()[0] == '.' {
			continue
		}

		f := g.Add(fi.Name())
		f.Add("name").Add(fi.Name())
		if fi.IsDir() {
			f.Add("type").Add("dir")
		} else {
			f.Add("type").Add("file")
		}
		f.Add("size").Add(fi.Size())
		f.Add("time").Add(fi.ModTime().Unix())

		// Special trick for symbolic links (we want to follow them)
		// TODO Check this in go.embed

		if fi.Mode()&fs.ModeSymlink != 0 {
			fii, err := fn.stat(fn.Path + "/" + fi.Name())
			if err == nil && fii.IsDir() {
				f.Set("type", "dir")
			}
		}
	}

	fn.Data = g
	return nil
}

// Navigate up to what exists.
//
// - Add fn.Parts to fn.Path until not found
// - fn must represent that last known dir or file found
// - start at fn.Path+fn.Parts[fn.N]
func (fn *FNode) navigate() error {

	for i := fn.n; i < len(fn.parts); i++ {

		fn.n++

		part := fn.parts[i]

		if len(part) > 1 && part[0] == '.' {
			return errors.New(". not allowed in paths")
		}

		savedPath := fn.Path

		if fn.Path == "" {
			fn.Path = part
		} else {
			fn.Path += "/" + part
		}
		typ := fn.info()

		if typ == "" {
			fn.n--
			fn.Path = savedPath
			return nil
		}
		fn.Type = typ
	}

	return nil
}

// dirType returns either 'dir ', 'svn' or 'git'
// for the path contained in fn.Path
//
// fn is not affected.
func (fn *FNode) dirType() string {

	var dir []fs.DirEntry
	var err error

	if fn.RootFs != nil {
		dir, err = fs.ReadDir(fn.RootFs, ioPathClean(fn.Path))
	} else {
		dir, err = os.ReadDir(fn.Path)
	}

	if err != nil {
		return ""
	}

	sscore := 0
	gscore := 0

	for _, f := range dir {

		switch f.Name() {
		case "format":
			sscore++
			if sscore > 1 {
				return "svn"
			}
		case "hooks":
			sscore++
			gscore++
			if sscore > 1 {
				return "svn"
			}
			if gscore > 1 {
				return "git"
			}
		case "HEAD":
			gscore++
			if gscore > 1 {
				return "svn"
			}
		}
	}
	return "dir"
}

// Prepare the path for io.fs.
// io.fs.Read* functions need a path that doesn't start with / and is not empty.
func ioPathClean(path string) string {

	if strings.HasPrefix(path, "/") {
		path = path[1:]
	}
	if path == "" {
		path = "."
	}
	return path
}

var exts = []string{".html", ".htm", ".md", ".ogdl"}

// If fn.Path points to a file or directory, return its type. Options are
//
// - data (end with .ogdl)
// - document (ends with .md)
// - file (any other file
// - svn (SVN root directory)
// - git (Git root directory)
// - dir (regular directory: not SVN or Git)
// - emtpy string: type is unknown.
//
// fn.Path is updated if a missing extension has been found.
// TODO: do we want this?
func (fn *FNode) info() string {

	f, err := fn.stat(fn.Path)

	if err == nil {
		if f.IsDir() {
			return fn.dirType()
		} else {
			return fn.fileType()
		}
	}

	if err != nil {

		// If the path has an extension, then return "".
		// If not, check some standard extensions that can be assumed
		// (.html, .htm, .md and .ogdl)

		if filepath.Ext(fn.Path) != "" {
			return ""
		}
		for _, ext := range exts {
			f, err = fn.stat(fn.Path + ext)
			if err == nil {
				fn.Path += ext
				return fn.fileType()
			}
		}
	}

	return ""
}
