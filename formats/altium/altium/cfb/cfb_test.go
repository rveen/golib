package cfb

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/richardlehane/mscfb"
)

// streams reads every stream of a compound file with mscfb, an independent
// implementation, keyed by its slash-joined path.
func streams(t *testing.T, b []byte) map[string][]byte {
	t.Helper()
	doc, err := mscfb.New(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("mscfb: %v", err)
	}
	out := make(map[string][]byte)
	for e, err := doc.Next(); err == nil; e, err = doc.Next() {
		if e.Size == 0 && len(e.Path) == 0 && e.Name == "" {
			continue
		}
		buf, err := io.ReadAll(doc)
		if err != nil {
			t.Fatalf("mscfb read %s: %v", e.Name, err)
		}
		out[strings.Join(append(e.Path, e.Name), "/")] = buf
	}
	return out
}

func TestRoundTripSynthetic(t *testing.T) {
	big := bytes.Repeat([]byte("0123456789abcdef"), 1000) // 16000 bytes, regular sectors
	root := &Entry{Name: "Root Entry", Type: TypeRoot, CLSID: [16]byte{1, 2, 3}}
	sub := &Entry{Name: "Board6", Type: TypeStorage, Created: 132657341163950000}
	sub.Children = []*Entry{
		{Name: "Header", Type: TypeStream, Data: []byte{1, 0, 0, 0}},
		{Name: "Data", Type: TypeStream, Data: big},
	}
	root.Children = []*Entry{
		{Name: "FileHeader", Type: TypeStream, Data: []byte("|HEADER=x|")},
		{Name: "Empty", Type: TypeStream, Data: []byte{}},
		sub,
	}
	for i := 0; i < 20; i++ {
		root.Children = append(root.Children, &Entry{Name: "S" + strings.Repeat("x", i), Type: TypeStream, Data: bytes.Repeat([]byte{byte(i)}, 100*i)})
	}

	b, err := root.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	got := streams(t, b)
	if !bytes.Equal(got["Board6/Data"], big) {
		t.Errorf("Board6/Data mismatch (len %d)", len(got["Board6/Data"]))
	}
	if string(got["FileHeader"]) != "|HEADER=x|" {
		t.Errorf("FileHeader = %q", got["FileHeader"])
	}

	back, err := Read(b)
	if err != nil {
		t.Fatal(err)
	}
	if back.CLSID != root.CLSID {
		t.Errorf("root CLSID not preserved")
	}
	if c := back.Child("board6"); c == nil || c.Created != sub.Created || len(c.Children) != 2 {
		t.Errorf("storage not preserved: %+v", c)
	}
}

func TestRoundTripLarge(t *testing.T) {
	// Big enough to need DIFAT sectors (more than 109 FAT sectors, ~7 MB).
	data := make([]byte, 8<<20)
	for i := range data {
		data[i] = byte(i * 7)
	}
	root := &Entry{Name: "Root Entry", Type: TypeRoot, Children: []*Entry{{Name: "Big", Type: TypeStream, Data: data}}}
	b, err := root.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(streams(t, b)["Big"], data) {
		t.Fatal("mscfb read back different data")
	}
	back, err := Read(b)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(back.Child("Big").Data, data) {
		t.Fatal("Read gave different data")
	}
}

// TestRoundTripFiles re-writes the Altium sample documents, when present, and
// checks that mscfb sees identical streams.
func TestRoundTripFiles(t *testing.T) {
	var files []string
	for _, pat := range []string{"*.SchDoc", "*.PcbDoc", "*/*/*.SchDoc", "*/*/*.PcbDoc"} {
		m, _ := filepath.Glob(filepath.Join("..", "..", "testdata", pat))
		files = append(files, m...)
	}
	if len(files) == 0 {
		t.Skip("no sample documents in testdata")
	}
	for _, f := range files {
		in, err := os.ReadFile(f)
		if err != nil || !IsCFB(in) {
			continue
		}
		root, err := Read(in)
		if err != nil {
			t.Errorf("%s: %v", f, err)
			continue
		}
		out, err := root.Bytes()
		if err != nil {
			t.Errorf("%s: %v", f, err)
			continue
		}
		want, got := streams(t, in), streams(t, out)
		if len(want) != len(got) {
			t.Errorf("%s: %d streams, want %d", f, len(got), len(want))
		}
		for k, v := range want {
			if !bytes.Equal(got[k], v) {
				t.Errorf("%s: stream %s differs", f, k)
			}
		}
	}
}
