// Package altium converts Altium Designer files to KiCad format.
//
// ConvertToKicadSch converts a .SchDoc file (read into a byte slice) to the
// KiCad .kicad_sch format. ConvertToKicadPcb does the same for .PcbDoc files.
package altium

import (
	"fmt"
	"log"

	"github.com/rveen/golib/formats/altium/altium/mapper"
	"github.com/rveen/golib/formats/altium/altium/pcbmapper"
	"github.com/rveen/golib/formats/altium/altium/pcbreader"
	"github.com/rveen/golib/formats/altium/altium/reader"
	kicad "github.com/rveen/golib/formats/altium/emit/kicad"
	"github.com/rveen/golib/formats/altium/emit/kicadpcb"
)

// ConvertToKicadSch converts an Altium .SchDoc file (as a byte slice) to
// KiCad .kicad_sch format, returning the output as a byte slice.
func ConvertToKicadSch(in []byte) ([]byte, error) {

	log.Printf("to be converted to kicad sch; size %d\n", len(in))

	records, isBinary, err := reader.ReadBytes(in)
	if err != nil {
		return nil, fmt.Errorf("reading schematic: %w", err)
	}

	coordScale := 1
	if isBinary {
		coordScale = 10
	}

	sch, _, err := mapper.Map(records, "", "", coordScale)
	if err != nil {
		return nil, fmt.Errorf("mapping schematic: %w", err)
	}

	artifacts, _, err := kicad.Emitter{}.Emit(sch, nil)
	if err != nil {
		return nil, fmt.Errorf("emitting kicad_sch: %w", err)
	}
	if len(artifacts) == 0 {
		return nil, fmt.Errorf("no output produced")
	}

	log.Printf("converted to kicad sch; size %d\n", len(artifacts[0].Data))
	return artifacts[0].Data, nil
}

// ConvertToKicadPcb converts an Altium .PcbDoc file (as a byte slice) to
// KiCad .kicad_pcb format, returning the output as a byte slice.
func ConvertToKicadPcb(in []byte) ([]byte, error) {
	rb, err := pcbreader.ReadBytes(in)
	if err != nil {
		return nil, fmt.Errorf("reading PCB: %w", err)
	}

	board, _, err := pcbmapper.Map(rb, "")
	if err != nil {
		return nil, fmt.Errorf("mapping PCB: %w", err)
	}

	artifacts, _, err := kicadpcb.Emitter{}.Emit(board, nil)
	if err != nil {
		return nil, fmt.Errorf("emitting kicad_pcb: %w", err)
	}
	if len(artifacts) == 0 {
		return nil, fmt.Errorf("no output produced")
	}
	log.Printf("converted to kicad sch; size %d\n", len(artifacts[0].Data))
	return artifacts[0].Data, nil
}
