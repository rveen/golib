package fs

import (
	"errors"
	// "log"
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
// @n is interpreted as a revision number. The revision applies to the complete path
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

	// TODO create this map only if there are parameters
	params := make(map[string]string)

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

	// log.Println("fs.Get", opath, len(parts))

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
			fe = &types.FileEntry{}
			fe.Typ = ""
		}

		switch fe.Typ {

		case "":
			// Path not found (as is), so:
			// - look for missing extension
			// - look for _*

			ext := missingExtension(fs, dir, part, rev)
			if ext != "" {
				path += ext
				goto retry
			}

			// If there is an entry of the form _token in this directory,
			// continue into it, adding token=unfound_element to fe.Params()
			p := fs.variable(fe, dir, part, params)
			if p == "" {
				return nil, errors.New("Not found")
			}
			path = path[0:len(path)-len(part)] + p
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

			fe.Data = ogdl.FromString(string(b))

			if i != len(parts)-1 {
				// Read the file and process the remaining part of the path
				dpath := ""
				for i++; i < len(parts); i++ {
					dpath += "." + parts[i]
				}
				dpath = dpath[1:]

				fe.Data = fe.Data.Get(dpath)
			}
			return fe, nil

		case "data/json":

			b, err := fs.File(path, rev)
			if err != nil {
				return nil, err
			}

			if i == len(parts)-1 {
				fe.Data, _ = ogdl.FromJSON(b)
			} else {
				// Read the file and process the remaining part of the path
				dpath := ""
				for i++; i < len(parts); i++ {
					dpath += "." + parts[i]
				}
				dpath = dpath[1:]

				g, _ := ogdl.FromJSON(b)
				if g != nil {
					fe.Data = g.Get(dpath)
				}
			}
			return fe, nil

		case "revs":

			var err error
			fe.Data, err = fs.Revisions(path, rev)
			return fe, err

		default:

			// log.Println("Get file", path, fe.Typ, i, len(parts))

			if i < len(parts)-1 {
				if fe.Typ != "text/markdown" {
					// A file (with no known structure). No more parts can be handled.
					return nil, errors.New("file found but can not navigate into it")
				}

				// Markdown can be dived in!

				fe.Content, _ = fs.File(path, rev)
				fe.Param = params
				fe.Name = path
				fe.Prepare()

				// Read the file and process the remaining part of the path
				dpath := ""
				for i++; i < len(parts); i++ {
					dpath += "." + parts[i]
				}
				dpath = dpath[1:]

				if dpath == "_" {
					fe.Typ = "data/ogdl"
					fe.Data = fe.Doc.Data()
				} else {
					fe.Typ = "m"
					fe.Content = []byte(fe.Doc.Part(dpath).Html())
					fe.Template = ogdl.NewTemplate(string(fe.Content))
					fe.Doc = fe.Doc.Part(dpath)
				}

				return fe, nil
			}

			fe.Content, _ = fs.File(path, rev)
			fe.Param = params
			fe.Name = path
			fe.Prepare()
			return fe, nil
		}
	}

	// Process directory

	s := link(fs, path, rev)
	if s != "" {
		return fs.Get(s, rev)
	}

	err = fs.Index(dir, path, rev)

	if err != nil {
		// TODO
	}

	dir.Param = params

	return dir, nil
}

// returns the missing extension if found, else "".
func missingExtension(fs FileSystem, dir *types.FileEntry, part, rev string) string {

	// Read the directory just above the latest unfound part.

	for _, f := range dir.Dir {
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

func (fs *fileSystem) variable(fe, dir *types.FileEntry, part string, params map[string]string) string {

	for _, f := range dir.Dir {
		if f.Name()[0] == '_' {
			params[f.Name()[1:]] = part
			return f.Name()
		}
	}
	return ""
}
