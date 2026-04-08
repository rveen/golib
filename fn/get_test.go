package fn

import (
	"testing"
)

const testRoot = "/files/go/src/github.com/rveen/golib/fn/test"

func TestGetDir(t *testing.T) {
	t.Skip("test/ fixtures not present")

	fnode := New(testRoot)

	if err := fnode.Get("adir"); err != nil {
		t.Fatal(err)
	}

	t.Log(fnode.Path)
	t.Log(fnode.Type)
	t.Log(fnode.Data.Text())
}

func TestGetFile(t *testing.T) {
	t.Skip("test/ fixtures not present")

	fnode := New(testRoot)

	if err := fnode.Get("test.go"); err != nil {
		t.Fatal(err)
	}

	t.Log(fnode.Path)
	t.Log(fnode.Type)
	t.Log(string(fnode.Content))
}

func TestGetData(t *testing.T) {
	t.Skip("test/ fixtures not present")

	fnode := New(testRoot)

	if err := fnode.Get("data.ogdl"); err != nil {
		t.Fatal(err)
	}

	t.Log(fnode.Path)
	t.Log(fnode.Type)
	t.Log(fnode.Data.Text())
}

func TestGetDoc(t *testing.T) {
	t.Skip("test/ fixtures not present")

	fnode := New(testRoot)

	if err := fnode.Get("test.md"); err != nil {
		t.Fatal(err)
	}

	t.Log(fnode.Path)
	t.Log(fnode.Type)
	t.Log(fnode.Document.Html())
}

func TestGetDoc1(t *testing.T) {
	t.Skip("test/ fixtures not present")

	fnode := New(testRoot)

	if err := fnode.Get("doc"); err != nil {
		t.Fatal(err)
	}

	t.Log(fnode.Path)
	t.Log(fnode.Type)
	t.Log(fnode.Document.Html())
}

func TestGetDocData(t *testing.T) {
	t.Skip("test/ fixtures not present")

	fnode := New(testRoot)

	if err := fnode.Get("doc/_"); err != nil {
		t.Fatal(err)
	}

	t.Log(fnode.Path)
	t.Log(fnode.Type)
	t.Log(fnode.Data.Text())
}

func TestGetDocPart(t *testing.T) {
	t.Skip("test/ fixtures not present")

	fnode := New(testRoot)

	if err := fnode.Get("doc/cap1"); err != nil {
		t.Fatal(err)
	}

	t.Log(fnode.Path)
	t.Log(fnode.Type)
	t.Log(fnode.Document.Html())
}
