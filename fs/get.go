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
func Get(fs FileSystem, path, rev string) (*types.FileEntry, error) {

	if rev == "" {
		rev = "HEAD"
	}

	// Prepare and clean path
	path = filepath.Clean(path)
	opath := path

	parts := strings.Split(path, "/")
	path = ""
	fe := &types.FileEntry{}
	dir := "."
	typ := ""
	v := make(map[string]string)

	if opath == "/" || opath == "." {
		parts = nil
		fe.Typ = "dir"
		path = "."
	}

	log.Println("fs.Get", opath, len(parts))

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

	tryAgain:

		// Get info on current path
		// typ, _ = Type(fs, path, rev)
		var err error
		fe, err = fs.Info(path, rev)
		if err != nil {
			return nil, err
		}

		// fe.typ = typ
		fe.Name = path
		log.Printf("Get: Type: path %s, Type %s, in dir %s\n", path, typ, dir)

		switch fe.Typ {

		case "_": // TODO Reserved for _* parts
			fe.Param = v

		case "":
			// Path not found (as is), so look for _* and missing extensions
			// TODO But return not found if within a data path

			// TODO check _* and add to params

			ext := missingExtension(fs, part, dir, rev)
			if ext == "" {
				return nil, errors.New("Not found")
			}

			path += ext
			goto tryAgain

		case "dir":
			// Check if there is a link (only in ordinary fs)
			if fs.Type() == "" {
				s := link(fs, path, rev)
				if s != "" {
					path = s
					goto tryAgain
				}
			}

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

			log.Println("Get(svnfs, ", dpath, rev)
			return svn.Get(dpath, rev)

		case "data/ogdl":

			b, err := fs.File(path, rev)
			if err != nil {
				return nil, err
			}

			fe.Tree = ogdl.FromString(string(b))

			log.Println("File read", path, fe.Tree.Text())

			if i != len(parts)-1 {
				// Read the file and process the remaining part of the path
				dpath := ""
				for i++; i < len(parts); i++ {
					dpath += "." + parts[i]
				}
				dpath = dpath[1:]

				log.Println("------ dpath", dpath)
				log.Printf("\n%s\n", fe.Tree.Show())

				fe.Tree = fe.Tree.Get(dpath)
			}
			return fe, nil
		/*
			case "data/json":

				b, err := fs.File(path)
				if err != nil {
					return nil, err
				}

				if i == len(parts)-1 {
					fe.tree, _ = ogdl.FromJSON(b)
				} else {
					// Read the file and process the remaining part of the path
					dpath := ""
					for i++; i < len(parts); i++ {
						dpath += "." + parts[i]
					}
					dpath = dpath[1:]

					g, _ := ogdl.FromJSON(b)
					if g != nil {
						fe.tree = g.Get(dpath)
					}
				}
				return fe, nil
		*/

		case "revs":

			var err error
			fe.Tree, err = fs.Revisions(path, rev)
			// fe.name = path[:len(path)-1]
			return fe, err

		default:
			// A file
			// No more parts can be handled, since this is a blob
			if i < len(parts)-1 {
				log.Println("file found, but more elements remaining", parts[i+1], i, len(parts))
				return nil, errors.New("file found but can not navigate into it")
			}

			fe.Content, _ = fs.File(path, rev)
		}

		dir = path

	}

	// log.Printf("Get (at exit): path %s type %s\n", path, fe.Type())

	if fe.Typ != "dir" {
		return fe, nil
	}

	// Process directory

	s := link(fs, path, rev)
	if s != "" {
		log.Println("link found", s)
		return Get(fs, s, rev)
	}

	indexFile, data, ls := Index(fs, path, rev)

	if indexFile != "" {
		fe.Content, _ = fs.File(indexFile, rev)
		fe.Typ, _ = Type(fs, indexFile, rev)
		fe.Name = indexFile
	}

	fe.Tree = data
	fe.Info = ls

	return fe, nil
}

// returns the missing extension if found, else "".
func missingExtension(fs FileSystem, part, dir, rev string) string {

	// Read the directory just above the latest unfound part.
	ff, err := fs.Dir(dir, rev)

	if err != nil {
		return ""
	}

	for _, f := range ff {
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
