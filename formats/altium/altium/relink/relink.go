// Package relink points the placed components of an Altium .SchDoc to another
// (database) library. Only the SOURCELIBRARYNAME and DATABASETABLENAME
// properties of component records (RECORD=1) change; every other byte of the
// record stream, and every other stream of the file, is kept as is.
package relink

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"reflect"
	"strings"

	"github.com/rveen/golib/formats/altium/altium/cfb"
	"github.com/rveen/golib/formats/altium/altium/reader"
)

// Keys of the component library link.
const (
	keyLibrary = "SOURCELIBRARYNAME"
	keyTable   = "DATABASETABLENAME"
	utf8Prefix = "%UTF8%" // Altium's companion key carrying the UTF-8 value
)

// Streams of a binary .SchDoc that hold property records.
var recordStreams = []string{"FileHeader", "Additional"}

// Link is a component's library and database table.
type Link struct {
	Library, Table string
}

func (l Link) String() string {
	if l.Table == "" {
		return l.Library
	}
	return l.Library + "/" + l.Table
}

// Options selects the components to relink and their new link.
type Options struct {
	Library string // new library, e.g. "mahle.DbLib"
	Table   string // new table; empty keeps each component's table

	// From lists the current libraries to relink (case-insensitive). When
	// empty, every component from a database library (.DbLib, .SVNDbLib) is
	// relinked.
	From []string
}

// Report counts components per original link.
type Report struct {
	Changed map[Link]int // relinked components, by their old link
	Kept    map[Link]int // components left alone
}

// Total returns the number of relinked components.
func (r Report) Total() int {
	n := 0
	for _, c := range r.Changed {
		n += c
	}
	return n
}

// Relink rewrites doc (a binary or ASCII .SchDoc). When no component
// changes, the returned slice is doc itself.
func Relink(doc []byte, opt Options) ([]byte, Report, error) {
	rep := Report{Changed: map[Link]int{}, Kept: map[Link]int{}}
	if err := checkValue(opt.Library); err != nil || opt.Library == "" {
		return nil, rep, fmt.Errorf("bad library name %q", opt.Library)
	}
	if err := checkValue(opt.Table); err != nil {
		return nil, rep, fmt.Errorf("bad table name %q", opt.Table)
	}

	if !cfb.IsCFB(doc) {
		out := relinkASCII(doc, opt, &rep)
		if rep.Total() == 0 {
			return doc, rep, nil
		}
		return out, rep, nil
	}

	root, err := cfb.Read(doc)
	if err != nil {
		return nil, rep, err
	}
	for _, name := range recordStreams {
		if s := root.Child(name); s != nil && s.Type == cfb.TypeStream {
			if s.Data, err = relinkStream(s.Data, opt, &rep); err != nil {
				return nil, rep, fmt.Errorf("%s: %w", name, err)
			}
		}
	}
	if rep.Total() == 0 {
		return doc, rep, nil
	}
	out, err := root.Bytes()
	return out, rep, err
}

// checkValue rejects values that cannot be stored in a property list.
// Non-ASCII is refused so that no %UTF8% companion keys are needed.
func checkValue(v string) error {
	for _, c := range v {
		if c < 0x20 || c > 0x7E || c == '|' || c == '=' {
			return fmt.Errorf("invalid character %q", c)
		}
	}
	return nil
}

// relinkStream rewrites a binary record stream: a sequence of uint32 headers
// (bits 0..23 payload length, bits 24..31 payload type, 0 = text) followed by
// the payload.
func relinkStream(buf []byte, opt Options, rep *Report) ([]byte, error) {
	out := make([]byte, 0, len(buf)+len(buf)/16)
	i := 0
	for i+4 <= len(buf) {
		hdr := binary.LittleEndian.Uint32(buf[i:])
		size := int(hdr & 0x00FFFFFF)
		if i+4+size > len(buf) {
			break
		}
		payload := buf[i+4 : i+4+size]
		if hdr>>24 == 0 {
			if p, ok := relinkRecord(payload, opt, rep); ok {
				if len(p) > 0x00FFFFFF {
					return nil, fmt.Errorf("record too long")
				}
				out = binary.LittleEndian.AppendUint32(out, hdr&0xFF000000|uint32(len(p)))
				out = append(out, p...)
				i += 4 + size
				continue
			}
		}
		out = append(out, buf[i:i+4+size]...)
		i += 4 + size
	}
	return append(out, buf[i:]...), nil // keep any trailing bytes
}

// relinkASCII rewrites an ASCII .SchDoc, one record per line.
func relinkASCII(doc []byte, opt Options, rep *Report) []byte {
	lines := bytes.SplitAfter(doc, []byte("\n"))
	for i, l := range lines {
		body := bytes.TrimRight(l, "\r\n")
		if p, ok := relinkRecord(body, opt, rep); ok {
			lines[i] = append(p, l[len(body):]...)
		}
	}
	return bytes.Join(lines, nil)
}

// relinkRecord rewrites one "|KEY=VALUE|..." payload (optionally followed by
// a NUL and padding) if it is a selected component. It reports whether the
// payload changed.
func relinkRecord(p []byte, opt Options, rep *Report) ([]byte, bool) {
	text, rest := p, []byte(nil)
	if i := bytes.IndexByte(p, 0); i >= 0 {
		text, rest = p[:i], p[i:]
	}
	fields := bytes.Split(text, []byte("|"))
	value := func(key string) (string, bool) {
		for _, f := range fields {
			if k, v, ok := bytes.Cut(f, []byte("=")); ok && strings.EqualFold(string(bytes.TrimSpace(k)), key) {
				return string(bytes.TrimSpace(v)), true
			}
		}
		return "", false
	}
	if r, _ := value("RECORD"); r != "1" {
		return nil, false
	}
	lib, _ := value(keyLibrary)
	table, hasTable := value(keyTable)
	old := Link{lib, table}

	want := Link{opt.Library, table}
	if opt.Table != "" {
		want.Table = opt.Table
	}
	if !selected(lib, opt.From) || old == want {
		rep.Kept[old]++
		return nil, false
	}

	set := map[string]string{keyLibrary: want.Library, utf8Prefix + keyLibrary: want.Library}
	if opt.Table != "" {
		set[keyTable] = want.Table
		set[utf8Prefix+keyTable] = want.Table
	}
	var out [][]byte
	for _, f := range fields {
		if k, _, ok := bytes.Cut(f, []byte("=")); ok {
			if v, ok := set[strings.ToUpper(string(bytes.TrimSpace(k)))]; ok {
				f = []byte(string(k) + "=" + v) // a copy: f aliases the input
			}
		}
		out = append(out, f)
		// A database component without a table gets one next to the library.
		if opt.Table != "" && !hasTable && bytes.HasPrefix(bytes.ToUpper(f), []byte(keyLibrary+"=")) {
			out = append(out, []byte(keyTable+"="+want.Table))
		}
	}
	rep.Changed[old]++
	return append(bytes.Join(out, []byte("|")), rest...), true
}

func selected(lib string, from []string) bool {
	if len(from) == 0 {
		l := strings.ToLower(lib)
		return strings.HasSuffix(l, ".dblib") || strings.HasSuffix(l, ".svndblib")
	}
	for _, f := range from {
		if strings.EqualFold(f, lib) {
			return true
		}
	}
	return false
}

// Verify checks that out differs from orig only in the library link of
// relinked components: both files are decoded (records through the mscfb
// based reader package, an implementation independent of the writer), all
// other streams must be byte-identical and all other properties equal.
func Verify(orig, out []byte, opt Options) error {
	a, _, err := reader.ReadBytes(orig)
	if err != nil {
		return fmt.Errorf("original: %w", err)
	}
	b, _, err := reader.ReadBytes(out)
	if err != nil {
		return fmt.Errorf("output: %w", err)
	}
	if len(a) != len(b) {
		return fmt.Errorf("record count changed: %d -> %d", len(a), len(b))
	}
	linkKeys := []string{keyLibrary, keyTable, utf8Prefix + keyLibrary, utf8Prefix + keyTable}
	for i := range a {
		pa, pb := a[i].Props, b[i].Props
		if a[i].Type == 1 && (pb[keyLibrary] != pa[keyLibrary] || pb[keyTable] != pa[keyTable]) {
			if pb[keyLibrary] != opt.Library || (opt.Table != "" && pb[keyTable] != opt.Table) {
				return fmt.Errorf("record %d: link is %s/%s", i, pb[keyLibrary], pb[keyTable])
			}
			pa, pb = without(pa, linkKeys), without(pb, linkKeys)
		}
		if !reflect.DeepEqual(pa, pb) {
			return fmt.Errorf("record %d (RECORD=%d) changed unexpectedly", i, a[i].Type)
		}
	}

	if !cfb.IsCFB(orig) {
		return nil
	}
	ra, err := cfb.Read(orig)
	if err != nil {
		return err
	}
	rb, err := cfb.Read(out)
	if err != nil {
		return err
	}
	collect := func(root *cfb.Entry) map[string]*cfb.Entry {
		m := map[string]*cfb.Entry{}
		root.Walk(func(path []string, e *cfb.Entry) { m[strings.Join(path, "/")] = e })
		return m
	}
	ma, mb := collect(ra), collect(rb)
	if len(ma) != len(mb) {
		return fmt.Errorf("entry count changed: %d -> %d", len(ma), len(mb))
	}
	for path, ea := range ma {
		eb, ok := mb[path]
		if !ok {
			return fmt.Errorf("entry %q missing", path)
		}
		x, y := ea, eb
		if x.Name != y.Name || x.Type != y.Type || x.CLSID != y.CLSID || x.StateBits != y.StateBits ||
			x.Created != y.Created || x.Modified != y.Modified {
			return fmt.Errorf("entry %q metadata changed", path)
		}
		edited := path == "FileHeader" || path == "Additional"
		if !edited && !bytes.Equal(ea.Data, eb.Data) {
			return fmt.Errorf("stream %q changed", path)
		}
	}
	return nil
}

func without(m map[string]string, keys []string) map[string]string {
	c := make(map[string]string, len(m))
	for k, v := range m {
		c[k] = v
	}
	for _, k := range keys {
		delete(c, k)
	}
	return c
}
