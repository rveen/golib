package fn

import (
	"fmt"
	"testing"
)

func TestGetDir(t *testing.T) {

	fnode := New("/files/go/src/github.com/rveen/golib/fn/test")

	err := fnode.Get("adir")
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	fmt.Println(fnode.Path)
	fmt.Println(fnode.Type)
	fmt.Println(fnode.Data.Text())
}

func TestGetFile(t *testing.T) {

	fnode := New("/files/go/src/github.com/rveen/golib/fn/test")

	err := fnode.Get("test.go")
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	fmt.Println(fnode.Path)
	fmt.Println(fnode.Type)
	fmt.Println(string(fnode.Content))
}

func TestGetData(t *testing.T) {

	fnode := New("/files/go/src/github.com/rveen/golib/fn/test")

	err := fnode.Get("data.ogdl")
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	fmt.Println(fnode.Path)
	fmt.Println(fnode.Type)
	fmt.Println(fnode.Data.Text())
}

func TestGetDoc(t *testing.T) {

	fnode := New("/files/go/src/github.com/rveen/golib/fn/test")

	err := fnode.Get("test.md")
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	fmt.Println(fnode.Path)
	fmt.Println(fnode.Type)
	fmt.Println(fnode.Document.Html())
}

func TestGetDoc1(t *testing.T) {

	fnode := New("/files/go/src/github.com/rveen/golib/fn/test")

	err := fnode.Get("doc")
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	fmt.Println(fnode.Path)
	fmt.Println(fnode.Type)
	fmt.Println(fnode.Document.Html())
}

func TestGetDocData(t *testing.T) {

	fnode := New("/files/go/src/github.com/rveen/golib/fn/test")

	err := fnode.Get("doc/_")
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	fmt.Println(fnode.Path)
	fmt.Println(fnode.Type)
	fmt.Println(fnode.Data.Text())
}

func TestGetDocPart(t *testing.T) {

	fnode := New("/files/go/src/github.com/rveen/golib/fn/test")

	err := fnode.Get("doc/cap1")
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	fmt.Println(fnode.Path)
	fmt.Println(fnode.Type)
	fmt.Println(fnode.Document.Html())
}
