package pcbmapper_test

import (
	"testing"

	"github.com/rveen/golib/formats/altium/altium/pcbmapper"
	"github.com/rveen/golib/formats/altium/altium/pcbreader"
	"github.com/rveen/golib/formats/altium/emit"
)

func TestMap(t *testing.T) {
	rb, err := pcbreader.ReadFile("../../testdata/test.PcbDoc")
	if err != nil {
		t.Fatal(err)
	}
	board, rep, err := pcbmapper.Map(rb, "test.PcbDoc")
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range rep.Notes {
		if n.Severity == emit.Error {
			t.Errorf("mapper error: %s", n.Message)
		}
	}
	if len(board.Nets) == 0 {
		t.Error("expected nets, got 0")
	}
	if len(board.Tracks) == 0 {
		t.Error("expected tracks, got 0")
	}
	if len(board.Pads) == 0 {
		t.Error("expected pads, got 0")
	}
	t.Logf("nets=%d comps=%d tracks=%d vias=%d pads=%d arcs=%d outline=%d",
		len(board.Nets), len(board.Components), len(board.Tracks),
		len(board.Vias), len(board.Pads), len(board.Arcs), len(board.BoardOutline))
}
