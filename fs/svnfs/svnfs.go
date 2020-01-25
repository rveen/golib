// (C) R.Veen, 2012-2018.
// License: BSD.

// Subversion API for accessing LOCAL server repositories.
//
// Uses the svn, svnlook, svnadmin and svnmucc commands
// as interface method. The svnmucc command is used
// to alter the repository without a local working copy.
//
// Using the SVN C API would sure be faster, and a pending
// exercice for the future, but the current solution is
// simpler.
//
package svnfs

import (
	"bytes"
	"encoding/xml"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"

	. "github.com/rveen/golib/fs"
	"github.com/rveen/ogdl"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

type fileSystem struct {
	root string
	typ  string
}

func New(root string) *fileSystem {
	fs := &fileSystem{root: root}
	fs.root, _ = filepath.Abs(root)

	log.Println("svnfs.New", fs.root)

	return fs
}

func (fs *fileSystem) Root() string {
	return fs.root
}

func (fs *fileSystem) Type() string {
	return "svn"
}

// svn propset -r N --revprop PROPNAME PROPVALUE URL
// (PROPNAME = svn:log for the log message)
func (fs *fileSystem) SetRevisionProperty(path string, name string, value string, rev int) error {

	var r string

	if rev >= 0 {
		r = strconv.Itoa(rev)
	} else {
		r = "HEAD"
	}

	return exec.Command("svn", "propset", "-r", r, "--revprop", name, value, "file:///"+fs.root+"/"+path).Run()
}

// svn propget -r N --revprop PROPNAME URL
func (fs *fileSystem) RevisionProperty(path string, name string, rev int) error {

	var r string

	if rev >= 0 {
		r = strconv.Itoa(rev)
	} else {
		r = "HEAD"
	}

	return exec.Command("svn", "propget", "-r", r, "--revprop", name, "file:///"+fs.root+"/"+path).Run()
}

// Create a repository at the given location. If already present, do nothing.
// If the ref parameter is not empty, the new repo is a copy of ref
// The group and user of the files created are set to the values given.
func Create(path, ref, user, group string) error {

	var err error

	if len(ref) != 0 {
		err = exec.Command("cp -a", ref, path).Run()

	} else {
		err = exec.Command("svnadmin", "create", path).Run()
	}

	if err != nil {
		return err
	}

	if len(user) != 0 && len(group) != 0 {
		err = exec.Command("chown", "-R", group+":"+user, path).Run()
	}

	return err
}

func (fs *fileSystem) Log(path, rev string) (*ogdl.Graph, error) {

	var err error
	var b []byte

	if rev != "HEAD" {
		b, err = exec.Command("svn", "log", "-r", rev, "-v", "--xml", "file:///"+fs.root, path).Output()
	} else {
		b, err = exec.Command("svn", "log", "-v", "--xml", "file:///"+fs.root, path).Output()
	}

	if err != nil {
		return nil, err
	}

	g := xml2graph(b)

	return g, nil
}

func (fs *fileSystem) File(path, rev string) ([]byte, error) {

	var b []byte
	var err error

	if rev != "HEAD" {
		b, err = exec.Command("svnlook", "-r", rev, "cat", fs.root, path).Output()
	} else {
		b, err = exec.Command("svnlook", "cat", fs.root, path).Output()
	}

	log.Println("svnfs.File()", path, rev, len(b))

	return b, err
}

func (fs *fileSystem) Revisions(path, rev string) (*ogdl.Graph, error) {

	if path[len(path)-1] == '@' {
		path = path[:len(path)-1]
	}
	g, err := fs.Log(path, rev)
	if err != nil {
		return g, err
	}

	// Trace the path (it may have been moved)
	// Get all the revision numbers from the previous log, issue an svn info
	// and get the relative url

	var b []byte

	for _, n := range g.Node("log").Out {

		rev, _ := n.GetString("'@'.revision")

		b, err = exec.Command("svn", "info", "--xml", "-r", rev, "file:///"+fs.root+"/"+path).Output()
		m := xml2graph(b)
		rel := m.Get("info.entry.'relative-url'").String()
		if rel[0] == '^' {
			rel = rel[1:]
		}
		n.Set("urlRel", rel)
		url := m.Get("info.entry.url").String()
		n.Set("url", url)
		n.Set("urlBase", url[7:len(url)-len(rel)])
	}

	return g, err
}

// Size returns the size in bytes of a file
// In case the path is not a file it returns -1.
func (fs *fileSystem) size(path, rev string) int64 {

	var b []byte
	var err error
	var i int

	if rev != "HEAD" {
		b, err = exec.Command("svnlook", "-r", rev, "filesize", fs.root, path).Output()
	} else {
		b, err = exec.Command("svnlook", "filesize", fs.root, path).Output()
	}

	if err != nil {
		return -1
	}

	i, err = strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		return -1
	}

	return int64(i)
}

func (fs *fileSystem) Get(path, rev string) (*fileEntry, error) {

	var err error
	fe := &fileEntry{}

	if rev == "" {
		rev = "HEAD"
	}

	// Prepare and clean path
	path = filepath.Clean(path)

	if path[len(path)-1] == '@' {
		fe.tree, err = fs.Revisions(path[:len(path)-1], rev)
		fe.typ = "revs"
		fe.name = path
		return fe, err
	}

	fi, err := fs.Info(path, rev)

	if fi == nil {
		return nil, err
	}

	fe = fi.(*fileEntry)

	switch fe.Type() {

	case "dir":

		indexFile, data, ls := fs.Index(path, rev)

		if indexFile != "" {
			fe.content, _ = fs.File(indexFile, rev)
			//fe.typ, _ = Type(fs, indexFile, rev)
			fe.name = indexFile
		}

		fe.tree = data
		fe.info = ls

	case "file":
		fe.content, _ = fs.File(path, rev)

	}
	return fe, err
}

// Index checks if there are index.* files, and the dir info (list).
//
// - index.ogdl -> graph
// - index.* -> string (if there are several, take highest in the list (htm, md, ...)
// - dir info -> graph.dir (only if index.nolist is not found)
func (fs *fileSystem) Index(path, rev string) (string, *ogdl.Graph, *ogdl.Graph) {

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
			d.Add("type").Add(TypeByExtension(filepath.Ext(name)))
			d.Add("name").Add(name)
		}

		if strings.HasPrefix(strings.ToLower(name), "readme.") {
			indexFile = path + "/" + name
		}
	}

	return indexFile, g, dir
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
func (fs *fileSystem) Info(path, rev string) (os.FileInfo, error) {

	if path == "" {
		path = "."
	}

	log.Println("svnfs.Info()", fs.root, path, rev)

	var err error
	var b []byte

	if rev == "" || rev == "HEAD" {
		b, err = exec.Command("svnlook", "meta", fs.root, path).Output()
	} else {
		b, err = exec.Command("svnlook", "meta", "-r", rev, fs.root, path).Output()
	}

	if err != nil {
		return fs.info(path, rev)
	}

	log.Println("svnfs.Info: ", string(b))

	if err != nil {
		return nil, err
	}

	g := ogdl.FromBytes(b)

	fe := &fileEntry{}
	fe.typ = g.Get("kind").String()
	fe.name = path

	log.Println("svnfs.Type", fe.typ)

	if fe.typ != "dir" {
		fe.size = g.Get("size").Int64()
		fe.tree = g
	}

	return fe, nil
}

func (fs *fileSystem) info(path, rev string) (os.FileInfo, error) {

	log.Println("svnfs.Info()", path, rev)

	b, err := exec.Command("svn", "info", "--xml", "-r", rev, "file:///"+fs.root+"/"+path).Output()

	if err != nil {
		return nil, err
	}

	g := xml2graph(b)

	fe := &fileEntry{}
	fe.typ = g.Get("info.entry.'@'.kind").String()

	log.Println("svnfs.Type", fe.typ)

	if fe.typ != "dir" {
		fe.size = fs.size(path, rev)
	}

	return fe, nil
}

func (fs *fileSystem) Dir(path, rev string) ([]os.FileInfo, error) {

	log.Println("svnfs.Dir()", path, rev)

	b, err := exec.Command("svn", "list", "--xml", "-r", rev, "file:///"+fs.root+"/"+path).Output()

	if err != nil {
		return nil, err
	}

	g := xml2graph(b)
	g = g.Get("lists.list")

	var dir []os.FileInfo

	for _, e := range g.Out {

		if e.ThisString() != "entry" {
			continue
		}

		f := &fileEntry{}
		dir = append(dir, f)

		f.name = e.Get("name").String()
		f.size = e.Get("size").Int64()
		f.typ = e.Get("'@'.kind").String()
		f.time, _ = time.Parse(time.RFC3339, e.Get("commit.date").String())
	}

	return dir, nil
}

func xml2graph(b []byte) *ogdl.Graph {

	decoder := xml.NewDecoder(bytes.NewReader(b))

	g := ogdl.New(nil)
	var key string
	level := -1

	att := true

	var stack []*ogdl.Graph
	stack = append(stack, g)

	tr := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)

	for {
		// Read tokens from the XML document in a stream.
		t, _ := decoder.Token()
		if t == nil {
			break
		}
		// Inspect the type of the token just read.
		switch se := t.(type) {

		case xml.StartElement:
			level++

			key = se.Name.Local
			// No accents in key
			key, _, _ = transform.String(tr, key)

			n := stack[len(stack)-1].Add(key)
			// push
			stack = append(stack, n)
			if att && len(se.Attr) != 0 {
				a := n.Add("@")
				for _, at := range se.Attr {
					a.Add(at.Name.Local).Add(at.Value)
				}
			}

		case xml.CharData:

			val := strings.TrimSpace(string(se))
			if len(val) > 0 {
				stack[len(stack)-1].Add(val)
			}

		case xml.EndElement:
			level--
			// pop

			stack = stack[:len(stack)-1]

		}
	}

	return g
}

/*
func getRev(path string) (string, string) {

	i := strings.LastIndex(path, "@")
	if i == -1 {
		return path, "HEAD"
	}
	return path[0:i], path[i+1:]

}
*/