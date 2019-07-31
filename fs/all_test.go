package fs

import (
	"log"
	"testing"
)

var testDir = "/files/projects/go/src/golib/fs/test"

func TestHtm(t *testing.T) {

	fs := New(testDir)

	ty, _ := Type(fs, "index.htm", "")
	if ty != "text/html" {
		t.Error()
	}
}

func TestSvn(t *testing.T) {
	fs := New(testDir)
	ty, _ := Type(fs, "svn", "")
	if ty != "svn" {
		t.Error()
	}
}

func TestSvnDir(t *testing.T) {
	fs := New(testDir)
	fe, _ := Get(fs, "svn/adir", "")
	log.Println(fe.Type())
}

func TestSvnFile(t *testing.T) {
	fs := New(testDir)
	fe, err := Get(fs, "svn/adir/dir.go", "")
	if err != nil {
		log.Println(err.Error())
		return
	}
	log.Println(string(fe.Content()))
}

func TestGit(t *testing.T) {
	fs := New(testDir)
	ty, _ := Type(fs, "/var/git/ogdl.git", "")
	if ty != "git" {
		t.Error()
	}
}

func TestMd(t *testing.T) {
	fs := New(testDir)
	ty, _ := Type(fs, "test.md", "")
	if ty != "text/markdown" {
		t.Error()
	}
}

func TestOo(t *testing.T) {
	fs := New(testDir)
	ty, _ := Type(fs, "test.pdf", "")
	if ty != "pdf" {
		t.Error()
	}
}

func TestOgdlAtRoot(t *testing.T) {
	fs := New(testDir)
	fe, err := Get(fs, "data/title", "")
	if err != nil {
		t.Error("ogdl data not found")
	}

	if fe.Tree() == nil {
		t.Error("ogdl data not found 2")
	}

	if fe.Tree().Text() != "Yeah!" {
		t.Error(fe.Info().Text())
	}

	if fe.Type() != "data/ogdl" {
		t.Error("incorrect type")
	}
}

func TestGet1(t *testing.T) {
	fs := New(testDir)
	f, err := Get(fs, "test.md", "")
	if err != nil {
		log.Println(err.Error())
	} else {
		log.Printf("%s %s\n", f.Type(), string(f.Content()))
	}
}
