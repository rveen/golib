package fs

import (
	"errors"
	"log"
	"path/filepath"
	"strings"

	"github.com/rveen/golib/fs/svnfs"
	"github.com/rveen/golib/fs/types"
	"github.com/rveen/ogdl"
)

// Get returns the specified revision of a directory, file or data part
//
//     Path := "" | "/" | (element["/" element]*)
//
// _n is interpreted as a revision number. The revision applies to the complete path
// given, and makes only sense if the given file system is versioned.
//
// Directory entries with the format _token (where token is not a number) are
// treated in a special way. When a path element is not found but
// there is a _token entry, it is followed and the path element set as parameter.
//
// Paths that start in an ordinary fie system, can continue to a versioned fs (SVN,
// Git), and then into a data file. Paths that point to a versioned fs can go inside
// that fs and into a data file. Any other combination will not work.
//
// [ordinary fs] [-> versioned fs] [-> data file]
//
func (fs *fileSystem) Get(path, rev string) (*types.FileEntry, error) {

	// Clean input
	if rev == "" {
		rev = "HEAD"
	}
	path = filepath.Clean(path)

	// Split path into elements
	opath := path

	parts := strings.Split(path, "/")
	path = ""

	if opath == "/" || opath == "." {
		parts = nil
		path = "."
	}

	log.Println("fs.Get", opath, len(parts))

	// Start by getting the root dir (either 'dir' or 'svn')
	dir, err := fs.Info("", "")
	if err != nil {
		return nil, err
	}

	switch dir.Typ {
	case "svn":
		svn := svnfs.New(path)
		return svn.Get(path, rev)
	}

	fe := &types.FileEntry{}

	for i := 0; i < len(parts); i++ {

		part := parts[i]

		log.Println("Part:", part)

		// protection agains starting slash
		if part == "" {
			continue
		}

		if part[0] == '.' {
			return nil, errors.New("path element starting with . not allowed (see files.go)")
		}

		// path is the path that we are going to analyze in iteration of the loop

		if path == "" {
			path = part
		} else {
			path += "/" + part
		}

	retry:
		fe, err = fs.Info(path, rev)
		if err != nil {
			fe.Typ = ""
		}

		switch fe.Typ {

		case "_": // TODO Reserved for _* parts

		case "":
			// Path not found (as is), so look for _* and missing extensions
			// TODO But return not found if within a data path
			// TODO check _* and add to params

			ext := missingExtension(fs, dir, part, rev)
			if ext == "" {
				return nil, errors.New("Not found")
			}

			path += ext
			goto retry

		case "dir":
			// Check if there is a link (only in ordinary fs)
			if fs.Type() == "" {
				s := link(fs, path, rev)
				if s != "" {
					path = s
					goto retry
				}
			}
			dir = fe

		case "git":
			// A Git server repository
			//return GetGit(path, parts, i, -1)

		case "svn":
			svn := svnfs.New(path)
			dpath := ""
			rev = "HEAD"

			// Make up the path to the SVN repo
			// If any revision number appears, extract it.
			for i++; i < len(parts); i++ {
				j := strings.IndexRune(parts[i], '@')
				if j != -1 {
					rev = parts[i][j+1:]
					if rev != "" {
						dpath += "/" + parts[i][0:j]
					} else {
						dpath += "/" + parts[i]
					}
				} else {
					dpath += "/" + parts[i]
				}
			}

			if dpath != "" {
				dpath = dpath[1:]
			}

			return svn.Get(dpath, rev)

		case "data/ogdl":

			b, err := fs.File(path, rev)
			if err != nil {
				return nil, err
			}

			fe.Tree = ogdl.FromString(string(b))

			if i != len(parts)-1 {
				// Read the file and process the remaining part of the path
				dpath := ""
				for i++; i < len(parts); i++ {
					dpath += "." + parts[i]
				}
				dpath = dpath[1:]

				fe.Tree = fe.Tree.Get(dpath)
			}
			return fe, nil

		case "data/json":

			b, err := fs.File(path, rev)
			if err != nil {
				return nil, err
			}

			if i == len(parts)-1 {
				fe.Tree, _ = ogdl.FromJSON(b)
			} else {
				// Read the file and process the remaining part of the path
				dpath := ""
				for i++; i < len(parts); i++ {
					dpath += "." + parts[i]
				}
				dpath = dpath[1:]

				g, _ := ogdl.FromJSON(b)
				if g != nil {
					fe.Tree = g.Get(dpath)
				}
			}
			return fe, nil

		case "revs":

			var err error
			fe.Tree, err = fs.Revisions(path, rev)
			return fe, err

		default:

			// A file (with no known structure). No more parts can be handled.
			if i < len(parts)-1 {
				return nil, errors.New("file found but can not navigate into it")
			}

			fe.Content, _ = fs.File(path, rev)
			return fe, nil
		}
	}

	// Process directory

	s := link(fs, path, rev)
	if s != "" {
		return fs.Get(s, rev)
	}

	indexFile, data, ls := fs.Index(fs, path, rev)

	if indexFile != "" {
		fe.Content, _ = fs.File(indexFile, rev)
		// TODO: fe.Typ, _ = Type(fs, indexFile, rev)
		fe.Name = indexFile
	}

	fe.Tree = data
	fe.Info = ls

	return fe, nil
}

// returns the missing extension if found, else "".
func missingExtension(fs FileSystem, dir *types.FileEntry, part, rev string) string {

	// Read the directory just above the latest unfound part.

	for _, f := range dir.Dir {
		log.Println("  -", f.Name())
		// Strip extension and check
		j := strings.LastIndexByte(f.Name(), '.')
		if j == -1 {
			continue
		}
		name := f.Name()[0:j]
		if name == part {
			return filepath.Ext(f.Name())
		}
	}
	return ""
}

// link returns the content of index.link, if present in the directory
// given in the path argument, or an empty string.
func link(fs FileSystem, path, rev string) string {

	// TODO revert to fs.Info()
	b, err := fs.File(path+"/index.link", rev)

	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(b))
}
