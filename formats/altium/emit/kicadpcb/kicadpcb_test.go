package kicadpcb_test

import (
	"strings"
	"testing"

	"github.com/rveen/golib/formats/altium/altium/pcbmapper"
	"github.com/rveen/golib/formats/altium/altium/pcbreader"
	"github.com/rveen/golib/formats/altium/emit/kicadpcb"
)

func TestEmit(t *testing.T) {
	rb, err := pcbreader.ReadFile("../../testdata/test.PcbDoc")
	if err != nil {
		t.Fatal(err)
	}
	board, _, err := pcbmapper.Map(rb, "test.PcbDoc")
	if err != nil {
		t.Fatal(err)
	}
	artifacts, rep, err := kicadpcb.Emitter{}.Emit(board, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(artifacts) == 0 {
		t.Fatal("no artifacts produced")
	}
	s := string(artifacts[0].Data)
	if !strings.HasPrefix(s, "(kicad_pcb") {
		t.Errorf("output does not start with (kicad_pcb, got: %.60q", s)
	}
	if !strings.Contains(s, "(net ") {
		t.Error("output missing net declarations")
	}
	if !strings.Contains(s, "(segment ") {
		t.Error("output missing segment entries")
	}
	for _, n := range rep.Notes {
		t.Logf("emitter note: %s", n.Message)
	}
	t.Logf("output size: %d bytes", len(s))
}
