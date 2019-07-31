package fs

import (
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
)

type FileSystem interface {
	Root() string
	Info(path, rev string) (os.FileInfo, error)
	Dir(path, rev string) ([]os.FileInfo, error)
	File(path, rev string) ([]byte, error)
}

type fileSystem struct {
	root string
}

// New creates the FileSystem object needed to operate with a file system. A path
// to an ordinary directory should be given.
func New(root string) FileSystem {

	fs := &fileSystem{}
	fs.root, _ = filepath.Abs(root)

	log.Println("fs.New", fs.root)

	return fs
}

func (fs *fileSystem) Root() string {
	return fs.root
}

func (fs *fileSystem) Dir(path, rev string) ([]os.FileInfo, error) {
	return ioutil.ReadDir(fs.root + "/" + path)
}

func (fs *fileSystem) File(path, rev string) ([]byte, error) {
	return ioutil.ReadFile(fs.root + "/" + path)
}

func (fs *fileSystem) Info(path, rev string) (os.FileInfo, error) {
	return os.Stat(fs.root + "/" + path)
}
