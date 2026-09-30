package nomo

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestRenderTypesets(t *testing.T) {
	page, err := Render("r = 5 cm\nh = 12 cm\nV = r^2*h/3\n", "cone", "/f.woff2")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"<!doctype html>", "<title>cone</title>", "<math", `url("/f.woff2")`} {
		if !bytes.Contains(page, []byte(want)) {
			t.Errorf("page lacks %q:\n%s", want, page)
		}
	}
}

func TestAWorksheetWithErrorsIsStillAPage(t *testing.T) {
	page, err := Render("x = 1 m + 1 s\n", "bad", "")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(page, []byte(`class="error`)) {
		t.Errorf("no diagnostic drawn:\n%s", page)
	}
}

func TestConcurrentRendersDoNotShareAnInstance(t *testing.T) {
	var wg sync.WaitGroup
	errs := make(chan error, 16)
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := Render("a = 2 m\nb = a^2\n", "c", ""); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

// TestSameAsNomoHtml holds the embedded module to the native Nomo CLI, byte for
// byte, over every worked example. It needs a Nomo checkout with its CLI built
// (cargo build --release -p nomo-cli), found at $NOMO_DIR or beside golib, and
// skips without one. Nomo proves native == WebAssembly under Node
// (scripts/compare-html.mjs); this is the same claim under wazero.
func TestSameAsNomoHtml(t *testing.T) {
	dir := os.Getenv("NOMO_DIR")
	if dir == "" {
		dir = filepath.Join("..", "..", "..", "nomo")
	}
	cli := filepath.Join(dir, "target", "release", "nomo")
	examples, _ := filepath.Glob(filepath.Join(dir, "examples", "*.nomo"))
	if _, err := os.Stat(cli); err != nil || len(examples) == 0 {
		t.Skip("no Nomo checkout with a built CLI; set NOMO_DIR")
	}

	const font = "/.nomo/stix-two-math-subset.woff2"
	work := t.TempDir()
	for _, src := range examples {
		name := filepath.Base(src)
		source, err := os.ReadFile(src)
		if err != nil {
			t.Fatal(err)
		}
		copy := filepath.Join(work, name)
		if err := os.WriteFile(copy, source, 0o644); err != nil {
			t.Fatal(err)
		}
		// A worksheet with errors exits non-zero and still writes its page.
		_ = exec.Command(cli, "html", "--mathml", "--font-url", font, copy).Run()
		native, err := os.ReadFile(strings.TrimSuffix(copy, ".nomo") + ".html")
		if err != nil {
			t.Fatalf("%s: nomo html wrote nothing: %v", name, err)
		}

		got, err := Render(string(source), strings.TrimSuffix(name, ".nomo"), font)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !bytes.Equal(got, native) {
			t.Errorf("%s: wazero page differs from nomo html", name)
		}
	}
	t.Logf("%d worksheets compared", len(examples))
}
