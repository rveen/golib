// Package pcbreader decodes Altium .PcbDoc files into a RawBoard structure.
// Both text-format storages (Board6, Components6, Nets6, Polygons6) and
// binary-format storages (Arcs6, Tracks6, Vias6, Pads6, Texts6, Fills6,
// Regions6, BoardRegions) are parsed.
package pcbreader

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"strings"
	"unicode/utf16"

	"github.com/richardlehane/mscfb"

	"github.com/rveen/golib/formats/altium/altium/record"
)

// RawBoard holds all data extracted from a .PcbDoc file.
// Text-storage records are in record.Record slices.
// Binary primitives are in typed slices with native coordinate units (int32,
// where 1 unit = 0.1 µin = 2.54 nm).
type RawBoard struct {
	BoardProps    []record.Record // Board6
	ComponentRecs []record.Record // Components6
	NetRecs       []record.Record // Nets6
	PolygonRecs   []record.Record // Polygons6
	Arcs          []RawArc
	Pads          []RawPad
	Vias          []RawVia
	Tracks        []RawTrack
	Texts         []RawText
	Fills         []RawFill
	Regions       []RawRegion // Regions6 + BoardRegions
}

// RawArc is a decoded arc record (record_type=1).
type RawArc struct {
	Layer      uint8
	Net        uint16
	Polygon    uint16 // 0xFFFF = not part of a pour; else owning polygon index
	Component  uint16
	CenterX    int32
	CenterY    int32
	Radius     int32
	StartAngle float64
	EndAngle   float64
	Width      int32
}

// RawTrack is a decoded track segment record (record_type=4).
type RawTrack struct {
	Layer     uint8
	Net       uint16
	Polygon   uint16 // 0xFFFF = routing track; else pour-fill track (skip in mapper)
	Component uint16
	StartX    int32
	StartY    int32
	EndX      int32
	EndY      int32
	Width     int32
}

// RawVia is a decoded via record (record_type=3).
type RawVia struct {
	Net        uint16
	PosX       int32
	PosY       int32
	Diameter   int32
	HoleSize   int32
	StartLayer uint8
	EndLayer   uint8
}

// RawPad is a decoded pad record (record_type=2).
type RawPad struct {
	Designator string
	Layer      uint8
	Net        uint16
	Component  uint16
	PosX       int32
	PosY       int32
	TopSizeX   int32
	TopSizeY   int32
	MidSizeX   int32
	MidSizeY   int32
	BotSizeX   int32
	BotSizeY   int32
	HoleSize   int32
	TopShape   uint8
	BotShape   uint8
	Rotation   float64
	Plated     bool
	// AltShape/CornerRadius come from the optional sub-record 6 padstack block.
	// AltShape == 9 (ROUNDRECT) promotes a CIRCLE pad to a rounded rectangle whose
	// KiCad corner ratio is CornerRadius/200. Top layer (index 0) only.
	AltShape     uint8
	CornerRadius uint8
}

// RawText is a decoded text record (record_type=5).
type RawText struct {
	Layer        uint8
	Component    uint16
	PosX         int32
	PosY         int32
	Height       uint32
	Rotation     float64
	Mirrored     bool
	StrokeWidth  uint32
	IsComment    bool
	IsDesignator bool
	FontType     uint8 // ALTIUM_TEXT_TYPE: 0=stroke, 1=truetype, 2=barcode
	FontName     string
	Text         string
}

// RawFill is a decoded rectangular fill record (record_type=6).
type RawFill struct {
	Layer     uint8
	Net       uint16
	Component uint16
	Pos1X     int32
	Pos1Y     int32
	Pos2X     int32
	Pos2Y     int32
	Rotation  float64
}

// RawRegion is a decoded region record (record_type=11).
// Vertices are in native units (int32, where 1 unit = 0.1 µin = 2.54 nm).
// Non-extended regions (Regions6/BoardRegions) store vertices as float64 in the
// file; the parser converts them to int32 by rounding.
type RawRegion struct {
	Layer         uint8
	Net           uint16 // from binary header offset 3
	Polygon       uint16 // 0xFFFF = not a pour fill; else owning polygon index
	Component     uint16 // 0xFFFF = board-level
	IsKeepout     bool   // flags2 == 2: region is a keepout rule area
	IsBoardCutout bool   // KIND=0 + ISBOARDCUTOUT=true: contributes to board Edge.Cuts
	Kind          int    // ALTIUM_REGION_KIND: 0=copper,1=polygon-cutout,2=dashed,4=cavity
	Storage       string // source storage: "Regions6", "ShapeBasedRegions6", or "BoardRegions"
	Vertices      [][2]int32
	VertexArcs    []RawVertexArc // arc info per outline vertex (extended regions only; aligned with Vertices)
	Holes         [][][2]int32   // hole polygons cutting into this region's copper
}

// RawVertexArc carries the per-vertex arc parameters of an extended (shape-based)
// region outline. When IsArc is false the vertex is a straight point and the
// other fields are unused. Coordinates and radius are native int32 units.
type RawVertexArc struct {
	IsArc      bool
	CX, CY     int32
	Radius     int32
	StartAngle float64 // degrees
	EndAngle   float64 // degrees
}

// ReadFile opens a .PcbDoc CFB file and returns the raw board data.
func ReadFile(path string) (*RawBoard, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return readFrom(f)
}

// ReadBytes reads a .PcbDoc CFB file from an in-memory byte slice.
func ReadBytes(data []byte) (*RawBoard, error) {
	return readFrom(bytes.NewReader(data))
}

// readFrom parses a .PcbDoc CFB container from any io.ReaderAt.
func readFrom(rs io.ReaderAt) (*RawBoard, error) {
	doc, err := mscfb.New(rs)
	if err != nil {
		return nil, fmt.Errorf("CFB open: %w", err)
	}

	// Collect Data streams keyed by parent storage name.
	// mscfb entry.Path contains the ancestor directory names, NOT the entry's
	// own name. For a stream "Tracks6/Data", entry.Path = ["Tracks6"] and
	// entry.Name = "Data".
	streamBufs := make(map[string][]byte)
	for entry, err := doc.Next(); err == nil; entry, err = doc.Next() {
		if entry.Name != "Data" || len(entry.Path) < 1 {
			continue
		}
		parentStorage := entry.Path[len(entry.Path)-1]
		buf := make([]byte, entry.Size)
		if _, err := io.ReadFull(doc, buf); err != nil {
			return nil, fmt.Errorf("reading %s/Data: %w", parentStorage, err)
		}
		streamBufs[parentStorage] = buf
	}

	rb := &RawBoard{}

	textStorages := map[string]*[]record.Record{
		"Board6":      &rb.BoardProps,
		"Components6": &rb.ComponentRecs,
		"Nets6":       &rb.NetRecs,
		"Polygons6":   &rb.PolygonRecs,
	}
	for name, dst := range textStorages {
		if buf, ok := streamBufs[name]; ok {
			recs, err := parseTextStorage(buf)
			if err != nil {
				return nil, fmt.Errorf("parsing %s: %w", name, err)
			}
			*dst = recs
		}
	}

	binaryStorages := []string{
		"Arcs6", "Pads6", "Vias6", "Tracks6",
		"Texts6", "Fills6", "Regions6", "ShapeBasedRegions6", "BoardRegions",
	}
	for _, name := range binaryStorages {
		if buf, ok := streamBufs[name]; ok {
			if err := parseBinaryStorage(name, buf, rb); err != nil {
				return nil, fmt.Errorf("parsing %s: %w", name, err)
			}
		}
	}

	return rb, nil
}

// ---------- Text storage parser ----------

// parseTextStorage decodes a sequence of property-list records from a text-format
// storage Data stream. Format: u4 length-prefix (top byte non-zero = binary blob,
// skip); low 24 bits = byte count of pipe-delimited property string.
func parseTextStorage(buf []byte) ([]record.Record, error) {
	r := bytes.NewReader(buf)
	var records []record.Record
	for r.Len() > 0 {
		var hdr uint32
		if err := binary.Read(r, binary.LittleEndian, &hdr); err != nil {
			break
		}
		size := int(hdr & 0x00FFFFFF)
		isBinary := (hdr >> 24) != 0
		if size > r.Len() {
			break
		}
		payload := make([]byte, size)
		if _, err := io.ReadFull(r, payload); err != nil {
			break
		}
		if isBinary {
			continue
		}
		if idx := bytes.IndexByte(payload, 0); idx >= 0 {
			payload = payload[:idx]
		}
		rec, err := parsePropString(string(payload))
		if err != nil {
			continue
		}
		records = append(records, rec)
	}
	return records, nil
}

// parsePropString converts "|KEY=VALUE|KEY=VALUE|" into a record.Record.
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
	rec := record.Record{Props: props, Index: -1}
	if v, ok := props["RECORD"]; ok {
		fmt.Sscanf(v, "%d", &rec.Type)
	}
	if v, ok := props["INDEXINSHEET"]; ok {
		fmt.Sscanf(v, "%d", &rec.Index)
	}
	return rec, nil
}

// ---------- Binary storage parser ----------

// parseBinaryStorage reads binary records from buf and appends them to rb.
// Each record: u1 record_type, then one or more u4-length-prefixed subrecords.
func parseBinaryStorage(name string, buf []byte, rb *RawBoard) error {
	pos := 0
	for pos < len(buf) {
		if pos >= len(buf) {
			break
		}
		recType := buf[pos]
		pos++

		switch recType {
		case 1: // Arc
			sub, n := readSubrecord(buf, pos)
			pos += n
			if sub == nil {
				continue
			}
			if len(sub) < 45 {
				continue
			}
			rb.Arcs = append(rb.Arcs, RawArc{
				Layer:      sub[0],
				Net:        readU2(sub, 3),
				Polygon:    readU2(sub, 5),
				Component:  readU2(sub, 7),
				CenterX:    readS4(sub, 13),
				CenterY:    readS4(sub, 17),
				Radius:     readS4(sub, 21),
				StartAngle: readF8(sub, 25),
				EndAngle:   readF8(sub, 33),
				Width:      readS4(sub, 41),
			})

		case 2: // Pad — 6 subrecords
			// sub1: designator (short pascal string)
			sub1, n1 := readSubrecord(buf, pos)
			pos += n1
			// sub2,3,4: reserved
			_, n2 := readSubrecord(buf, pos)
			pos += n2
			_, n3 := readSubrecord(buf, pos)
			pos += n3
			_, n4 := readSubrecord(buf, pos)
			pos += n4
			// sub5: geometry
			sub5, n5 := readSubrecord(buf, pos)
			pos += n5
			// sub6: optional full pad stack (padstack/size-and-shape block)
			sub6, n6 := readSubrecord(buf, pos)
			pos += n6

			if sub1 == nil || sub5 == nil || len(sub5) < 110 {
				continue
			}
			desig, _ := readPascalStr(sub1, 0)
			// sub6 (when present, >=596 bytes) carries per-layer alt_shape[32] at
			// offset 532 and cornerradius[32] at 564; index 0 is the top layer.
			var altShape, cornerRadius uint8
			if sub6 != nil && len(sub6) >= 596 {
				altShape = sub6[532]
				cornerRadius = sub6[564]
			}
			rb.Pads = append(rb.Pads, RawPad{
				Designator:   desig,
				Layer:        sub5[0],
				Net:          readU2(sub5, 3),
				Component:    readU2(sub5, 7),
				PosX:         readS4(sub5, 13),
				PosY:         readS4(sub5, 17),
				TopSizeX:     readS4(sub5, 21),
				TopSizeY:     readS4(sub5, 25),
				MidSizeX:     readS4(sub5, 29),
				MidSizeY:     readS4(sub5, 33),
				BotSizeX:     readS4(sub5, 37),
				BotSizeY:     readS4(sub5, 41),
				HoleSize:     readS4(sub5, 45),
				TopShape:     sub5[49],
				BotShape:     sub5[51],
				Rotation:     readF8(sub5, 52),
				Plated:       sub5[60] != 0,
				AltShape:     altShape,
				CornerRadius: cornerRadius,
			})

		case 3: // Via
			sub, n := readSubrecord(buf, pos)
			pos += n
			if sub == nil || len(sub) < 31 {
				continue
			}
			rb.Vias = append(rb.Vias, RawVia{
				Net:        readU2(sub, 3),
				PosX:       readS4(sub, 13),
				PosY:       readS4(sub, 17),
				Diameter:   readS4(sub, 21),
				HoleSize:   readS4(sub, 25),
				StartLayer: sub[29],
				EndLayer:   sub[30],
			})

		case 4: // Track
			sub, n := readSubrecord(buf, pos)
			pos += n
			if sub == nil || len(sub) < 33 {
				continue
			}
			rb.Tracks = append(rb.Tracks, RawTrack{
				Layer:     sub[0],
				Net:       readU2(sub, 3),
				Polygon:   readU2(sub, 5),
				Component: readU2(sub, 7),
				StartX:    readS4(sub, 13),
				StartY:    readS4(sub, 17),
				EndX:      readS4(sub, 21),
				EndY:      readS4(sub, 25),
				Width:     readS4(sub, 29),
			})

		case 5: // Text — 2 subrecords
			sub1, n1 := readSubrecord(buf, pos)
			pos += n1
			sub2, n2 := readSubrecord(buf, pos)
			pos += n2
			if sub1 == nil || len(sub1) < 42 || sub2 == nil {
				continue
			}
			txt, _ := readPascalStr(sub2, 0)
			height := binary.LittleEndian.Uint32(sub1[21:25])
			strokeW := binary.LittleEndian.Uint32(sub1[36:40])
			// Font type/name live in the extended part of subrecord1 (>=123 bytes);
			// shorter records are stroke fonts. fonttype@43, fontname@46 (UTF-16LE,64).
			var fontType uint8
			var fontName string
			if len(sub1) >= 123 {
				fontType = sub1[43]
				fontName = decodeUTF16LE(sub1[46:110])
			}
			rb.Texts = append(rb.Texts, RawText{
				Layer:        sub1[0],
				Component:    readU2(sub1, 7),
				PosX:         readS4(sub1, 13),
				PosY:         readS4(sub1, 17),
				Height:       height,
				Rotation:     readF8(sub1, 27),
				Mirrored:     sub1[35] != 0,
				StrokeWidth:  strokeW,
				IsComment:    sub1[40] != 0,
				IsDesignator: sub1[41] != 0,
				FontType:     fontType,
				FontName:     fontName,
				Text:         txt,
			})

		case 6: // Fill
			sub, n := readSubrecord(buf, pos)
			pos += n
			if sub == nil || len(sub) < 37 {
				continue
			}
			rb.Fills = append(rb.Fills, RawFill{
				Layer:     sub[0],
				Net:       readU2(sub, 3),
				Component: readU2(sub, 7),
				Pos1X:     readS4(sub, 13),
				Pos1Y:     readS4(sub, 17),
				Pos2X:     readS4(sub, 21),
				Pos2Y:     readS4(sub, 25),
				Rotation:  readF8(sub, 29),
			})

		case 0x0B: // Region (11)
			sub, n := readSubrecord(buf, pos)
			pos += n
			// Minimum: 18-byte binary header + 4-byte property-string length prefix = 22 bytes.
			if len(sub) < 22 {
				continue
			}
			layer := sub[0]
			flags2 := sub[2]
			net := readU2(sub, 3)
			polygon := readU2(sub, 5)
			comp := readU2(sub, 7)
			// offset 9: 5 skip bytes
			holecount := int(readU2(sub, 14))
			// offset 16: 2 skip bytes
			// offset 18: u32-length-prefixed property string
			// Top byte of u32 is non-zero for binary blobs; low 24 bits = byte count (includes NUL).
			rawLen := binary.LittleEndian.Uint32(sub[18:])
			isBinBlob := (rawLen >> 24) != 0
			propLen := int(rawLen & 0x00FFFFFF)
			if isBinBlob || propLen == 0 || 22+propLen > len(sub) {
				continue
			}
			propBytes := sub[22 : 22+propLen]
			// Strip trailing NUL if present (C++ includes it in propLen).
			if propBytes[len(propBytes)-1] == 0 {
				propBytes = propBytes[:len(propBytes)-1]
			}

			// Parse the property string for KIND and ISBOARDCUTOUT.
			var kindVal int
			var isBoardCutout bool
			for _, field := range strings.Split(string(propBytes), "|") {
				idx := strings.IndexByte(field, '=')
				if idx < 0 {
					continue
				}
				k := strings.ToUpper(strings.TrimSpace(field[:idx]))
				v := strings.TrimSpace(field[idx+1:])
				switch k {
				case "KIND":
					fmt.Sscanf(v, "%d", &kindVal)
				case "ISBOARDCUTOUT":
					isBoardCutout = strings.EqualFold(v, "true")
				}
			}

			// Vertex data begins immediately after the property string block.
			vtxOff := 22 + propLen
			if vtxOff+4 > len(sub) {
				continue
			}
			count := int(binary.LittleEndian.Uint32(sub[vtxOff:]))
			vtxOff += 4

			// ShapeBasedRegions6 uses extended (int32) vertices; Regions6 and BoardRegions
			// use non-extended (float64) vertices. The storage name is passed as `name`.
			extended := name == "ShapeBasedRegions6"

			// Guard: cap count to remaining buffer size to avoid OOM on corrupt data.
			const maxVerts = 65536
			if count > maxVerts {
				count = maxVerts
			}
			verts := make([][2]int32, 0, count)
			var vertexArcs []RawVertexArc
			if extended {
				// Extended vertices are a fixed 37-byte record each, and the stream
				// contains count+1 of them (the final one closes the loop):
				//   u1 isRound, s4 x, s4 y, s4 cx, s4 cy, s4 radius, f8 a1, f8 a2.
				// Every field is present for every vertex regardless of isRound.
				// Coordinates/radius are Altium native int32 units (0.1 µin), Y-up.
				ecount := count + 1
				if ecount > maxVerts {
					ecount = maxVerts
				}
				vertexArcs = make([]RawVertexArc, 0, ecount)
				for i := 0; i < ecount && vtxOff+37 <= len(sub); i++ {
					isRound := sub[vtxOff] != 0
					x := readS4(sub, vtxOff+1)
					y := readS4(sub, vtxOff+5)
					cx := readS4(sub, vtxOff+9)
					cy := readS4(sub, vtxOff+13)
					radius := readS4(sub, vtxOff+17)
					a1 := readF8(sub, vtxOff+21)
					a2 := readF8(sub, vtxOff+29)
					vtxOff += 37
					verts = append(verts, [2]int32{x, y})
					vertexArcs = append(vertexArcs, RawVertexArc{
						IsArc: isRound, CX: cx, CY: cy, Radius: radius,
						StartAngle: a1, EndAngle: a2,
					})
				}
			} else {
				// Non-extended: each vertex is two f64 values (Altium native units, Y-up).
				// Convert to int32 by rounding so rawToNm() works correctly downstream.
				for i := 0; i < count && vtxOff+16 <= len(sub); i++ {
					x := int32(math.Round(readF8(sub, vtxOff)))
					y := int32(math.Round(readF8(sub, vtxOff+8)))
					verts = append(verts, [2]int32{x, y})
					vtxOff += 16
				}
			}

			// Hole polygons: always stored as float64 pairs, same as non-extended outline.
			holes := make([][][2]int32, 0, holecount)
			for k := 0; k < holecount; k++ {
				if vtxOff+4 > len(sub) {
					break
				}
				hcount := int(binary.LittleEndian.Uint32(sub[vtxOff:]))
				vtxOff += 4
				if hcount > maxVerts {
					hcount = maxVerts
				}
				hv := make([][2]int32, 0, hcount)
				for i := 0; i < hcount && vtxOff+16 <= len(sub); i++ {
					x := int32(math.Round(readF8(sub, vtxOff)))
					y := int32(math.Round(readF8(sub, vtxOff+8)))
					hv = append(hv, [2]int32{x, y})
					vtxOff += 16
				}
				if len(hv) >= 3 {
					holes = append(holes, hv)
				}
			}

			rb.Regions = append(rb.Regions, RawRegion{
				Layer:         layer,
				Net:           net,
				Polygon:       polygon,
				Component:     comp,
				IsKeepout:     flags2 == 2,
				IsBoardCutout: kindVal == 0 && isBoardCutout,
				Kind:          kindVal,
				Storage:       name,
				Vertices:      verts,
				VertexArcs:    vertexArcs,
				Holes:         holes,
			})

		case 0x0C: // ComponentBody — skip
			sub, n := readSubrecord(buf, pos)
			pos += n
			_ = sub

		default:
			// Unknown record type — try to skip one subrecord to stay in sync.
			_, n := readSubrecord(buf, pos)
			if n == 0 {
				return nil // can't advance; abort parsing this storage
			}
			pos += n
		}
	}
	return nil
}

// ---------- Binary read helpers ----------

// readSubrecord reads one u4-length-prefixed subrecord at buf[pos].
// Returns (payload, total_bytes_consumed). payload is nil on error.
// total_bytes_consumed is always 4 + subrecord_length (or 0 on EOF).
func readSubrecord(buf []byte, pos int) ([]byte, int) {
	if pos+4 > len(buf) {
		return nil, 0
	}
	length := int(binary.LittleEndian.Uint32(buf[pos:]))
	pos += 4
	if pos+length > len(buf) {
		return nil, 4 + length // advance past claimed length even if truncated
	}
	return buf[pos : pos+length], 4 + length
}

func readS4(b []byte, off int) int32 {
	return int32(binary.LittleEndian.Uint32(b[off:]))
}

func readU2(b []byte, off int) uint16 {
	return binary.LittleEndian.Uint16(b[off:])
}

func readF8(b []byte, off int) float64 {
	return math.Float64frombits(binary.LittleEndian.Uint64(b[off:]))
}

// readPascalStr reads a short Pascal string: u1 length + length bytes.
// Returns (string, bytes_consumed).
func readPascalStr(b []byte, off int) (string, int) {
	if off >= len(b) {
		return "", 1
	}
	n := int(b[off])
	end := off + 1 + n
	if end > len(b) {
		end = len(b)
	}
	return string(b[off+1 : end]), 1 + n
}

// decodeUTF16LE decodes a fixed-width UTF-16LE byte buffer into a string,
// stopping at the first NUL terminator (used for Altium font names).
func decodeUTF16LE(b []byte) string {
	u := make([]uint16, 0, len(b)/2)
	for i := 0; i+1 < len(b); i += 2 {
		c := binary.LittleEndian.Uint16(b[i:])
		if c == 0 {
			break
		}
		u = append(u, c)
	}
	return string(utf16.Decode(u))
}
