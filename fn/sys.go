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

func New(root string) *FNode {
	return &FNode{Base: root}
}

func (fn *FNode) Put(path string, content []byte) error {
	return nil
}

// Get
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

func (fn *FNode) get(path string, raw bool) error {

	log.Println("fn.get", fn.Base, path)

	// Navigate the standard file system part

	path = filepath.Clean(path)

	if len(path) > 0 && path[0] == '/' {
		path = path[1:]
	}

	fn.Parts = strings.Split(path, "/")
	fn.N = 0
	fn.Path = fn.Base

	for {
	nav:
		err := fn.navigate()
		if err != nil {
			return err
		}
		log.Println(" - fn.get.navigate", fn.Path, fn.Type, path)

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
			err = fn.dir()
			if err != nil {
				return err
			}

			// If left !=0, there can be an index.ogdl to follow, or a _token
			// if left == 0, check index / readme
			if left == 0 {
				if !fn.index() {
					return nil
				}
				goto again
			}

			log.Println("going to check for _token, part:", fn.Parts[fn.N])

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
					if fn.Params == nil {
						fn.Params = make(map[string]string)
					}
					fn.Params[token[1:]] = fn.Parts[fn.N]
					fn.Parts[fn.N] = token
					goto nav
				}
			}
			return errors.New("404")

		case "file":
			if left != 0 {
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

// file() -> fn.Content
func (fn *FNode) file() error {
	var err error
	fn.Content, err = os.ReadFile(fn.Path)
	return err
}

// dir() -> fn.Data
func (fn *FNode) dir() error {
	dir, err := os.ReadDir(fn.Path)
	if err != nil {
		return err
	}

	g := ogdl.New(nil)

	for _, entry := range dir {
		fi, err := entry.Info()
		if err != nil {
			continue
		}

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

		if fi.Mode()&fs.ModeSymlink != 0 {
			fii, _ := os.Stat(fn.Path + "/" + fi.Name())
			if fii.IsDir() {
				f.Set("type", "dir")
			}
		}
	}

	fn.Data = g

	return nil
}

// Navigate up to what exists
func (fn *FNode) navigate() error {

	for i := fn.N; i < len(fn.Parts); i++ {

		part := fn.Parts[i]

		if part != "" && part[0] == '.' {
			return errors.New(". not allowed in paths")
		}

		saveThis := fn.Path
		fn.Path += "/" + part
		fn.Path = filepath.Clean(fn.Path)

		typ := fn.info()

		if typ == "" {
			fn.Path = saveThis
			return nil
		}

		fn.Type = typ
		fn.N++

		if typ != "dir" {
			return nil
		}
	}

	return nil
}

var exts = []string{".html", ".htm", ".md", ".ogdl"}

// return info about a concrete path
//
// fn is not affected.
func (fn *FNode) info() string {
	f, err := os.Stat(fn.Path)
	if err != nil {
		// check assumed extensions (if the path has no extension already)
		if filepath.Ext(fn.Path) != "" {
			return ""
		}
		for _, ext := range exts {
			f, err = os.Stat(fn.Path + ext)
			if err == nil {
				fn.Path += ext
				return fn.fileType()
			}
		}
		return ""
	}

	if f.IsDir() {
		return fn.dirType()
	}

	return fn.fileType()
}

// dirType returns either 'dir ', 'svn' or 'git'
// for the path contained in fn.Path
//
// fn is not affected.
func (fn *FNode) dirType() string {
	ff, err := os.ReadDir(fn.Path)
	if err != nil {
		return ""
	}

	sscore := 0
	gscore := 0

	for _, f := range ff {

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
