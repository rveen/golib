// Command pcbconv reads Altium .PcbDoc files and converts them to KiCad .kicad_pcb.
//
// Usage:
//
//	pcbconv [options] file.PcbDoc
//
// Options:
//
//	-kicad   convert to .kicad_pcb (default when no mode flag is given)
//	-i       print storage record counts
//	-out dir output directory (default: directory of input file)
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rveen/golib/formats/altium/altium/pcbmapper"
	"github.com/rveen/golib/formats/altium/altium/pcbreader"
	"github.com/rveen/golib/formats/altium/emit"
	"github.com/rveen/golib/formats/altium/emit/kicadpcb"
)

func main() {
	doKicad := flag.Bool("kicad", false, "convert to .kicad_pcb")
	doInfo := flag.Bool("i", false, "print storage record counts")
	outDir := flag.String("out", "", "output directory (default: directory of input file)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: pcbconv [options] file.PcbDoc\n\nOptions:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(1)
	}
	path := flag.Arg(0)

	if !*doKicad && !*doInfo {
		*doKicad = true
	}

	if *outDir == "" {
		*outDir = filepath.Dir(path)
	}

	var err error
	switch {
	case *doInfo:
		err = cmdInfo(path)
	default:
		err = cmdConvert(path, *outDir)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// ---------- info ----------

func cmdInfo(path string) error {
	rb, err := pcbreader.ReadFile(path)
	if err != nil {
		return err
	}
	fmt.Printf("Board6 records:       %d\n", len(rb.BoardProps))
	fmt.Printf("Components6 records:  %d\n", len(rb.ComponentRecs))
	fmt.Printf("Nets6 records:        %d\n", len(rb.NetRecs))
	fmt.Printf("Polygons6 records:    %d\n", len(rb.PolygonRecs))
	fmt.Printf("Arcs:                 %d\n", len(rb.Arcs))
	fmt.Printf("Pads:                 %d\n", len(rb.Pads))
	fmt.Printf("Vias:                 %d\n", len(rb.Vias))
	fmt.Printf("Tracks:               %d\n", len(rb.Tracks))
	fmt.Printf("Texts:                %d\n", len(rb.Texts))
	fmt.Printf("Fills:                %d\n", len(rb.Fills))
	fmt.Printf("Regions:              %d\n", len(rb.Regions))
	return nil
}

// ---------- convert ----------

func cmdConvert(path, outDir string) error {
	rb, err := pcbreader.ReadFile(path)
	if err != nil {
		return err
	}

	board, rep, err := pcbmapper.Map(rb, path)
	if err != nil {
		return err
	}
	printReport(rep, "mapper")

	artifacts, rep2, err := kicadpcb.Emitter{}.Emit(board, nil)
	if err != nil {
		return err
	}
	printReport(rep2, "kicadpcb")

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
