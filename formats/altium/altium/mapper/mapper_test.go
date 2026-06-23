package mapper

import (
	"testing"

	"github.com/rveen/golib/formats/altium/altium/reader"
)

func TestMapFromTestSchDoc(t *testing.T) {
	recs, isBinary, err := reader.ReadFile("../../testdata/test.SchDoc")
	if err != nil {
		t.Fatal(err)
	}
	coordScale := 1
	if isBinary {
		coordScale = 10
	}

	sch, rep, err := Map(recs, "test", "test.SchDoc", coordScale)
	if err != nil {
		t.Fatal(err)
	}

	if len(sch.Sheets) != 1 {
		t.Fatalf("expected 1 sheet, got %d", len(sch.Sheets))
	}
	sh := sch.Sheets[0]

	if len(sh.Components) == 0 {
		t.Error("no components mapped")
	}
	if len(sch.Symbols) == 0 {
		t.Error("no symbols deduplicated")
	}

	// Report errors must be zero; warnings are allowed.
	for _, n := range rep.Notes {
		if n.Severity == 2 { // Error
			t.Errorf("mapper error: %s", n.Message)
		}
	}

	t.Logf("components=%d symbols=%d wires=%d netLabels=%d powerPorts=%d warnings=%d",
		len(sh.Components), len(sch.Symbols),
		len(sh.Wires), len(sh.NetLabels), len(sh.PowerPorts),
		len(rep.Notes))
}

func TestSymbolDedup(t *testing.T) {
	recs, isBinary, err := reader.ReadFile("../../testdata/test.SchDoc")
	if err != nil {
		t.Fatal(err)
	}
	coordScale := 1
	if isBinary {
		coordScale = 10
	}
	sch, _, err := Map(recs, "test", "test.SchDoc", coordScale)
	if err != nil {
		t.Fatal(err)
	}

	// There must be fewer symbols than components if any component type is reused.
	if len(sch.Symbols) >= len(sch.Sheets[0].Components) {
		t.Logf("symbols=%d components=%d — no deduplication occurred (may be OK for unique designs)",
			len(sch.Symbols), len(sch.Sheets[0].Components))
	}
	// Every component must reference a symbol that exists in the map.
	for _, comp := range sch.Sheets[0].Components {
		if _, ok := sch.Symbols[comp.Symbol]; !ok {
			t.Errorf("component %q references unknown symbol %s", comp.Designator, comp.Symbol)
		}
	}
}
