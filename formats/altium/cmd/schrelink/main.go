// Command schrelink points the components of Altium schematics to another
// database library (DbLib) and table.
//
// Usage:
//
//	schrelink [options] file.SchDoc ... | project.PrjPcb
//
// A .PrjPcb argument stands for all the schematic documents of the project.
// By default every component placed from a .DbLib/.SVNDbLib is relinked; use
// -from to restrict that to some libraries. Each output is re-read and
// checked to differ from its input only in the library link.
//
// Options:
//
//	-lib name    new library (default "mahle"; ".DbLib" is added when there is no extension)
//	-table name  new database table (default "components"; "" keeps the current table)
//	-from list   comma-separated libraries to relink (default: all database libraries)
//	-n           dry run: report, do not write
//	-inplace     overwrite the inputs, keeping a .bak copy of each
//	-out dir     output directory (default: next to the input, as name.relinked.SchDoc)
package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rveen/golib/formats/altium/altium/relink"
)

func main() {
	lib := flag.String("lib", "mahle", "new library (\".DbLib\" is added when there is no extension)")
	table := flag.String("table", "components", "new database table (\"\" keeps the current table)")
	from := flag.String("from", "", "comma-separated libraries to relink (default: all database libraries)")
	dryRun := flag.Bool("n", false, "dry run: report, do not write")
	inPlace := flag.Bool("inplace", false, "overwrite the inputs, keeping a .bak copy of each")
	outDir := flag.String("out", "", "output directory (default: next to the input, as name.relinked.SchDoc)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: schrelink [options] file.SchDoc ... | project.PrjPcb\n\nOptions:\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() == 0 || (*inPlace && *outDir != "") {
		flag.Usage()
		os.Exit(1)
	}

	opt := relink.Options{Library: *lib, Table: *table}
	if filepath.Ext(opt.Library) == "" {
		opt.Library += ".DbLib"
	}
	for _, f := range strings.Split(*from, ",") {
		if f = strings.TrimSpace(f); f != "" {
			opt.From = append(opt.From, f)
		}
	}

	var files []string
	for _, arg := range flag.Args() {
		if strings.EqualFold(filepath.Ext(arg), ".PrjPcb") {
			docs, err := projectSchematics(arg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
			files = append(files, docs...)
		} else {
			files = append(files, arg)
		}
	}

	failed := false
	for _, f := range files {
		if err := run(f, opt, *dryRun, *inPlace, *outDir); err != nil {
			fmt.Fprintf(os.Stderr, "%s: error: %v\n", f, err)
			failed = true
		}
	}
	if failed {
		os.Exit(1)
	}
}

func run(path string, opt relink.Options, dryRun, inPlace bool, outDir string) error {
	in, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	out, rep, err := relink.Relink(in, opt)
	if err != nil {
		return err
	}

	fmt.Printf("%s:\n", path)
	print := func(label string, m map[relink.Link]int) {
		var links []relink.Link
		for l := range m {
			links = append(links, l)
		}
		sort.Slice(links, func(i, j int) bool { return links[i].String() < links[j].String() })
		for _, l := range links {
			fmt.Printf("  %-8s %5d  %s\n", label, m[l], l)
		}
	}
	print("relink", rep.Changed)
	print("keep", rep.Kept)
	if rep.Total() == 0 {
		fmt.Printf("  nothing to change\n")
		return nil
	}

	if err := relink.Verify(in, out, opt); err != nil {
		return fmt.Errorf("verification failed, nothing written: %w", err)
	}
	if dryRun {
		return nil
	}

	dst := strings.TrimSuffix(path, filepath.Ext(path)) + ".relinked" + filepath.Ext(path)
	switch {
	case inPlace:
		dst = path
		bak := path + ".bak"
		if _, err := os.Stat(bak); err == nil {
			return fmt.Errorf("backup %s already exists", bak)
		}
		if err := os.WriteFile(bak, in, 0o644); err != nil {
			return err
		}
	case outDir != "":
		dst = filepath.Join(outDir, filepath.Base(path))
		if same(dst, path) {
			return fmt.Errorf("output %s would overwrite the input (use -inplace)", dst)
		}
	}
	if err := writeFile(dst, out); err != nil {
		return err
	}
	fmt.Printf("  -> %s (%d components relinked to %s)\n", dst, rep.Total(), relink.Link{Library: opt.Library, Table: opt.Table})
	return nil
}

// writeFile writes via a temporary file in the same directory, so that a
// failure never leaves a half-written document behind.
func writeFile(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".schrelink-*")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	os.Chmod(tmp.Name(), 0o644)
	return os.Rename(tmp.Name(), path)
}

func same(a, b string) bool {
	fa, err1 := os.Stat(a)
	fb, err2 := os.Stat(b)
	return err1 == nil && err2 == nil && os.SameFile(fa, fb)
}

// projectSchematics returns the .SchDoc documents listed in a .PrjPcb
// (DocumentPath= lines, relative to the project with '\' separators).
func projectSchematics(prj string) ([]string, error) {
	data, err := os.ReadFile(prj)
	if err != nil {
		return nil, err
	}
	var docs []string
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		k, v, ok := strings.Cut(strings.TrimSpace(sc.Text()), "=")
		if !ok || !strings.EqualFold(k, "DocumentPath") || !strings.EqualFold(filepath.Ext(v), ".SchDoc") {
			continue
		}
		v = filepath.FromSlash(strings.ReplaceAll(v, `\`, "/"))
		if !filepath.IsAbs(v) {
			v = filepath.Join(filepath.Dir(prj), v)
		}
		docs = append(docs, v)
	}
	if len(docs) == 0 {
		return nil, fmt.Errorf("%s lists no .SchDoc documents", prj)
	}
	return docs, sc.Err()
}
