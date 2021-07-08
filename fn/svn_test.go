package fn

import (
	"fmt"
	"testing"
)

func TestSvnDir(t *testing.T) {

	fnode := New("/files/go/src/github.com/rveen/golib/fn/test/svn")

	fnode.svnDir()

	fmt.Println(fnode.Data.Text())

}

func TestSvnInfo(t *testing.T) {

	fnode := New("/files/go/src/github.com/rveen/golib/fn/test/svn")

	g := fnode.svnInfo()

	fmt.Println(g.Text())

}

func TestSvnType(t *testing.T) {

	fnode := New("/files/go/src/github.com/rveen/golib/fn/test/svn")

	fnode.Path = "adir/dir.go"

	fmt.Println(fnode.svnType())
}

func TestSvnLog(t *testing.T) {

	fnode := New("/files/go/src/github.com/rveen/golib/fn/test/svn")

	fnode.Path = "adir/dir.go"

	fnode.svnLog()

	fmt.Println(fnode.Data.Text())
}
