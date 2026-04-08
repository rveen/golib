package fn

import (
	"testing"
)

func TestSvnDir(t *testing.T) {

	fnode := New("/files/go/src/github.com/rveen/golib/fn/test/svn")

	if err := fnode.svnDir(); err != nil {
		t.Fatal(err)
	}

	t.Log(fnode.Data.Text())
}

func TestSvnInfo(t *testing.T) {

	fnode := New("/files/go/src/github.com/rveen/golib/fn/test/svn")

	g, err := fnode.svnInfo()
	if err != nil {
		t.Fatal(err)
	}

	t.Log(g.Text())
}

func TestSvnType(t *testing.T) {

	fnode := New("/files/go/src/github.com/rveen/golib/fn/test/svn")

	fnode.Path = "adir/dir.go"

	t.Log(fnode.svnType())
}

func TestSvnLog(t *testing.T) {

	fnode := New("/files/go/src/github.com/rveen/golib/fn/test/svn")

	fnode.Path = "adir/dir.go"

	if err := fnode.svnLog(); err != nil {
		t.Fatal(err)
	}

	t.Log(fnode.Data.Text())
}
