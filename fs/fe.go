package fs

import (
	"os"
	"time"

	"github.com/rveen/ogdl"
)

// FileEntry implements the os.FileInfo interface and can hold also metainfo and
// the content of the file.
type FileEntry interface {
	Name() string
	Size() int64
	Content() []byte
	Info() *ogdl.Graph
	Tree() *ogdl.Graph
	Type() string
	Mime() string
	Param() map[string]string
	Prepare()
}

// fileEntry implements FileEntry
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

func (f *fileEntry) Name() string       { return f.name }
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
func (f *fileEntry) Tree() *ogdl.Graph        { return f.tree }
func (f *fileEntry) Type() string             { return f.typ }
func (f *fileEntry) Mime() string             { return f.mime }
func (f *fileEntry) Param() map[string]string { return f.param }
