package svn

import (
	"os"
	"strings"
	"time"

	"github.com/rveen/ogdl"
)

// fileEntry fullfils the fs.FileEntry and os.FileInfo interfaces
type fileEntry struct {
	name    string
	size    int64
	content []byte
	tree    *ogdl.Graph
	info    *ogdl.Graph
	typ     string
	mime    string
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
func (f *fileEntry) ModTime() time.Time { return time.Time{} }

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
