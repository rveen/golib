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
// * _n, where n is a number, is interpreted as a release number, and removed from
// the path. Instead a "revision" parameter is added to fe.Param().
//
// * If an element is not found in a directory but the directory contains a _token
// entry, that one is followed. A parameter is attached to fe.Params() with the
// token as name and the element as value.
//
// Example
//
// The two main functions of this package are New and Get.
//
//   fs := fs.New("/dir")
//   fe := fs.Get("file")
//
// Get returns a FileEntry object that implements the os.FileInfo interface and
// holds also the content (directoty list, file).
//
// TODO: for what os.FileInfo ??
//
// Templates
//
// File extensions that are configured as OGDL templates are preprocessed as such,
// that is, they are parsed and converted into an OGDL object accessible through fe.Tree().
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
	"log"
	"path/filepath"
	"strings"

	"github.com/rveen/golib/fs/svn"
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
func Get(fs FileSystem, path, rev string) (FileEntry, error) {

	if rev == "" {
		rev = "HEAD"
	}

	// Prepare and clean path
	path = filepath.Clean(path)
	opath := path

	parts := strings.Split(path, "/")
	path = ""
	fe := &fileEntry{}
	dir := "."
	typ := ""
	v := make(map[string]string)

	if opath == "/" || opath == "." {
		parts = nil
		fe.typ = "dir"
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
		typ, _ = Type(fs, path, rev)
		fe.typ = typ
		fe.name = path
		log.Printf("fs, Path %s, Type %s, Dir %s\n", path, typ, dir)

		switch typ {

		case "_": // TODO Reserved for _* parts
			fe.param = v

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
			svnfs := svn.New(path)
			dpath := ""
			rev = "HEAD"

			// Make up the path to the SVN repo
			// If any revision number appears, extract it.
			for i++; i < len(parts); i++ {
				j := strings.IndexRune(parts[i], '@')
				if j != -1 {
					rev = parts[i][j+1:]
					dpath += "/" + parts[i][0:j]
				} else {
					dpath += "/" + parts[i]
				}
			}

			if dpath != "" {
				dpath = dpath[1:]
			}

			log.Println("Get(svnfs, ", dpath, rev)
			return Get(svnfs, dpath, rev)

		case "data/ogdl":

			b, err := fs.File(path, rev)
			if err != nil {
				return nil, err
			}

			fe.tree = ogdl.FromString(string(b))

			log.Println("File read", path, fe.tree.Text())

			if i != len(parts)-1 {
				// Read the file and process the remaining part of the path
				dpath := ""
				for i++; i < len(parts); i++ {
					dpath += "." + parts[i]
				}
				dpath = dpath[1:]

				log.Println("------ dpath", dpath)
				log.Printf("\n%s\n", fe.tree.Show())

				fe.tree = fe.tree.Get(dpath)
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
			fe.tree, err = fs.Revisions(path, rev)
			// fe.name = path[:len(path)-1]
			return fe, err

		default:
			// A file
			// No more parts can be handled, since this is a blob
			if i < len(parts)-1 {
				log.Println("file found, but more elements remaining", parts[i+1], i, len(parts))
				return nil, errors.New("file found but can not navigate into it")
			}

			fe.content, _ = fs.File(path, rev)
		}

		dir = path

	}

	log.Printf("Get (at exit): path %s type %s\n", path, fe.Type())

	if fe.typ != "dir" {
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
		fe.content, _ = fs.File(indexFile, rev)
		fe.typ, _ = Type(fs, indexFile, rev)
		fe.name = indexFile
	}

	fe.tree = data
	fe.info = ls

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
