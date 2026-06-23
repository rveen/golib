package reader

import (
	"strings"
	"testing"

	"github.com/rveen/golib/formats/altium/altium/record"
)

func TestReadASCII(t *testing.T) {
	const sample = `|HEADER=Test|Weight=1
|RECORD=1|LIBREFERENCE=R1|INDEXINSHEET=0
|RECORD=27|LOCATION.X=100|LOCATION.Y=200|INDEXINSHEET=1
`
	recs, err := ReadASCII(strings.NewReader(sample))
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 3 {
		t.Fatalf("expected 3 records, got %d", len(recs))
	}
	if recs[0].Str("HEADER") != "Test" {
		t.Errorf("record 0 HEADER: got %q", recs[0].Str("HEADER"))
	}
	if recs[1].Type != record.TypeComponent {
		t.Errorf("record 1 type: got %d want %d", recs[1].Type, record.TypeComponent)
	}
	if recs[1].Index != 0 {
		t.Errorf("record 1 index: got %d want 0", recs[1].Index)
	}
	if recs[2].Type != record.TypeWire {
		t.Errorf("record 2 type: got %d want %d", recs[2].Type, record.TypeWire)
	}
}

func TestReadFileCFB(t *testing.T) {
	recs, isBinary, err := ReadFile("../../testdata/test.SchDoc")
	if err != nil {
		t.Fatal(err)
	}
	if !isBinary {
		t.Error("expected isBinary=true for CFB file")
	}
	if len(recs) == 0 {
		t.Fatal("no records decoded")
	}
	// First record must be the HEADER.
	if recs[0].Type != record.TypeHeader {
		t.Errorf("first record type: got %d want %d (HEADER)", recs[0].Type, record.TypeHeader)
	}
	// Must have at least one COMPONENT.
	var comps int
	for _, r := range recs {
		if r.Type == record.TypeComponent {
			comps++
		}
	}
	if comps == 0 {
		t.Error("no COMPONENT records found")
	}
	t.Logf("total records: %d, components: %d", len(recs), comps)
}
