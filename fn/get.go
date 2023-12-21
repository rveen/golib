package fn

import (
	"errors"
	"golib/gstore/directory"

	"strings"
)

// Get returns the FNode that corresponds to the path given.
//
// It receives an FNode where only the root of the file system is set.
//
func (fn *FNode) get(path string, raw bool) error {

	// Split the path into its parts or elements
	fn.Parts = directory.Parts(path)

	// fn.Root should be a directory. Load the dir info into fn.
	fn.Path = fn.Root
	fn.Type = "dir"

	// Navigate the fyle system part

	for fn.N = 0; fn.N < len(fn.Parts); fn.N++ {

		part := fn.Parts[fn.N]

		if len(part) >= 1 && part[0] == '.' {
			return errors.New(". not allowed in paths")
		}

		savePath := fn.Path
		fn.Path += "/" + part
		fn.Type = fn.info()

		switch fn.Type {
		case "dir":
			// continue

		case "svn":
			// Create a new fn to return the SVN part
			fn2 := New(fn.Path)
			err := fn2.svnGet(fn.remainingPath())
			*fn = *fn2
			return err

		case "file":
			// Blobs cannot be further navigated into, only data and document files,
			// which are detected by fn.info()
			break

		case "document":
			fn.file()

			if !raw {
				// Process remaining parts in document()
				fn.N++
				fn.document()
			}
			return nil

		case "data":
			fn.file()

			if !raw {
				// Process remaining parts in data()
				fn.data()
			}
			return nil

		case "":
			// A part has been found that is not directly in the upper directory
			// Cases:
			// - _path: an entry in the directory
			// - index.* file
			fn.Path = savePath
			fn.dir()

			genericPart := fn.generic()
			if genericPart == "" {
				if !fn.index() {
					return errors.New("404")
				}
			} else {
				fn.Path += "/" + genericPart
			}
		}
	}

	// log.Println("fn.get", fn.Path, fn.Type)

	switch fn.Type {

	case "file":
		if fn.N != len(fn.Parts) {
			return errors.New("404 (extra path after file)")
		}
		return fn.file()

	case "dir":
		err := fn.dir()
		if err == nil {
			fn.index()
			fn.processFile(raw)
		}
		return err
	}

	return errors.New("404 (eop)")
}

func (fn *FNode) processFile(raw bool) error {

	err := fn.file()

	if raw || err != nil {
		return err
	}

	switch fn.Type {

	case "document":
		/* TODO return*/ fn.document()
	case "data":
		/* TODO return*/ fn.data()
	}
	return nil
}

func (fn *FNode) generic() string {

	for _, entry := range fn.Data.Out {
		token := entry.ThisString()

		if token[0] == '_' {
			if fn.Params == nil {
				fn.Params = make(map[string]string)
			}

			// Forced, not so elegant (because it could also be a file)
			fn.Type = "dir"

			if strings.HasSuffix(token, "_end") {
				fn.Params[token[1:len(token)-4]] = fn.remainingPath()
				fn.Parts[fn.N] = token
				fn.N = len(fn.Parts)
				return token
			} else {
				fn.Params[token[1:]] = fn.Parts[fn.N]
				fn.Parts[fn.N] = token
				return token
			}
		}
	}
	return ""
}
