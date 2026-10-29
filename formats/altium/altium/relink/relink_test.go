package relink

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/rveen/golib/formats/altium/altium/cfb"
	"github.com/rveen/golib/formats/altium/altium/reader"
)

func record(s string) []byte {
	b := binary.LittleEndian.AppendUint32(nil, uint32(len(s)+1))
	return append(append(b, s...), 0)
}

func TestRelinkStream(t *testing.T) {
	stream := bytes.Join([][]byte{
		record("|HEADER=Protel for Windows - Schematic Capture Binary File Version 5.0"),
		record("|RECORD=1|LIBREFERENCE=CAP NP|SOURCELIBRARYNAME=ESMP-SQLSVR01.DbLib|DATABASETABLENAME=xQuery|DESIGNITEMID=070201749"),
		record("|Record=1|LibReference=R|SourceLibraryName=Old.DbLib"),
		record("|RECORD=1|LIBREFERENCE=X|SOURCELIBRARYNAME=Misc.SchLib"),
		record("|RECORD=41|NAME=Code|TEXT=Old.DbLib"),
		{0x01, 0x00, 0x00, 0x01, 0xAA}, // binary payload, kept as is
	}, nil)
	orig := bytes.Clone(stream)

	rep := Report{Changed: map[Link]int{}, Kept: map[Link]int{}}
	out, err := relinkStream(stream, Options{Library: "mahle.DbLib", Table: "components"}, &rep)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(stream, orig) {
		t.Fatal("input buffer was modified")
	}
	want := bytes.Join([][]byte{
		record("|HEADER=Protel for Windows - Schematic Capture Binary File Version 5.0"),
		record("|RECORD=1|LIBREFERENCE=CAP NP|SOURCELIBRARYNAME=mahle.DbLib|DATABASETABLENAME=components|DESIGNITEMID=070201749"),
		record("|Record=1|LibReference=R|SourceLibraryName=mahle.DbLib|DATABASETABLENAME=components"),
		record("|RECORD=1|LIBREFERENCE=X|SOURCELIBRARYNAME=Misc.SchLib"),
		record("|RECORD=41|NAME=Code|TEXT=Old.DbLib"),
		{0x01, 0x00, 0x00, 0x01, 0xAA},
	}, nil)
	if !bytes.Equal(out, want) {
		t.Errorf("got\n%q\nwant\n%q", out, want)
	}
	if rep.Total() != 2 || rep.Kept[Link{"Misc.SchLib", ""}] != 1 {
		t.Errorf("report %+v", rep)
	}

	// Restricting to one source library.
	rep = Report{Changed: map[Link]int{}, Kept: map[Link]int{}}
	relinkStream(stream, Options{Library: "mahle.DbLib", From: []string{"old.dblib"}}, &rep)
	if rep.Total() != 1 || rep.Changed[Link{"Old.DbLib", ""}] != 1 {
		t.Errorf("From: report %+v", rep)
	}
}

func TestRelinkASCII(t *testing.T) {
	in := []byte("|HEADER=Protel for Windows - Schematic Capture Ascii File Version 5.0\r\n" +
		"|RECORD=1|SOURCELIBRARYNAME=A.DbLib|DATABASETABLENAME=T|LIBREFERENCE=R\r\n")
	out, rep, err := Relink(in, Options{Library: "mahle.DbLib", Table: "components"})
	if err != nil {
		t.Fatal(err)
	}
	want := "|HEADER=Protel for Windows - Schematic Capture Ascii File Version 5.0\r\n" +
		"|RECORD=1|SOURCELIBRARYNAME=mahle.DbLib|DATABASETABLENAME=components|LIBREFERENCE=R\r\n"
	if string(out) != want || rep.Total() != 1 {
		t.Errorf("got %q (%d changed)", out, rep.Total())
	}
	if err := Verify(in, out, Options{Library: "mahle.DbLib", Table: "components"}); err != nil {
		t.Error(err)
	}
}

func TestRelinkBadName(t *testing.T) {
	if _, _, err := Relink(nil, Options{Library: "a|b"}); err == nil {
		t.Error("accepted '|' in library name")
	}
}

// TestRelinkSample relinks the DbLib sample design, when present.
func TestRelinkSample(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "dblib", "02_SCH", "sg-1154-5.1.SchDoc")
	in, err := os.ReadFile(path)
	if err != nil {
		t.Skip("sample not present")
	}
	opt := Options{Library: "mahle.DbLib", Table: "components"}
	out, rep, err := Relink(in, opt)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Total() != 266 {
		t.Errorf("relinked %d components, want 266 (%v)", rep.Total(), rep.Changed)
	}
	if err := Verify(in, out, opt); err != nil {
		t.Fatal(err)
	}

	recs, _, err := reader.ReadBytes(out)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range recs {
		if r.Type == 1 && (r.Props[keyLibrary] != "mahle.DbLib" || r.Props[keyTable] != "components") {
			t.Fatalf("component not relinked: %v", r.Props)
		}
	}

	// Running again changes nothing.
	again, rep, err := Relink(out, opt)
	if err != nil || rep.Total() != 0 || !bytes.Equal(again, out) {
		t.Errorf("second run: %d changed, err %v", rep.Total(), err)
	}
	if root, _ := cfb.Read(out); root == nil || root.Child("Storage") == nil {
		t.Error("Storage stream lost")
	}
}
