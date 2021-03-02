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
	"log"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/rveen/golib/fs/types"

	"github.com/rveen/ogdl"
	"github.com/rveen/ogdl/v2/io/gxml"
)

type fileSystem struct {
	root string
	typ  string
}

func New(root string) *fileSystem {
	fs := &fileSystem{root: root}
	fs.root, _ = filepath.Abs(root)
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

	g := gxml.FromXML(b)

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
		m := gxml.FromXML(b)
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

func (fs *fileSystem) Get(path, rev string) (*types.FileEntry, error) {

	log.Println("svn.Get", path, rev)

	var err error
	fe := &types.FileEntry{}

	if rev == "" {
		rev = "HEAD"
	}

	// Prepare and clean path
	path = filepath.Clean(path)

	if path[len(path)-1] == '@' {
		fe.Data, err = fs.Revisions(path[:len(path)-1], rev)
		fe.Typ = "revs"
		fe.Name = path
		return fe, err
	}

	fi, err := fs.Info(path, rev)

	if fi == nil {
		fe = &types.FileEntry{}
	} else {
		fe = fi
	}

	switch fe.Typ {

	case "dir":

		err := fs.Index(fe, path, rev)

		if err != nil {

		}

	case "file":
		fe.Content, _ = fs.File(path, rev)
		fe.Name = path
		fe.Prepare()

	}
	return fe, err
}

// Index checks if there are index.* files, and the dir info (list).
//
// - index.ogdl -> graph
// - index.* -> string (if there are several, take highest in the list (htm, md, ...)
// - dir info -> graph.dir (only if index.nolist is not found)
// Index checks if there are index.* files in the given directory
//
// - index.ogdl -> graph
// - index.* -> string (if there are several, take highest in the list (htm, md, ...)
// - dir info -> graph.dir (only if index.nolist is not found)
//
// This function assumes that the input file entry already holds the directory
// listing (in d.Dir, as []os.FileInfo).
//
// The same file entry is used as output. It just adds the content of the index file
// if found.
//
func (fs *fileSystem) Index(d *types.FileEntry, path, rev string) error {

	log.Println("svn.Index", path)

	// Read the directory
	nodir := false

	ff, err := fs.Dir(path, rev)
	if err != nil {
		return err
	}
	d.Content = nil

	// Read any index.* files
	for _, f := range ff {
		name := f.Name

		if name == "index.link" {
			continue
		}

		if name == "index.nolist" {
			nodir = true
			continue
		}

		if name == "index.ogdl" {
			b, err := fs.File(path+"/index.ogdl", rev)
			if err != nil {
				return err
			}
			d.Data = ogdl.FromString(string(b))
			continue
		}

		// Index files overwrite readme's
		if strings.HasPrefix(name, "index.") {
			b, _ := fs.File(path+"/"+name, rev)
			d.Content = b
			d.Name = path + "/" + name
			d.Prepare()
			continue
		}

		// The readme file is only read in if no index file is present
		if d.Content == nil && strings.HasPrefix(strings.ToLower(name), "readme.") {
			b, _ := fs.File(path+"/"+name, rev)
			d.Content = b
			d.Name = path + "/" + name
			d.Prepare()
		}
	}

	if nodir {
		return nil
	}

	// Read dir info

	dir := ogdl.New("dir")

	// Add directories to the list, but not those starting with . or _
	for _, f := range ff {
		gd := dir.Add("-")
		gd.Add("name").Add(f.Name)
		gd.Add("type").Add(f.Typ)
	}

	d.Data = dir
	return nil
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
func (fs *fileSystem) Info(path, rev string) (*types.FileEntry, error) {

	if path == "" {
		path = "."
	}

	log.Println("svnfs.Info", fs.root, path, rev)

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

	fe := &types.FileEntry{}
	fe.Typ = g.Get("kind").String()
	fe.Name = path

	log.Println("svnfs.Type", fe.Typ)

	if fe.Typ != "dir" {
		fe.Size = g.Get("size").Int64()
		fe.Data = g
	}

	return fe, nil
}

func (fs *fileSystem) info(path, rev string) (*types.FileEntry, error) {

	//log.Println("svnfs.info: command: svn info --xml -r", rev, "file:///"+fs.root+"/"+path)

	b, err := exec.Command("svn", "info", "--xml", "-r", rev, "file:///"+fs.root+"/"+path).Output()

	if err != nil {
		return nil, err
	}

	g := gxml.FromXML(b)

	fe := &types.FileEntry{}
	fe.Typ = g.Get("info.entry.'@'.kind").String()

	log.Println("svnfs.Type", fe.Typ)

	if fe.Typ != "dir" {
		fe.Size = fs.size(path, rev)
	}

	return fe, nil
}

func (fs *fileSystem) Dir(path, rev string) ([]*types.FileEntry, error) {

	log.Println("svnfs.Dir()", path, rev)

	b, err := exec.Command("svn", "list", "--xml", "-r", rev, "file:///"+fs.root+"/"+path).Output()

	if err != nil {
		return nil, err
	}

	g := gxml.FromXML(b)
	g = g.Get("lists.list")

	var dir []*types.FileEntry

	for _, e := range g.Out {

		if e.ThisString() != "entry" {
			continue
		}

		f := &types.FileEntry{}
		dir = append(dir, f)

		f.Name = e.Get("name").String()
		f.Size = e.Get("size").Int64()
		f.Typ = e.Get("'@'.kind").String()
		f.Time, _ = time.Parse(time.RFC3339, e.Get("commit.date").String())
	}

	return dir, nil
}
