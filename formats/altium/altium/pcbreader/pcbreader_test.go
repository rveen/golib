package pcbreader_test

import (
	"testing"

	"github.com/rveen/golib/formats/altium/altium/pcbreader"
)

func TestReadFile(t *testing.T) {
	rb, err := pcbreader.ReadFile("../../testdata/test.PcbDoc")
	if err != nil {
		t.Fatal(err)
	}
	if len(rb.NetRecs) == 0 {
		t.Error("expected net records, got 0")
	}
	if len(rb.ComponentRecs) == 0 {
		t.Error("expected component records, got 0")
	}
	if len(rb.Tracks) == 0 {
		t.Error("expected tracks, got 0")
	}
	t.Logf("board=%d comps=%d nets=%d polys=%d arcs=%d pads=%d vias=%d tracks=%d texts=%d fills=%d regions=%d",
		len(rb.BoardProps), len(rb.ComponentRecs), len(rb.NetRecs), len(rb.PolygonRecs),
		len(rb.Arcs), len(rb.Pads), len(rb.Vias), len(rb.Tracks),
		len(rb.Texts), len(rb.Fills), len(rb.Regions))
}
