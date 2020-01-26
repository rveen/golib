package sysfs

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/rveen/golib/fs/types"
	"github.com/rveen/ogdl"
)

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

func (fs *fileSystem) Root() string {
	return fs.root
}

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

func (fs *fileSystem) Info(path, rev string) (*types.FileEntry, error) {

	fe := &types.FileEntry{}

	if path[len(path)-1] == '@' {
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

// Index checks if there are index.* files, and the dir info (list).
//
// - index.ogdl -> graph
// - index.* -> string (if there are several, take highest in the list (htm, md, ...)
// - dir info -> graph.dir (only if index.nolist is not found)
func (fs *fileSystem) Index(d *types.FileEntry, path, rev string) (*types.FileEntry, error) {

	// Read the directory
	var g *ogdl.Graph
	var err error
	indexFile := ""
	nodir := false

	// Read any index.* files
	for _, f := range d.Dir {
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
				return nil, err
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
			d := dir.Add("-")
			d.Add("name").Add(name)
			d.Add("type").Add("d")

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
			d := dir.Add("-")
			d.Add("type").Add(types.TypeByExtension(filepath.Ext(name)))
			d.Add("name").Add(name)
			d.Add("time").Add(f.ModTime().String())
		}

		if strings.HasPrefix(strings.ToLower(name), "readme.") {
			indexFile = path + "/" + name
		}
	}

	log.Println(dir.Text())

	return indexFile, g, dir
}
