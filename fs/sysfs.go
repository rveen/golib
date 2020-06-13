// This package implements a file system 'browser' for use in a web server. By
// specifying paths in the file system, it returns directories, files and part of files
// (specific types of files) that correspond to that path. The particularity of this
// package is that it allows navigation into either conventional or versioned file
// systems (such as Subversion or Git), and into data files (only OGDL at the moment).
// Use of file extensions is optional (if the file name is unique).
//
// For now this is a read-only implementation. The content of a path is returned if found,
// but the file system cannot be modified.
//
// When the path points to a directory that contains an index.* file
// it returnes this file along with the directory list. Presence of several
// index.* files is not supported except in special cases: index.nolist(= do not return
// directory list).
//
// Paths
//
// Paths are a sequence of elements separated by slashes, following the Unix / Linux
// notation. Two special cases exist:
//
// - @token is interpreted as a release or commit identifier, and removed from
// the path. Instead a "revision" parameter is added to fe.Param().
//
// - If an element is not found in a directory but the directory contains a _token
// entry, that one is followed. A parameter is attached to fe.Params() with the
// token as name and the unfound element as value.
//
// Example
//
// The two main functions of this package are New and Get.
//
//   ff := fs.New("/dir")
//   fe, err := ff.Get("file")
//
// Get returns a FileEntry object that implements the os.FileInfo interface and
// holds also the content (directoty list, file).
//
// TODO: for what os.FileInfo ??
//
// Templates
//
// File extensions that are configured as OGDL templates are preprocessed as such,
// that is, they are parsed and converted into an OGDL object accessible through fe.Data.
// TODO: should this be done outside of this package?? (caching is a reason to do
// it here)
//
//
// Navigating data files
//
// Navigation within an OGDL file is handled over to the ogdl package (ogdl.Get).
//
// Navigating documents
//
// Markdown document navigation is handled by the document package (document.Get).
//
// Database navigation
//
// Not supported (done through templates).
//
// Relation between path and template
//
// Is this a fixed relation or can we specify a template for a path in an elegant
// way ? Or is it better to just write a template with the query or path inside ?
// Are we mixing functions?
//
// Revision list
//
// How to obtain the log of a path and use it in a template.
//
//   g := fs.Log(path)
package fs

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/rveen/golib/fs/types"
	"github.com/rveen/ogdl"
)

type FileSystem interface {
	Root() string
	Info(path, rev string) (*types.FileEntry, error)
	Dir(path, rev string) ([]os.FileInfo, error)
	File(path, rev string) ([]byte, error)
	Revisions(path, rev string) (*ogdl.Graph, error)
	Type() string
	Get(path, rev string) (*types.FileEntry, error)
}

type fileSystem struct {
	root string
}

// New creates the FileSystem object needed to operate with a file system. A path
// to an ordinary directory should be given.
func New(root string) *fileSystem {
	fs := &fileSystem{}
	fs.root, _ = filepath.Abs(root)
	return fs
}

// Root returns the absolute path to the root of this FileSystem (the path
// given to New).
func (fs *fileSystem) Root() string {
	return fs.root
}

// Types return either "svn", "git" or "".
func (fs *fileSystem) Type() string {
	return ""
}

func (fs *fileSystem) Revisions(path, rev string) (*ogdl.Graph, error) {
	return nil, errors.New("not versioned")
}

func (fs *fileSystem) Dir(path, rev string) ([]os.FileInfo, error) {
	return ioutil.ReadDir(fs.root + "/" + path)
}

func (fs *fileSystem) File(path, rev string) ([]byte, error) {
	return ioutil.ReadFile(fs.root + "/" + path)
}

// Info returns the type of 'path'.
//
// If 'path' ends with @, it returns "revs" (meaning that we expect a list of
// revisions of that path).
//
// 'path' can be either a directory or a file. If it's a directory, it will
// return "svn" or "git" if it identifies it as the root of a server repository
// of either type, else it returns "dir".
//
// If 'path' is a file, it returns its type if in the types.go list, else "file".
func (fs *fileSystem) Info(path, rev string) (*types.FileEntry, error) {

	fe := &types.FileEntry{}

	if len(path) > 1 && path[len(path)-1] == '@' {
		fe.Typ = "revs"
		return fe, nil
	}

	f, err := os.Stat(fs.root + "/" + path)
	if err != nil {
		return nil, err
	}

	fe.Mode = f.Mode()

	if !f.IsDir() {
		// return its type by looking at the extension
		s := types.TypeByExtension(f.Name())
		if s == "" {
			fe.Typ = "file"
		} else {
			fe.Typ = s
		}
		return fe, nil
	}

	ff, err := fs.Dir(path, rev)
	if err != nil {
		return nil, err
	}

	fe.Dir = ff

	// Git
	sscore := 0
	gscore := 0
	for _, f := range ff {

		switch f.Name() {
		case "format":
			sscore++
			if sscore > 1 {
				fe.Typ = "svn"
				return fe, nil
			}
		case "hooks":
			sscore++
			gscore++
			if sscore > 1 {
				fe.Typ = "svn"
				return fe, nil
			}
			if gscore > 1 {
				fe.Typ = "git"
				return fe, nil
			}
		case "HEAD":
			gscore++
			if gscore > 1 {
				fe.Typ = "svn"
				return fe, nil
			}
		}
	}
	fe.Typ = "dir"

	return fe, err
}

// Index checks if there are index.* files in the given directory
//
// - index.ogdl -> graph
// - index.* -> string (if there are several, take highest in the list (htm, md, ...)
// - dir info -> graph.dir (only if index.nolist is not found)
//
// This function assumes that the input file entry already holds the directory
// listing (in d.Dir, as []os.FileInfo).
//
// The same file entry is used as output. It just adds the content of the index file
// if found.
//
func (fs *fileSystem) Index(d *types.FileEntry, path, rev string) error {

	// Read the directory
	var err error
	nodir := false

	d.Content = nil

	// Read any index.* files
	for _, f := range d.Dir {
		name := f.Name()

		if name == "index.link" {
			continue
		}

		if name == "index.nolist" {
			nodir = true
			continue
		}

		if name == "index.ogdl" {
			b, err := fs.File(path+"/index.ogdl", rev)
			if err != nil {
				return err
			}
			d.Data = ogdl.FromString(string(b))
			continue
		}

		// Index files overwrite readme's
		if strings.HasPrefix(name, "index.") {

			fmt.Println("Index", name)

			b, _ := fs.File(path+"/"+name, rev)
			d.Content = b
			d.Name = path + "/" + name
			d.Prepare()
			continue
		}

		// The readme file is only read in if no index file is present
		if d.Content == nil && strings.HasPrefix(strings.ToLower(name), "readme.") {
			b, _ := fs.File(path+"/"+name, rev)
			d.Content = b
			d.Name = path + "/" + name
			d.Prepare()
		}
	}

	if nodir {
		return nil
	}

	// Read dir info

	dir := ogdl.New("dir")

	// Add directories to the list, but not those starting with . or _
	for _, f := range d.Dir {
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
			gd := dir.Add("-")
			gd.Add("name").Add(name)
			gd.Add("type").Add("d")

		}
	}

	// Add regular files to the list, but not those starting with . or _
	// SVN and git: do not set mode, because Lstat will not work
	for _, f := range d.Dir {
		name := f.Name()
		if !f.IsDir() && name[0] != '_' && name[0] != '.' {
			// If a symlink, we want the info of the object where it points to
			if f.Mode()&os.ModeSymlink != 0 {
				f, err = os.Lstat(path + "/" + name + "/")
				if err != nil || f.IsDir() {
					continue
				}
			}
			gd := dir.Add("-")
			gd.Add("type").Add(types.TypeByExtension(filepath.Ext(name)))
			gd.Add("name").Add(name)
			gd.Add("time").Add(f.ModTime().String())
		}
	}

	d.Data = dir
	return nil
}
