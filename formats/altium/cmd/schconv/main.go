// Command schconv reads Altium schematic files and converts or inspects them.
//
// Usage:
//
//	schconv [options] file.SchDoc
//
// Options:
//
//	-kicad   convert to KiCad .kicad_sch (default when no mode flag is given)
//	-svg     convert to SVG
//	-i       print record-type counts
//	-json    dump all records as JSON
//	-out dir output directory (default: same directory as the input file)
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/rveen/golib/formats/altium/altium/mapper"
	"github.com/rveen/golib/formats/altium/altium/reader"
	"github.com/rveen/golib/formats/altium/altium/record"
	"github.com/rveen/golib/formats/altium/emit"
	kicademit "github.com/rveen/golib/formats/altium/emit/kicad"
	svgemit "github.com/rveen/golib/formats/altium/emit/svg"
	symcatemit "github.com/rveen/golib/formats/altium/emit/symcat"
)

func main() {
	doKicad := flag.Bool("kicad", false, "convert to KiCad .kicad_sch")
	doSVG := flag.Bool("svg", false, "convert to SVG")
	doSym := flag.Bool("sym", false, "render symbol catalog SVG")
	doInfo := flag.Bool("i", false, "print record-type counts")
	doJSON := flag.Bool("json", false, "dump all records as JSON")
	outDir := flag.String("out", "", "output directory (default: directory of input file)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: schconv [options] file.SchDoc\n\nOptions:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(1)
	}
	path := flag.Arg(0)

	// Default to -kicad when no mode flag is given.
	if !*doKicad && !*doSVG && !*doSym && !*doInfo && !*doJSON {
		*doKicad = true
	}

	if *outDir == "" {
		*outDir = filepath.Dir(path)
	}

	var err error
	switch {
	case *doInfo:
		err = cmdInfo(path)
	case *doJSON:
		err = cmdJSON(path)
	case *doSVG:
		err = cmdConvert(path, svgemit.Emitter{}, *outDir)
	case *doSym:
		err = cmdConvert(path, symcatemit.Emitter{}, *outDir)
	default: // -kicad
		err = cmdConvert(path, kicademit.Emitter{}, *outDir)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// ---------- info ----------

func cmdInfo(path string) error {
	records, _, err := reader.ReadFile(path)
	if err != nil {
		return err
	}

	counts := make(map[int]int)
	for _, r := range records {
		counts[r.Type]++
	}

	ids := make([]int, 0, len(counts))
	for id := range counts {
		ids = append(ids, id)
	}
	sort.Ints(ids)

	fmt.Printf("Total records: %d\n\n", len(records))
	fmt.Printf("%-6s  %-25s  %s\n", "ID", "Name", "Count")
	fmt.Printf("%-6s  %-25s  %s\n", "------", "-------------------------", "-----")
	for _, id := range ids {
		name, known := record.TypeName[id]
		if !known {
			name = "UNKNOWN"
		}
		note := ""
		if !known {
			note = "  <-- undocumented"
		}
		fmt.Printf("%-6d  %-25s  %d%s\n", id, name, counts[id], note)
	}
	return nil
}

// ---------- json ----------

func cmdJSON(path string) error {
	records, _, err := reader.ReadFile(path)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	type jsonRecord struct {
		StreamPos int               `json:"stream_pos"`
		Type      int               `json:"type"`
		Name      string            `json:"name,omitempty"`
		Index     int               `json:"index"`
		Props     map[string]string `json:"props"`
	}
	for i, r := range records {
		if err := enc.Encode(jsonRecord{
			StreamPos: i,
			Type:      r.Type,
			Name:      record.TypeName[r.Type],
			Index:     r.Index,
			Props:     r.Props,
		}); err != nil {
			return err
		}
	}
	return nil
}

// ---------- convert ----------

func cmdConvert(path string, emitter emit.Emitter, outDir string) error {
	records, isBinary, err := reader.ReadFile(path)
	if err != nil {
		return err
	}

	base := filepath.Base(path)
	ext := filepath.Ext(base)
	name := base[:len(base)-len(ext)]

	coordScale := 1
	if isBinary {
		coordScale = 10
	}
	sch, rep, err := mapper.Map(records, name, path, coordScale)
	if err != nil {
		return err
	}
	printReport(rep, "mapper")

	artifacts, rep2, err := emitter.Emit(sch, nil)
	if err != nil {
		return err
	}
	printReport(rep2, emitter.Name())

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	for _, a := range artifacts {
		outPath := filepath.Join(outDir, a.Name)
		if err := os.WriteFile(outPath, a.Data, 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", outPath, err)
		}
		fmt.Printf("wrote %s (%d bytes)\n", outPath, len(a.Data))
	}
	return nil
}

func printReport(rep *emit.Report, stage string) {
	for _, n := range rep.Notes {
		sev := "INFO"
		switch n.Severity {
		case emit.Warn:
			sev = "WARN"
		case emit.Error:
			sev = "ERROR"
		}
		fmt.Fprintf(os.Stderr, "[%s] %s: %s\n", stage, sev, n.Message)
	}
}
