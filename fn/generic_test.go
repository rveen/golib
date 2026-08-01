package fn

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tildeRoot builds the directory shape that made unknown sub-paths resolve:
//
//	item/_id/index.htm      the item page
//	item/_id/comps.htm      a real sub-page
//	item/_id/_tilde/index.htm   reached by a literal '~' segment only
func tildeRoot(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	dir := filepath.Join(root, "item", "_id", "_tilde")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(path, body string) {
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(root, "item", "_id", "index.htm"), "item page")
	write(filepath.Join(root, "item", "_id", "comps.htm"), "components")
	write(filepath.Join(dir, "index.htm"), "partial")

	return root
}

// A wildcard directory still captures its segment.
func TestGenericCapturesTheSegment(t *testing.T) {
	fnode := New(tildeRoot(t))

	if err := fnode.Get("item/123"); err != nil {
		t.Fatal(err)
	}
	if got := fnode.Params["id"]; got != "123" {
		t.Fatalf("Params[id] = %q, want 123", got)
	}
	if got := string(fnode.Content); got != "item page" {
		t.Fatalf("content = %q, want the item page", got)
	}
}

// A named sub-page resolves to itself, not to the wildcard.
func TestKnownSubPathResolves(t *testing.T) {
	fnode := New(tildeRoot(t))

	if err := fnode.Get("item/123/comps"); err != nil {
		t.Fatal(err)
	}
	if got := string(fnode.Content); got != "components" {
		t.Fatalf("content = %q, want the components page", got)
	}
}

// A sub-path that names nothing must fail, not fall into _tilde.
//
// It used to resolve to item/_id/_tilde and render that page with HTTP 200, so
// a typo — or a URL from documentation that no longer exists — looked like it
// had worked.
func TestUnknownSubPathIsNotFound(t *testing.T) {
	fnode := New(tildeRoot(t))

	err := fnode.Get("item/123/no-such-page")
	if err == nil {
		t.Fatalf("unknown sub-path resolved to %q with content %q, want an error",
			fnode.Path, string(fnode.Content))
	}
	if !strings.Contains(err.Error(), "404") {
		t.Fatalf("err = %v, want a 404", err)
	}
}

// _tilde is still reachable the way it is meant to be: by a literal '~'.
func TestTildeStillRoutes(t *testing.T) {
	fnode := New(tildeRoot(t))

	if err := fnode.Get("item/123/~/some/section"); err != nil {
		t.Fatal(err)
	}
	if got := fnode.Params["tilde"]; got != "some/section" {
		t.Fatalf("Params[tilde] = %q, want some/section", got)
	}
	if !strings.Contains(fnode.Path, "/_tilde/") {
		t.Fatalf("Path = %q, want it under /_tilde", fnode.Path)
	}
}
