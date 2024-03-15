package fn

import (
	"errors"
	"strings"
)

// Get returns the FNode that corresponds to the path given.
//
// It receives an FNode where only the root of the file system is set.
func (fn *FNode) get(path string, raw bool) error {

	// Split the path into its parts or elements
	fn.parts = parts(path)

	// fn.Root should be a directory. Load the dir info into fn.
	fn.Path = fn.Root
	fn.Type = "dir"

	// Navigate the fyle system part

	for fn.n = 0; fn.n < len(fn.parts); fn.n++ {

		part := fn.parts[fn.n]

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
			fn.n++
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
				fn.n++
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

	switch fn.Type {

	case "document":
		if fn.n != len(fn.parts) {
			// ???
		}
		fn.document()
		return nil

	case "file":
		if fn.n != len(fn.parts) {
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
				fn.parts[fn.n] = token
				fn.n = len(fn.parts)
				return token
			} else {
				fn.Params[token[1:]] = fn.parts[fn.n]
				fn.parts[fn.n] = token
				return token
			}
		}
	}
	return ""
}

func parts(path string) []string {
	ss := strings.Split(path, "/")
	var st []string
	for _, s := range ss {
		if s != "" {
			st = append(st, s)
		}
	}
	return st
}
