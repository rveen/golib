package fn

import (
	"errors"
	"path/filepath"
	"strings"
	"unicode"
)

// Get returns the FNode that corresponds to the path given.
//
// It receives an FNode where only the root of the file system is set.
func (fn *FNode) get(path string, raw, noRead bool) error {

	// Split the path into its parts or elements
	fn.parts = parts(path)

	// fn.Root should be a directory. Load the dir info into fn.
	fn.Path = fn.Root
	fn.Type = "dir"

	// Navigate the file system part

	for fn.n = 0; fn.n < len(fn.parts); fn.n++ {

		part := fn.parts[fn.n]

		if len(part) >= 1 && part[0] == '.' {
			return errors.New(". not allowed in paths")
		}

		if part == "~" {
			fn.dir()
			found := false
			for _, entry := range fn.Data.Out {
				if entry.ThisString() == "_tilde" {
					found = true
					break
				}
			}
			if !found {
				return errors.New("404")
			}
			if fn.Params == nil {
				fn.Params = make(map[string]string)
			}
			fn.n++ // advance past ~ so remainingPath excludes it
			fn.Params["tilde"] = fn.remainingPath()
			fn.n = len(fn.parts) // consume all remaining parts
			fn.Path += "/_tilde"
			fn.Type = "dir"
			break
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
			if noRead {
				return nil
			}
			fn.file()

			if !raw {
				// Process remaining parts in document()
				fn.n++
				if err := fn.document(); err != nil {
					return err
				}
			}
			return nil

		case "data":
			if noRead {
				return nil
			}
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
				// info() has already established that this element is not
				// there. The index fallback exists for a path *continuing into
				// a document* (docs/cap1 -> a section of docs/index.md), not
				// for a request naming a file: answering GET /app/missing.js
				// with app/index.html is a 200 carrying the wrong bytes, which
				// is worse than a 404 because nothing downstream can see it.
				if namesAFile(part) {
					return errors.New("404")
				}
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
		return fn.document()

	case "file":
		if fn.n != len(fn.parts) {
			return errors.New("404 (extra path after file)")
		}
		if noRead {
			return nil
		}
		return fn.file()

	case "dir":
		err := fn.dir()
		if err == nil && fn.index() {
			// Only when index() moved fn.Path onto a real file. Without an
			// index the node stays a directory: fn.Data holds the listing and
			// fn.Content stays empty, which is what a caller rendering a
			// directory listing needs. Calling file() here would only be an
			// os.ReadFile on a directory, discarded.
			fn.processFile(raw, noRead)
		}
		return err
	}

	return errors.New("404 (eop)")
}

func (fn *FNode) processFile(raw, noRead bool) error {

	if noRead {
		return nil
	}

	err := fn.file()

	if raw || err != nil {
		return err
	}

	switch fn.Type {
	case "document":
		return fn.document()
	case "data":
		fn.data()
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

// namesAFile reports whether a path element is asking for a file by name,
// which is what separates GET /app/missing.js from GET /docs/cap1.
//
// The test is a filename extension containing at least one letter, not merely
// a dot. filepath.Ext("1.2") is ".2", so a plain dot test would turn docs/1.2
// -- a section number, a legitimate document part -- into a 404. Same for
// docs/v0.98. Requiring a letter keeps those resolving while catching .js,
// .css, .wasm, .map and the rest.
func namesAFile(part string) bool {
	ext := filepath.Ext(part)
	if len(ext) < 2 {
		return false
	}
	for _, r := range ext[1:] {
		if unicode.IsLetter(r) {
			return true
		}
	}
	return false
}
