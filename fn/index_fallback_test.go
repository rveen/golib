package fn

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// siteRoot locates test/site relative to this source file, so the tests do not
// depend on an absolute path or on the working directory.
func siteRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate the test source file")
	}
	return filepath.Join(filepath.Dir(thisFile), "test", "site")
}

// A request for a file that is not there must fail, even when the directory
// holding it has an index.
//
// This is a regression test for a real fault. `get` treated "this part does not
// exist" as "then serve the directory's index", whatever the part looked like.
// A browser asking for `app/bundle.js` after a rename got `app/index.html` back
// under a 200, rejected it because a module script may not be `text/html`, and
// reported a MIME error — which sends whoever reads it looking at server
// configuration instead of at the path that was wrong. Every missing asset was
// silently a success.
func TestMissingFileDoesNotFallBackToIndex(t *testing.T) {
	root := siteRoot(t)

	for _, path := range []string{
		"app/missing.js",
		"app/bundle.js.map",
		"app/nope.css",
		"app/sheaf.wasm",
		"app/index.htmlx",
	} {
		t.Run(path, func(t *testing.T) {
			f := New(root)
			err := f.Get(path)
			if err == nil {
				t.Fatalf("got %s (type %s) with no error; want a 404", f.Path, f.Type)
			}
			if !strings.Contains(err.Error(), "404") {
				t.Fatalf("err = %v; want a 404", err)
			}
		})
	}
}

// The file that is there must still be served, or the guard has gone too far.
func TestExistingFileIsStillServed(t *testing.T) {
	root := siteRoot(t)

	f := New(root)
	if err := f.Get("app/bundle.js"); err != nil {
		t.Fatalf("Get(app/bundle.js) = %v; want nil", err)
	}
	if filepath.Base(f.Path) != "bundle.js" {
		t.Fatalf("path = %s; want the bundle itself", f.Path)
	}
	if !strings.Contains(string(f.Content), "answer = 42") {
		t.Fatalf("content = %q; want the bundle's contents", f.Content)
	}
}

// A directory still resolves to its index. The fallback is not being removed,
// only kept away from requests that name a file.
func TestDirectoryStillResolvesToItsIndex(t *testing.T) {
	root := siteRoot(t)

	for _, path := range []string{"app", "app/"} {
		f := New(root)
		if err := f.Get(path); err != nil {
			t.Fatalf("Get(%q) = %v; want nil", path, err)
		}
		if filepath.Base(f.Path) != "index.html" {
			t.Fatalf("Get(%q) resolved to %s; want index.html", path, f.Path)
		}
	}
}

// The feature the fallback exists for: a path element with no extension
// continues into the directory's index document. `docs/cap1` is a section of
// `docs/index.md`, not a file called cap1.
func TestPathContinuesIntoTheIndexDocument(t *testing.T) {
	root := siteRoot(t)

	f := New(root)
	if err := f.Get("docs/cap1"); err != nil {
		t.Fatalf("Get(docs/cap1) = %v; want nil", err)
	}
	if f.Type != "document" {
		t.Fatalf("type = %s; want document", f.Type)
	}
	if filepath.Base(f.Path) != "index.md" {
		t.Fatalf("path = %s; want the index document", f.Path)
	}
}

// A missing path in a directory with no index was already a 404. Pinned so the
// two directory shapes cannot drift apart.
func TestMissingFileInADirectoryWithoutAnIndex(t *testing.T) {
	root := siteRoot(t)

	f := New(root)
	if err := f.Get("plain/missing.js"); err == nil {
		t.Fatalf("got %s with no error; want a 404", f.Path)
	}
}

// The line between "names a file" and "continues into a document".
//
// A dot is not enough on its own: `1.2` is a section number and `v0.98` is a
// version, and both are legitimate document parts. Requiring a letter in the
// extension is what keeps them working, since `filepath.Ext("1.2")` is ".2".
func TestNamesAFile(t *testing.T) {
	files := []string{
		"bundle.js", "style.css", "sheaf.wasm", "app.mjs", "bundle.js.map",
		"photo.JPEG", "archive.tar.gz", "a.b",
	}
	notFiles := []string{
		"cap1", "section", "", "1.2", "v0.98", "3.14", "trailing.", ".", "..",
	}

	for _, name := range files {
		if !namesAFile(name) {
			t.Errorf("namesAFile(%q) = false; want true", name)
		}
	}
	for _, name := range notFiles {
		if namesAFile(name) {
			t.Errorf("namesAFile(%q) = true; want false", name)
		}
	}
}

// A document part carrying a dot must still reach its document, which is the
// case the letter rule in namesAFile exists to protect.
func TestNumberedDocumentPartStillResolves(t *testing.T) {
	root := siteRoot(t)

	f := New(root)
	if err := f.Get("docs/1.2"); err != nil {
		t.Fatalf("Get(docs/1.2) = %v; want it to reach the index document", err)
	}
	if filepath.Base(f.Path) != "index.md" {
		t.Fatalf("path = %s; want the index document", f.Path)
	}
}

// IsDir answers only the literal question. Everything Get resolves — index
// files, guessed extensions, wildcards, document sub-paths — must answer false,
// or a caller using it to decide on a trailing-slash redirect would send clients
// to URLs that do not resolve.
func TestIsDirResolvesNothing(t *testing.T) {
	root := New(siteRoot(t))

	dirs := []string{"app", "app/", "/app", "docs", "plain"}
	for _, path := range dirs {
		if !root.IsDir(path) {
			t.Errorf("IsDir(%q) = false; want true", path)
		}
	}

	notDirs := []string{
		"app/bundle.js",  // a file
		"app/index.html", // a file
		"app/missing.js", // nothing at all
		"docs/cap1",      // a part of the index document, not a path on disk
		"docs/index",     // would resolve to index.md by extension guessing
		"",               // the root, which never needs a redirect
		"/",              //   likewise
		"nosuchdir",      // nothing at all
	}
	for _, path := range notDirs {
		if root.IsDir(path) {
			t.Errorf("IsDir(%q) = true; want false", path)
		}
	}
}

// IsDir must not disturb the node it is called on, since handlers share one
// root across every request.
func TestIsDirLeavesTheReceiverAlone(t *testing.T) {
	root := New(siteRoot(t))
	wantRoot, wantPath, wantType := root.Root, root.Path, root.Type

	root.IsDir("app")

	if root.Root != wantRoot || root.Path != wantPath || root.Type != wantType {
		t.Fatalf(
			"IsDir modified the receiver: Root %q->%q, Path %q->%q, Type %q->%q",
			wantRoot, root.Root, wantPath, root.Path, wantType, root.Type,
		)
	}
	if root.Content != nil || root.Data != nil {
		t.Fatal("IsDir populated the receiver; it must only ask a question")
	}
}

// A directory holding several index.*/readme.* files must serve the one a
// browser asking for the directory expects. Taking the first entry in readdir
// order picked test/site/multi/index.css, and the request for the directory
// came back 200 text/css.
func TestIndexPicksTheServableCandidate(t *testing.T) {
	root := New(siteRoot(t))

	if err := root.Get("multi"); err != nil {
		t.Fatalf("Get(%q) = %v; want the directory index", "multi", err)
	}
	if got := filepath.Base(root.Path); got != "index.html" {
		t.Errorf("Get(%q) served %q; want index.html", "multi", got)
	}
}

// index.* outranks readme.*, and an unlisted extension still qualifies last so
// a directory whose only candidate is index.json keeps resolving.
func TestIndexRank(t *testing.T) {
	better := [][2]string{
		{"index.html", "index.htm"},
		{"index.htm", "index.md"},
		{"index.md", "index.ogdl"},
		{"index.css", "readme.md"},
		{"readme.md", "readme.css"},
	}
	for _, p := range better {
		if indexRank(p[0]) >= indexRank(p[1]) {
			t.Errorf("indexRank(%q)=%d not better than indexRank(%q)=%d",
				p[0], indexRank(p[0]), p[1], indexRank(p[1]))
		}
	}
	for _, name := range []string{"bundle.js", "notindex.html", "index"} {
		if indexRank(name) >= 0 {
			t.Errorf("indexRank(%q) = %d; want -1 (not a candidate)", name, indexRank(name))
		}
	}
}

// A directory with no index.* must stay a directory: Get succeeds, the node is
// still type "dir", Content is empty and Data holds the listing a caller needs
// in order to render it.
func TestDirectoryWithoutIndexKeepsItsListing(t *testing.T) {
	root := New(siteRoot(t))

	if err := root.Get("plain"); err != nil {
		t.Fatalf("Get(%q) = %v; want the directory itself", "plain", err)
	}
	if root.Type != "dir" {
		t.Errorf("Type = %q; want dir", root.Type)
	}
	if len(root.Content) != 0 {
		t.Errorf("Content = %q; want empty for a directory without an index", root.Content)
	}
	if root.Data == nil || len(root.Data.Out) == 0 {
		t.Fatal("Data is empty; the caller has no listing to render")
	}
}
