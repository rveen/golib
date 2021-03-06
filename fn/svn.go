package fn

import (
	"errors"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/rveen/ogdl"
	"github.com/rveen/ogdl/io/gxml"
)

func (fn *FNode) svnGet(path string) error {

	/*revs := false
	if len(path) > 1 && path[len(path)-1] == '@' {
		revs = true
	}*/

	log.Printf("svnGet: [%s] [%s] [%s]\n", fn.Base, fn.Path, path)

	for {
		err := fn.svnNavigate(path)
		if err != nil {
			return err
		}

		left := len(fn.Parts) - fn.N

		switch fn.Type {

		case "document":
			fn.svnFile()
			fn.document()
			return nil

		case "data":
			fn.svnFile()
			fn.data()
			return nil

		case "dir":
			// check _token
			// check index / readme
			fn.svnDir()
			return nil

		case "log":
			fn.svnLog()
			return nil

		case "file":
			if left != 0 {
				return errors.New("not navigable")
			}
			return fn.svnFile()

		default:
			return errors.New("unknown type " + fn.Type)
		}
	}
}

func (fn *FNode) svnNavigate(path string) error {

	// Handle revisions
	// @ at the end means list revisions (in fn.Data, fn.Type => "log")
	// else, part@token at any place in the path defines the revision to get.

	if len(path) > 0 && path[0] == '/' {
		path = path[1:]
	}

	revs := false
	if strings.HasSuffix(path, "@") {
		revs = true
		path = path[:len(path)-1]
	}

	if path != "" {
		fn.Parts = strings.Split(path, "/")
	}
	fn.N = 0
	fn.Path = ""

	// Extract revision if any
	fn.Revision = ""
	for i, part := range fn.Parts {
		n := strings.IndexByte(part, '@')
		if n != -1 {
			log.Println("nav: revision found", n, part[:n], part[n+1:], part)
			fn.Parts[i] = part[:n]
			fn.Revision = part[n+1:]
			break
		}
	}

	// Go step by step, we don't know where the file path ends.
	for _, part := range fn.Parts {

		saveThis := fn.Path
		fn.Path += "/" + part
		fn.Path = filepath.Clean(fn.Path)

		typ := fn.svnType()

		if typ == "" {
			fn.Path = saveThis
			return nil
		}

		fn.Type = typ
		fn.N++

		if typ == "file" {
			// Cannot navigate into a file here
			if revs {
				fn.Type = "log"
			}
			return nil
		}
	}

	fn.Type = fn.svnType()
	if revs {
		fn.Type = "log"
	}
	log.Printf("svnNav: [%s] [%s] [%s]\n", fn.Base, fn.Path, fn.Type)

	return nil
}

// svnFile loads the file in fn.Path into fn.Content
func (fn *FNode) svnFile() error {

	var err error

	log.Printf("svnFile [%s|%s] [%s]\n", fn.Base, fn.Path, fn.Revision)

	if fn.Revision == "" || fn.Revision == "HEAD" {
		fn.Content, err = exec.Command("svnlook", "cat", fn.Base, fn.Path).Output()
	} else {
		fn.Content, err = exec.Command("svnlook", "-r", fn.Revision, "cat", fn.Base, fn.Path).Output()

	}

	return err
}

func (fn *FNode) svnDir() error {

	path := fn.Path
	if path == "" {
		path = "."
	}

	rev := fn.Revision
	if rev == "" {
		rev = "HEAD"
	}

	b, err := exec.Command("svn", "list", "--xml", "-r", rev, "file:///"+fn.Base+"/"+path).Output()

	if err != nil {
		return err
	}

	g := gxml.FromXML(b)
	g = g.Get("lists.list")
	dd := ogdl.New(nil)
	for _, e := range g.Out {
		if e.ThisString() != "entry" {
			continue
		}
		d := dd.Add(e.Node("name").String())
		d.Add("name").Add(e.Node("name").String())
		if e.Node("@kind").String() == "dir" {
			d.Add("type").Add("dir")
		} else {
			d.Add("type").Add("file")
		}

	}
	fn.Data = dd
	log.Println("svnDir", fn.Base, fn.Path, dd.Text())
	return nil
}

func (fn *FNode) svnLog() error {

	log.Printf("svnLog [%s] [%s] [%s]", fn.Base, fn.Path, fn.Revision)

	path := fn.Path
	if path != "" && path[0] == '/' {
		path = path[1:]
	}

	var b []byte
	var err error

	if fn.Revision == "" || fn.Revision == "HEAD" {
		b, err = exec.Command("svn", "log", "-v", "--xml", "file:///"+fn.Base, path).Output()
	} else {
		b, err = exec.Command("svn", "log", "-r", fn.Revision, "-v", "--xml", "file:///"+fn.Base, path).Output()
	}

	if err != nil {
		log.Println("svnLog error", err.Error())
		return err
	}

	g := gxml.FromXML(b)

	if g == nil || g.Len() == 0 {
		log.Println("svnLog error, g empty")
		return errors.New("no svn log")
	}

	// Trace the path (it may have been moved)
	// Get all the revision numbers from the previous log, issue an svn info
	// and get the relative url

	for _, n := range g.Node("log").Out {

		rev := n.Node("@revision").String()

		b, err = exec.Command("svn", "info", "--xml", "-r", rev, "file:///"+fn.Base+"/"+fn.Path).Output()
		m := gxml.FromXML(b)

		fmt.Println(m.Text())

		rel := m.Get("info.entry.relative_url'").String()
		if len(rel) > 0 && rel[0] == '^' {
			rel = rel[1:]
		}
		n.Set("urlRel", rel)
		url := m.Get("info.entry.url").String()
		n.Set("url", url)
		n.Set("urlBase", url[7:len(url)-len(rel)])
	}

	fn.Data = g
	return err
}

// Info returns metadata on the path. When no revision is given, the latest one
// is taken. If a revision is specified, the path given can be either the current
// one if it exists (the function looks up the historical one) or the path that
// existed at the moment the revision was made.
//
// The command line tools svn and svnlook from the subversion distribution don't
// give all the required information that we need.
// 'svn info' needs an existing path in order to return info on old versions of
// that path. If a path doesn't exist anymore, specifying a revision will not help
// and no info is returned.
// 'svnlook info' on the other hand doesn't return info on paths, only on releases
// 'svnlook meta' is a modification that gives info on paths as they are at the time
// of the release. See https://github.com/rveen/subversion
//
func (fn *FNode) svnInfo() *ogdl.Graph {

	path := fn.Path
	if path == "" {
		path = "."
	}

	var err error
	var b []byte

	if fn.Revision == "" || fn.Revision == "HEAD" {
		b, err = exec.Command("svnlook", "meta", fn.Base, path).Output()
	} else {
		b, err = exec.Command("svnlook", "meta", "-r", fn.Revision, fn.Base, path).Output()
	}

	if err != nil {
		return nil
	}

	return ogdl.FromBytes(b)
}

// Return 'dir', 'file', 'document', 'data' or ''
func (fn *FNode) svnType() string {

	path := fn.Path
	if path == "" {
		path = "."
	}

	var err error
	var b []byte

	if fn.Revision == "" || fn.Revision == "HEAD" {
		b, err = exec.Command("svnlook", "meta", fn.Base, path).Output()
	} else {
		b, err = exec.Command("svnlook", "meta", "-r", fn.Revision, fn.Base, path).Output()
	}

	if err != nil {
		return ""
	}
	if strings.HasPrefix(string(b), "kind dir") {
		return "dir"
	}
	return fn.fileType()
}
