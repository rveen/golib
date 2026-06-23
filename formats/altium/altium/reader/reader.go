// Package reader decodes Altium .SchDoc files into a flat slice of records.
// Both binary CFB and ASCII variants are supported; the format is auto-detected.
package reader

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/richardlehane/mscfb"

	"github.com/rveen/golib/formats/altium/altium/record"
)

// cfbMagic is the first 8 bytes of every CFB (OLE Structured Storage) file.
var cfbMagic = []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}

// ReadBytes auto-detects CFB vs ASCII from an in-memory byte slice and returns
// all records. The second return value is true for binary CFB files.
func ReadBytes(data []byte) ([]record.Record, bool, error) {
	if len(data) >= 8 && bytes.Equal(data[:8], cfbMagic) {
		recs, err := readCFB(bytes.NewReader(data))
		return recs, true, err
	}
	recs, err := ReadASCII(bytes.NewReader(data))
	return recs, false, err
}

// ReadFile opens path, auto-detects CFB vs ASCII, and returns all records.
// The second return value is true when the file is a binary CFB container
// (coordinates stored in decamils; multiply by 10 to get mils).
func ReadFile(path string) ([]record.Record, bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, false, err
	}
	defer f.Close()

	hdr := make([]byte, 8)
	if _, err := io.ReadFull(f, hdr); err != nil {
		return nil, false, fmt.Errorf("reading header: %w", err)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, false, err
	}

	if bytes.Equal(hdr, cfbMagic) {
		recs, err := readCFB(f)
		return recs, true, err
	}
	recs, err := ReadASCII(f)
	return recs, false, err
}

// ReadASCII parses the ASCII .SchDoc variant: one record per non-empty line,
// pipe-delimited KEY=VALUE pairs.  The file should start with |HEADER=...
func ReadASCII(r io.Reader) ([]record.Record, error) {
	var records []record.Record
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		rec, err := parsePropString(line)
		if err != nil {
			continue // best-effort
		}
		records = append(records, rec)
	}
	return records, sc.Err()
}

// readCFB opens the CFB container and extracts records from FileHeader and
// the optional Additional stream (which carries harness records and extends
// the same index sequence).
func readCFB(rs io.ReaderAt) ([]record.Record, error) {
	doc, err := mscfb.New(rs)
	if err != nil {
		return nil, fmt.Errorf("CFB open: %w", err)
	}

	streamBufs := make(map[string][]byte)
	for entry, err := doc.Next(); err == nil; entry, err = doc.Next() {
		name := entry.Name
		if name != "FileHeader" && name != "Additional" {
			continue
		}
		buf := make([]byte, entry.Size)
		if _, err := io.ReadFull(doc, buf); err != nil {
			return nil, fmt.Errorf("reading %s: %w", name, err)
		}
		streamBufs[name] = buf
	}

	var records []record.Record
	for _, name := range []string{"FileHeader", "Additional"} {
		buf, ok := streamBufs[name]
		if !ok {
			continue
		}
		recs, err := parseRecordStream(buf)
		if err != nil {
			return nil, err
		}
		records = append(records, recs...)
	}
	return records, nil
}

// parseRecordStream decodes a binary record stream (FileHeader or Additional).
//
// Wire format (section 3 of the format spec):
//
//	For each record:
//	  uint32 LE header word
//	      bits 0..23  payload length in bytes (mask 0x00FFFFFF)
//	      bits 24..31 payload type: 0x00 = text (property list), nonzero = binary
//	  <length> bytes  payload
//
// Text payloads are null-terminated property lists; binary payloads (type≠0)
// are skipped (used by the Storage stream and SchLib pin-auxiliary streams).
func parseRecordStream(buf []byte) ([]record.Record, error) {
	r := bytes.NewReader(buf)
	var records []record.Record
	for r.Len() > 0 {
		var hdr uint32
		if err := binary.Read(r, binary.LittleEndian, &hdr); err != nil {
			break
		}
		payloadType := byte(hdr >> 24)
		size := int(hdr & 0x00FFFFFF)
		if size > r.Len() {
			break
		}
		prop := make([]byte, size)
		if _, err := io.ReadFull(r, prop); err != nil {
			break
		}
		if payloadType != 0 {
			// Binary payload — not a property list; skip.
			continue
		}
		// Strip null terminator and any trailing garbage.
		if idx := bytes.IndexByte(prop, 0); idx >= 0 {
			prop = prop[:idx]
		}
		rec, err := parsePropString(string(prop))
		if err != nil {
			continue
		}
		records = append(records, rec)
	}
	return records, nil
}

// parsePropString converts a pipe-delimited "|KEY=VALUE|KEY=VALUE|" string into
// a Record. Keys are uppercased. The RECORD property sets Type; INDEXINSHEET
// sets Index.
func parsePropString(s string) (record.Record, error) {
	props := make(map[string]string)
	for _, field := range strings.Split(s, "|") {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		idx := strings.IndexByte(field, '=')
		if idx < 0 {
			continue
		}
		k := strings.ToUpper(strings.TrimSpace(field[:idx]))
		v := strings.TrimSpace(field[idx+1:])
		props[k] = v
	}
	if len(props) == 0 {
		return record.Record{}, fmt.Errorf("empty record")
	}

	var rec record.Record
	rec.Props = props
	rec.Index = -1

	if v, ok := props["RECORD"]; ok {
		var err error
		fmt.Sscanf(v, "%d", &rec.Type)
		_ = err
	}
	if v, ok := props["INDEXINSHEET"]; ok {
		fmt.Sscanf(v, "%d", &rec.Index)
	}
	return rec, nil
}
