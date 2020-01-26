package sysfs

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"

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
