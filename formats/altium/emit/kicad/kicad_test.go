package kicad_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rveen/golib/formats/altium/altium/mapper"
	"github.com/rveen/golib/formats/altium/altium/reader"
	kicademit "github.com/rveen/golib/formats/altium/emit/kicad"
)

func TestEmitFromTestSchDoc(t *testing.T) {
	recs, isBinary, err := reader.ReadFile("../../testdata/test.SchDoc")
	if err != nil {
		t.Fatal(err)
	}
	coordScale := 1
	if isBinary {
		coordScale = 10
	}
	sch, _, err := mapper.Map(recs, "test", "test.SchDoc", coordScale)
	if err != nil {
		t.Fatal(err)
	}

	artifacts, rep, err := kicademit.Emitter{}.Emit(sch, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range rep.Notes {
		t.Logf("emit note: %s", n.Message)
	}

	if len(artifacts) == 0 {
		t.Fatal("no artifacts produced")
	}

	data := artifacts[0].Data
	t.Logf("artifact: %s, size: %d bytes", artifacts[0].Name, len(data))

	s := string(data)
	if !strings.HasPrefix(s, "(kicad_sch") {
		t.Errorf("output does not start with (kicad_sch, got: %.40q", s)
	}
	if !strings.Contains(s, "(lib_symbols") {
		t.Error("output missing lib_symbols block")
	}
	if !strings.Contains(s, "(symbol ") {
		t.Error("output missing symbol instances")
	}
	if !strings.Contains(s, "(wire") {
		t.Error("output missing wires")
	}
	// test.SchDoc carries Altium ports (→ hierarchical_label) rather than plain
	// net labels, so check for the labels this fixture actually produces.
	if !strings.Contains(s, "(hierarchical_label ") {
		t.Error("output missing hierarchical labels")
	}

	// Write golden file if UPDATE_GOLDEN=1 is set.
	goldenPath := filepath.Join("../../testdata", "golden_test.kicad_sch")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(goldenPath, data, 0o644); err != nil {
			t.Fatalf("writing golden: %v", err)
		}
		t.Logf("wrote golden %s", goldenPath)
		return
	}

	// Compare against golden if it exists.
	golden, err := os.ReadFile(goldenPath)
	if err != nil {
		if os.IsNotExist(err) {
			t.Logf("no golden file at %s — run with UPDATE_GOLDEN=1 to create", goldenPath)
			return
		}
		t.Fatal(err)
	}
	if string(golden) != s {
		t.Errorf("output differs from golden file %s", goldenPath)
	}
}

func TestMakeUUIDDeterministic(t *testing.T) {
	// Two artifacts from the same input must be byte-identical (deterministic UUIDs).
	recs, isBinary, err := reader.ReadFile("../../testdata/test.SchDoc")
	if err != nil {
		t.Fatal(err)
	}
	coordScale := 1
	if isBinary {
		coordScale = 10
	}
	sch, _, err := mapper.Map(recs, "test", "test.SchDoc", coordScale)
	if err != nil {
		t.Fatal(err)
	}

	a1, _, _ := kicademit.Emitter{}.Emit(sch, nil)
	a2, _, _ := kicademit.Emitter{}.Emit(sch, nil)

	if string(a1[0].Data) != string(a2[0].Data) {
		t.Error("KiCad emitter is not deterministic")
	}
}
