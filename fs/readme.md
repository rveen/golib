# fs, a file reader

This package implements a file system 'browser' for use in a web server. 
By specifying paths in the file system, it returns directories, files and part of files
(specific types of files) that correspond to that path. The particularity of this
package is that it allows navigation into either conventional or versioned file
systems (such as Subversion or Git), and into data files (only OGDL at the moment).
Use of file extensions is optional (if the file name is unique).

For now this is a read-only implementation. The content of a path is returned if found, but the file system cannot be modified.

When the path points to a directory that contains an index.* file
it returnes this file along with the directory list. Presence of several
index.* files is not supported except in special cases: index.nolist(= do not return
directory list).

## Main API

The two main functions of this package are New and Get.

    fs := fs.New("/dir")
    fe := fs.Get("path")

## Implementation details

### FileSystem

A file system is opened by giving a root location to New():

    include "github.com/rveen/golib/fs"

    fs := fs.New("/dir")

where fs is a FileSystem interface. Its root is per definition an ordinary directory.

Each FileSystem implements these functions:

    type FileSystem interface {
        Root() string
	    Info(path, rev string) (*types.FileEntry, error)
	    Dir(path, rev string) ([]os.FileInfo, error)
	    File(path, rev string) ([]byte, error)
	    Revisions(path, rev string) (*ogdl.Graph, error)
	    Type() string
	    Get(path, rev string) (*types.FileEntry, error)
    }




### FileEntry

FileEntry is an interface that extends the standard os.FileInfo.

    type FileEntry struct {
	    Name     string
	    Size     int64
	    Content  []byte
	    Template *ogdl.Graph
	    Data     *ogdl.Graph
	    Info     *ogdl.Graph
	    Typ      string
	    Mime     string
	    Time     time.Time
	    Param    map[string]string
	    Mode     os.FileMode
	    Dir      []os.FileInfo
    }