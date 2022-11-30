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
//
func (fn *FNode) Get(path string) error {
	return fn.get(path, false)
}

func (fn *FNode) GetRaw(path string) error {
	err := fn.get(path, true)
	fn.Type = "file"
	return err
}

// get returns an FNode (it updates its receiving object).
//
// What should fn contain to start with ?????
//
// If raw is true, files are returned as is, not as data or document.
// The rest of the behavior is unchanged.
func (fn *FNode) get(path string, raw bool) error {

	// Navigate the standard file system part. A get() allways starts at a
	// normal directory. The path given is relative to the root directory
	// as given to New(). It can be seen as absolute within that root.

	// clean up 'path' and prepare fn.Parts with the path elements or parts.
	// Reset the part counter fn.N.
	//
	// fn.Path is set to the absolute root path. fn.Parts contain the given
	// path starting from the root. As parts are processed, they are added
	// to fn.Part and the part counter fn.N is incremented.

	if len(path) > 0 && path[0] == '/' {
		path = path[1:]
	}

	fn.Parts = strings.Split(path, "/")
	fn.N = 0
	fn.Path = fn.Root

	final := false

	for {
	nav:
		err := fn.navigate()
		if err != nil {
			log.Println(err.Error())
			return err
		}
		// log.Printf(" - fn.get.navigate fn.Path=[%s] fn.Type=[%s] path=[%s]\n", fn.Path, fn.Type, path)

		// Case: we reached a 'svn' or 'git' dir. Pass remaining path to new fs.
		// Look for revisions

		if fn.Type == "svn" {
			// Create a new fn, where Base is the root of the svn repo, and Path
			// the path in the repo
			fn2 := New(fn.Path)
			err = fn2.svnGet(fn.remainingPath())
			*fn = *fn2
			return err
		}

		left := len(fn.Parts) - fn.N

	again:

		switch fn.Type {
		case "document":
			fn.file()
			if !raw {
				fn.document()
			}
			return nil

		case "data":
			fn.file()
			if !raw {
				fn.data()
			}
			return nil

		case "dir":
			// log.Println(" - fn.get: dir", fn.Path)
			err = fn.dir()
			if err != nil {
				return err
			}

			// If left !=0, there can be an index.ogdl to follow, or a _token
			// if left == 0, check index / readme
			if left == 0 || final {
				if !fn.index() {
					return nil
				}
				goto again
			}

			// log.Println("going to check for _token, part:", fn.Parts[fn.N])

			// check _token.
			// If the option is set and we have parts left to process, then
			// check if there is a _token entry in the dir. If yes, follow that
			// path and take note in fn.Params
			for _, entry := range fn.Data.Out {
				token := entry.ThisString()

				if token == "index.ogdl" {
					fn.Path += "/index.ogdl"
					fn.N--
					continue
				}
				if token[0] == '_' {
					// log.Println(" - token: ", token)
					if fn.Params == nil {
						fn.Params = make(map[string]string)
					}

					if strings.HasSuffix(token, "_end") {
						// log.Println(" - end token: ", token, fn.remainingPath())
						fn.Params[token[1:len(token)-4]] = fn.remainingPath()
						fn.Parts[fn.N] = token
						final = true
					} else {
						fn.Params[token[1:]] = fn.Parts[fn.N]
						fn.Parts[fn.N] = token
					}
					goto nav
				}
			}
			return errors.New("404")

		case "file":
			if left != 0 && !final {
				return errors.New("not navigable")
			}
			return fn.file()

		default:
			return errors.New("unknown type " + fn.Type)
		}
	}

	return errors.New("cannot reach this point!")
}

func (fn *FNode) index() bool {
	for _, entry := range fn.Data.Out {
		name := entry.ThisString()

		if strings.HasPrefix(name, "index.") || strings.HasPrefix(name, "readme.") {
			fn.Path += "/" + name
			fn.Type = fn.fileType()
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
//
func (fn *FNode) navigate() error {

	for i := fn.N; i < len(fn.Parts); i++ {

		fn.N++

		part := fn.Parts[i]

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
			fn.N--
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

// return info about a concrete path
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
