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
package svn

import (
	"bytes"
	"encoding/xml"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"github.com/rveen/ogdl"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

type fileSystem struct {
	root string
}

func New(root string) *fileSystem {
	fs := &fileSystem{root: root}
	fs.root, _ = filepath.Abs(root)

	log.Println("fs.New", fs.root)

	return fs
}

func (fs *fileSystem) Root() string {
	return fs.root
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

func (fs *fileSystem) Log(path string, rev int) (*ogdl.Graph, error) {

	var err error
	var b []byte

	if rev > -1 {
		b, err = exec.Command("svn", "log", "-r", strconv.Itoa(rev), "-v", "--xml", "file:///"+fs.root, path).Output()
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

	if rev != "" {
		b, err = exec.Command("svnlook", "-r", rev, "cat", fs.root, path).Output()
	} else {
		b, err = exec.Command("svnlook", "cat", fs.root, path).Output()
	}

	return b, err
}

// Size returns the size in bytes of a file
// In case the path is not a file it returns -1.
func (fs *fileSystem) size(path, rev string) int64 {

	var b []byte
	var err error
	var i int

	if rev != "" {
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

func (fs *fileSystem) Info(path, rev string) (os.FileInfo, error) {

	log.Println("svnfs.Info()", path)

	if rev == "" {
		rev = "HEAD"
	}

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

	if rev == "" {
		rev = "HEAD"
	}

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
