package fs

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/rveen/golib/fs/types"
	"github.com/rveen/ogdl"
)

// Type examines the path and returns its type. Path should be an existing directory
// or file in the file system.
/*
func Type(fs FileSystem, path, rev string) (string, error) {

	if path[len(path)-1] == '@' {
		return "revs", nil
	}

	f, err := fs.Info(path, rev)
	if err != nil {
		return "", err
	}

	if !f.IsDir() {
		// return its type by looking at the extension
		s := types.TypeByExtension(f.Name)
		if s == "" {
			return "file", nil
		}
		return s, nil
	}

	ff, err := fs.Dir(path, rev)
	if err != nil {
		return "", err
	}

	// Git
	sscore := 0
	gscore := 0
	for _, f := range ff {

		switch f.Name() {
		case "format":
			sscore++
			if sscore > 1 {
				return "svn", nil
			}
		case "hooks":
			sscore++
			gscore++
			if sscore > 1 {
				return "svn", nil
			}
			if gscore > 1 {
				return "git", nil
			}
		case "HEAD":
			gscore++
			if gscore > 1 {
				return "svn", nil
			}
		}
	}
	return "dir", nil
}
*/
// Index checks if there are index.* files, and the dir info (list).
//
// - index.ogdl -> graph
// - index.* -> string (if there are several, take highest in the list (htm, md, ...)
// - dir info -> graph.dir (only if index.nolist is not found)
func Index(fs FileSystem, path, rev string) (string, *ogdl.Graph, *ogdl.Graph) {

	// Read the directory
	ff, err := fs.Dir(path, rev)

	if err != nil {
		return "", nil, nil
	}

	var g *ogdl.Graph
	indexFile := ""
	nodir := false

	// Read any index.* files
	for _, f := range ff {
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
				return "", nil, nil
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
	for _, f := range ff {
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
	for _, f := range ff {
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
