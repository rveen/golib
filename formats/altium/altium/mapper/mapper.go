// Package mapper converts a flat slice of Altium records into a schema.Schematic.
// The central challenge is symbol reconstruction: Altium stores every pin and
// graphic primitive as a child record under its parent COMPONENT via OWNERINDEX,
// with no separate symbol-definition/instance split. The mapper rebuilds that
// split by collecting owned children, translating geometry to the symbol's local
// frame, and hashing the canonical form to deduplicate symbols.
package mapper

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/rveen/golib/formats/altium/altium/record"
	"github.com/rveen/golib/formats/altium/convert"
	"github.com/rveen/golib/formats/altium/emit"
	"github.com/rveen/golib/formats/altium/schema"
)

// Map converts records to a Schematic.  The sheet name and filename are taken
// from the SHEET record when present, or from sheetName/sheetFile parameters.
// coordScale is 10 for binary CFB files (coordinates in decamils) and 1 for ASCII.
func Map(records []record.Record, sheetName, sheetFile string, coordScale int) (*schema.Schematic, *emit.Report, error) {
	if coordScale <= 0 {
		coordScale = 1
	}
	rep := &emit.Report{}
	m := &mapper{
		records:    records,
		byIndex:    make(map[int]record.Record),
		children:   make(map[int][]record.Record),
		symbols:    make(map[schema.SymbolID]*schema.Symbol),
		report:     rep,
		sheetName:  sheetName,
		sheetFile:  sheetFile,
		coordScale: coordScale,
	}
	return m.run()
}

type mapper struct {
	records    []record.Record
	byIndex    map[int]record.Record
	children   map[int][]record.Record // ownerIndex -> []child records
	symbols    map[schema.SymbolID]*schema.Symbol
	report     *emit.Report
	sheetName  string
	sheetFile  string
	coordScale int // 10 for binary CFB (decamils), 1 for ASCII (mils)
}

func (m *mapper) run() (*schema.Schematic, *emit.Report, error) {
	// Step 1: index by INDEXINSHEET and bucket children by OWNERINDEX.
	for _, r := range m.records {
		if r.Index >= 0 {
			m.byIndex[r.Index] = r
		}
		owner := r.IntDef("OWNERINDEX", -1)
		m.children[owner] = append(m.children[owner], r)
	}

	sheet := m.buildSheet()

	sch := &schema.Schematic{
		Sheets:  []*schema.Sheet{sheet},
		Symbols: m.symbols,
		Meta:    schema.Meta{SourceFile: m.sheetFile},
	}
	return sch, m.report, nil
}

func (m *mapper) buildSheet() *schema.Sheet {
	sh := &schema.Sheet{
		Name:     m.sheetName,
		FileName: m.sheetFile,
	}

	// Process each top-level record (ownerIndex == -1 means sheet-level, but
	// COMPONENT records also appear there; filter by RECORD type).
	// We track position i so that buildComponent can look up children by
	// stream position (children store OWNERINDEX = parent_stream_pos - 1).
	for i, r := range m.records {
		switch r.Type {
		case record.TypeSheet:
			m.applySheetRecord(r, sh)
		case record.TypeComponent:
			comp, sym := m.buildComponent(i, r)
			if comp != nil {
				sh.Components = append(sh.Components, comp)
				if _, exists := m.symbols[sym.ID]; !exists {
					m.symbols[sym.ID] = sym
				}
			}
		case record.TypeWire:
			if w := m.buildWire(r); w != nil {
				sh.Wires = append(sh.Wires, w)
			}
		case record.TypeBus:
			if b := m.buildBus(r); b != nil {
				sh.Buses = append(sh.Buses, b)
			}
		case record.TypeJunction:
			sh.Junctions = append(sh.Junctions, m.readPoint(r, "LOCATION"))
		case record.TypeNetLabel:
			if nl := m.buildNetLabel(r); nl != nil {
				sh.NetLabels = append(sh.NetLabels, nl)
			}
		case record.TypePowerPort:
			if pp := m.buildPowerPort(r); pp != nil {
				sh.PowerPorts = append(sh.PowerPorts, pp)
			}
		case record.TypePort:
			if p := m.buildPort(r); p != nil {
				sh.Ports = append(sh.Ports, p)
			}
		case record.TypeSheetSymbol:
			if ss := m.buildSheetSymbol(i, r); ss != nil {
				sh.SubSheets = append(sh.SubSheets, ss)
			}
		case record.TypeLabel, record.TypeTextFrame:
			if t := m.buildText(r); t != nil {
				sh.Texts = append(sh.Texts, t)
			}
		case record.TypeLine:
			owner := r.IntDef("OWNERINDEX", -1)
			if owner == -1 {
				sh.Graphics = append(sh.Graphics, m.buildLine(r))
			}
		case record.TypeRectangle:
			owner := r.IntDef("OWNERINDEX", -1)
			if owner == -1 {
				sh.Graphics = append(sh.Graphics, m.buildRect(r))
			}
		case record.TypeEllipse:
			owner := r.IntDef("OWNERINDEX", -1)
			if owner == -1 {
				sh.Graphics = append(sh.Graphics, m.buildEllipse(r))
			}
		case record.TypeArc:
			owner := r.IntDef("OWNERINDEX", -1)
			if owner == -1 {
				sh.Graphics = append(sh.Graphics, m.buildArc(r))
			}
		case record.TypePolyline:
			owner := r.IntDef("OWNERINDEX", -1)
			if owner == -1 {
				sh.Graphics = append(sh.Graphics, m.buildPolyline(r))
			}
		case record.TypePolygon:
			owner := r.IntDef("OWNERINDEX", -1)
			if owner == -1 {
				sh.Graphics = append(sh.Graphics, m.buildPolygon(r))
			}

		// Explicitly skipped / not imported.
		case record.TypeHeader,
			record.TypeDesignator,
			record.TypeParameter,
			record.TypeImplementList,
			record.TypeImplementation,
			record.TypeNoERC,
			record.TypeTemplate,
			record.TypeSheetEntry,
			record.TypeSheetName,
			record.TypeFileName,
			record.TypeBusEntry,
			record.TypeImage,
			record.TypeIEEESymbol,
			record.TypeParameterSet,
			record.TypeMapDefinerList,
			record.TypeMapDefiner,
			record.TypeImplParams,
			record.TypeCompileMask,
			record.TypeBlanket,
			record.TypeNote,
			record.TypeHarnessConnector,
			record.TypeHarnessEntry,
			record.TypeHarnessType,
			record.TypeSignalHarness,
			record.TypeHyperlink,
			record.TypePin,           // owned by component; processed in buildComponent
			record.TypeEllipticalArc, // handled if owned by component
			record.TypeRoundRectangle,
			record.TypeBezier,
			record.TypePieChart:
			// no-op for sheet-level processing; child records are handled in buildComponent

		default:
			if _, known := record.TypeName[r.Type]; !known {
				m.report.Add(emit.Warn, schema.Provenance{Record: r.Index, Kind: "unknown"},
					"unknown record type %d at index %d", r.Type, r.Index)
			}
		}
	}
	return sh
}

// applySheetRecord reads the SHEET record to populate paper size.
func (m *mapper) applySheetRecord(r record.Record, sh *schema.Sheet) {
	style := r.IntDef("SHEETSTYLE", 0)
	useCustom := r.Bool("USECUSTOMSHEET")
	// Multiply integer parts by coordScale (10 for binary CFB, which stores
	// coordinates in decamils; FRAC is in 1/10000 regular mils regardless).
	cxM := r.IntDef("CUSTOMX", 0) * m.coordScale
	cxF := r.IntDef("CUSTOMX_FRAC", 0)
	cyM := r.IntDef("CUSTOMY", 0) * m.coordScale
	cyF := r.IntDef("CUSTOMY_FRAC", 0)
	portrait := r.IntDef("WORKSPACEORIENTATION", 0) == 1
	sh.Paper = convert.SheetSize(style, useCustom, cxM, cxF, cyM, cyF, portrait)
	sh.Fonts = m.buildFontTable(r)
}

// emHeightFactor converts an Altium font SIZEn (a line-spacing length) to the
// em height used for rendering. This matches KiCad's altium importer, which
// renders text at font.Size/2 (see ParseLabel/AddTextBox in sch_io_altium.cpp:
// SetTextSize({ font.Size / 2, font.Size / 2 })).
const emHeightFactor = 0.5

// buildFontTable decodes the sheet font table (FONTIDCOUNT + SIZEn/FONTNAMEn/…).
// The returned slice is 0-based; a record's FONTID of n refers to entry n-1.
func (m *mapper) buildFontTable(r record.Record) []schema.Font {
	n := r.IntDef("FONTIDCOUNT", 0)
	if n <= 0 {
		return nil
	}
	fonts := make([]schema.Font, n)
	for i := 1; i <= n; i++ {
		s := strconv.Itoa(i)
		spacing := m.scaleToNm(r.IntDef("SIZE"+s, 0), r.IntDef("SIZE"+s+"_FRAC", 0))
		fonts[i-1] = schema.Font{
			Name:      r.UTF8Str("FONTNAME" + s),
			Height:    schema.Length(float64(spacing) * emHeightFactor),
			Bold:      r.Bool("BOLD" + s),
			Italic:    r.Bool("ITALIC" + s),
			Underline: r.Bool("UNDERLINE" + s),
			Rotation:  schema.Angle(r.IntDef("ROTATION"+s, 0)),
		}
	}
	return fonts
}

// ---------- Component / Symbol reconstruction ----------

func (m *mapper) buildComponent(streamPos int, r record.Record) (*schema.Component, *schema.Symbol) {
	// In binary Altium, children store OWNERINDEX = parent_stream_pos - 1.
	// In ASCII Altium, children store OWNERINDEX = parent INDEXINSHEET.
	// We always use streamPos-1 here; for ASCII the INDEXINSHEET equals
	// streamPos-1 by convention (records are in index order).
	childKey := streamPos - 1
	anchor := m.readPointFrac(r, "LOCATION")

	// Collect all child records of this component.
	owned := m.children[childKey]

	// orient is the component's rotation in 90° CCW steps (0–3).
	// Child record positions are in absolute schematic coordinates (already
	// rotated by orient). buildSymbol de-rotates them to a canonical local
	// frame so that the same physical component at different placements
	// produces the same symbol definition.
	orient := r.IntDef("ORIENTATION", 0)
	rot := convert.ComponentOrientation(orient)
	sym := m.buildSymbol(r, owned, anchor, orient)

	mirrored := r.Bool("ISMIRRORED")
	unit := max(r.IntDef("CURRENTPARTID", 1), 1)

	comp := &schema.Component{
		Symbol:   sym.ID,
		Position: anchor,
		Rotation: rot,
		Mirrored: mirrored,
		Unit:     unit,
		Fields:   m.collectFields(owned, anchor, orient),
		Prov:     schema.Provenance{Record: r.Index, Kind: "COMPONENT"},
	}
	// Extract designator from the owned DESIGNATOR record.
	for _, child := range owned {
		if child.Type == record.TypeDesignator {
			comp.Designator = child.Str("TEXT")
			comp.DesignatorFont = schema.FontRef(child.IntDef("FONTID", 0))
			comp.DesignatorRot = convert.ComponentOrientation(child.IntDef("ORIENTATION", 0))
			comp.DesignatorJust = convert.AltiumJustification(child.IntDef("JUSTIFICATION", 0))
			absPos := m.readPoint(child, "LOCATION")
			comp.DesignatorPos = deRotatePoint(schema.Point{
				X: absPos.X - anchor.X,
				Y: absPos.Y - anchor.Y,
			}, orient)
			break
		}
	}

	return comp, sym
}

// deRotatePoint undoes an Altium component ORIENTATION rotation (0–3, in 90° CCW
// steps) on a point already translated to the component-relative frame.
// Altium stores child coordinates in absolute schematic space; subtracting the
// component anchor gives the rotated-relative frame; this function removes the
// rotation to yield the canonical local frame.
func deRotatePoint(p schema.Point, orient int) schema.Point {
	switch orient & 3 {
	case 1: // 90° CCW applied → undo with 90° CW: (x,y)→(y,−x)
		return schema.Point{X: p.Y, Y: -p.X}
	case 2: // 180° → (x,y)→(−x,−y)
		return schema.Point{X: -p.X, Y: -p.Y}
	case 3: // 270° CCW applied → undo with 90° CCW: (x,y)→(−y,x)
		return schema.Point{X: -p.Y, Y: p.X}
	default:
		return p
	}
}

// deRotateDir undoes an Altium component ORIENTATION rotation on a Dir4 value.
// PINCONGLOMERATE stores the pin's absolute (post-rotation) direction; this
// returns the canonical pre-rotation direction.
func deRotateDir(d schema.Dir4, orient int) schema.Dir4 {
	return schema.Dir4((int(d) - orient + 4) % 4)
}

func (m *mapper) buildSymbol(compRec record.Record, owned []record.Record, anchor schema.Point, orient int) *schema.Symbol {
	libRef := compRec.UTF8Str("LIBREFERENCE")
	partCount := compRec.IntDef("PARTCOUNT", 1)
	dispModeCount := compRec.IntDef("DISPLAYMODECOUNT", 1)

	sym := &schema.Symbol{
		LibRef:     libRef,
		UnitCount:  partCount,
		BodyStyles: dispModeCount,
		Prov:       schema.Provenance{Record: compRec.Index, Kind: "SYMBOL"},
	}

	// Collect pins and graphics, translating from absolute schematic
	// coordinates to the canonical local frame (subtract anchor, then
	// de-rotate by the component's ORIENTATION).
	for _, child := range owned {
		switch child.Type {
		case record.TypePin:
			if p := m.buildPin(child, anchor, orient); p != nil {
				sym.Pins = append(sym.Pins, p)
			}
		case record.TypeLine:
			g := m.buildLine(child)
			g.A = deRotatePoint(subPoint(g.A, anchor), orient)
			g.B = deRotatePoint(subPoint(g.B, anchor), orient)
			sym.Graphics = append(sym.Graphics, g)
		case record.TypeRectangle:
			g := m.buildRect(child)
			g.Box.Min = deRotatePoint(subPoint(g.Box.Min, anchor), orient)
			g.Box.Max = deRotatePoint(subPoint(g.Box.Max, anchor), orient)
			sym.Graphics = append(sym.Graphics, g)
		case record.TypeEllipse:
			g := m.buildEllipse(child)
			g.Center = deRotatePoint(subPoint(g.Center, anchor), orient)
			sym.Graphics = append(sym.Graphics, g)
		case record.TypeArc:
			g := m.buildArc(child)
			g.Center = deRotatePoint(subPoint(g.Center, anchor), orient)
			angleDelta := float64(orient) * 90
			g.Start = convert.NormalizeAngle(g.Start - angleDelta)
			g.End = convert.NormalizeAngle(g.End - angleDelta)
			sym.Graphics = append(sym.Graphics, g)
		case record.TypeEllipticalArc:
			g := m.buildEllArc(child)
			g.Center = deRotatePoint(subPoint(g.Center, anchor), orient)
			angleDelta := float64(orient) * 90
			g.Start = convert.NormalizeAngle(g.Start - angleDelta)
			g.End = convert.NormalizeAngle(g.End - angleDelta)
			sym.Graphics = append(sym.Graphics, g)
		case record.TypePolyline:
			g := m.buildPolyline(child)
			for i := range g.Points {
				g.Points[i] = deRotatePoint(subPoint(g.Points[i], anchor), orient)
			}
			sym.Graphics = append(sym.Graphics, g)
		case record.TypePolygon:
			g := m.buildPolygon(child)
			for i := range g.Points {
				g.Points[i] = deRotatePoint(subPoint(g.Points[i], anchor), orient)
			}
			sym.Graphics = append(sym.Graphics, g)
		case record.TypeBezier:
			g := m.buildBezier(child)
			for i := range g.Points {
				g.Points[i] = deRotatePoint(subPoint(g.Points[i], anchor), orient)
			}
			sym.Graphics = append(sym.Graphics, g)
		}
	}

	sym.ID = canonicalSymbolID(sym)
	return sym
}

// canonicalSymbolID hashes the symbol's pins and graphics for deduplication.
func canonicalSymbolID(sym *schema.Symbol) schema.SymbolID {
	h := sha256.New()

	// Sort pins by (unit, name, number) for a stable hash.
	pins := make([]*schema.Pin, len(sym.Pins))
	copy(pins, sym.Pins)
	sort.Slice(pins, func(i, j int) bool {
		if pins[i].Unit != pins[j].Unit {
			return pins[i].Unit < pins[j].Unit
		}
		if pins[i].Number != pins[j].Number {
			return pins[i].Number < pins[j].Number
		}
		return pins[i].Name < pins[j].Name
	})
	for _, p := range pins {
		fmt.Fprintf(h, "pin|%d|%s|%s|%d|%d|%d|%d|%d\n",
			p.Unit, p.Name, p.Number,
			p.Position.X, p.Position.Y, p.PinLength,
			int(p.Orientation), int(p.Electrical))
	}

	// LibRef and unit count also contribute to the hash so that differently
	// named components with identical geometry get different IDs.
	fmt.Fprintf(h, "libref|%s|units|%d\n", sym.LibRef, sym.UnitCount)

	sum := h.Sum(nil)
	return schema.SymbolID(fmt.Sprintf("%x", sum[:8]))
}

// ---------- Pin ----------

func (m *mapper) buildPin(r record.Record, anchor schema.Point, orient int) *schema.Pin {
	posX := m.scaleToNm(r.IntDef("LOCATION.X", 0), r.IntDef("LOCATION.X_FRAC", 0))
	posY := m.scaleToNm(r.IntDef("LOCATION.Y", 0), r.IntDef("LOCATION.Y_FRAC", 0))

	length := m.scaleToNm(r.IntDef("PINLENGTH", 0), r.IntDef("PINLENGTH_FRAC", 0))
	elec := convert.PinElectrical(r.IntDef("ELECTRICAL", 0))

	// PINCONGLOMERATE bitfield (section 9.5): bits 0-1 = orientation,
	// bit 2 = hidden, bit 3 = name shown, bit 4 = designator shown.
	// The orientation is in absolute schematic space (post-rotation); de-rotate
	// it along with the position to obtain the canonical local-frame value.
	pcon := r.IntDef("PINCONGLOMERATE", 0)
	absOri := convert.PinOrientation(pcon & 0x03)
	hidden := (pcon & 0x04) != 0
	nameVisible := (pcon & 0x08) != 0
	numVisible := (pcon & 0x10) != 0

	relPos := deRotatePoint(schema.Point{X: posX - anchor.X, Y: posY - anchor.Y}, orient)

	return &schema.Pin{
		Name:          r.UTF8Str("NAME"),
		Number:        r.Str("DESIGNATOR"),
		Position:      relPos,
		PinLength:     length,
		Orientation:   deRotateDir(absOri, orient),
		Electrical:    elec,
		NameVisible:   nameVisible,
		NumberVisible: numVisible,
		Hidden:        hidden,
		Unit:          r.IntDef("OWNERPARTID", 1),
		Prov:          schema.Provenance{Record: r.Index, Kind: "PIN"},
	}
}

// ---------- Connectivity records ----------

func (m *mapper) buildWire(r record.Record) *schema.Wire {
	pts := m.readLocationCount(r)
	if len(pts) < 2 {
		return nil
	}
	return &schema.Wire{Points: pts, Prov: schema.Provenance{Record: r.Index, Kind: "WIRE"}}
}

func (m *mapper) buildBus(r record.Record) *schema.Bus {
	pts := m.readLocationCount(r)
	if len(pts) < 2 {
		return nil
	}
	return &schema.Bus{Points: pts, Prov: schema.Provenance{Record: r.Index, Kind: "BUS"}}
}

func (m *mapper) buildNetLabel(r record.Record) *schema.NetLabel {
	text := r.UTF8Str("TEXT")
	if text == "" {
		text = r.UTF8Str("NAME")
	}
	return &schema.NetLabel{
		Text: text,
		Pos:  m.readPoint(r, "LOCATION"),
		Rot:  convert.ComponentOrientation(r.IntDef("ORIENTATION", 0)),
		Just: convert.AltiumJustification(r.IntDef("JUSTIFICATION", 0)),
		Font: schema.FontRef(r.IntDef("FONTID", 0)),
		Prov: schema.Provenance{Record: r.Index, Kind: "NET_LABEL"},
	}
}

func (m *mapper) buildPowerPort(r record.Record) *schema.PowerPort {
	return &schema.PowerPort{
		NetName:     r.UTF8Str("TEXT"),
		Style:       convert.AltiumPowerStyle(r.IntDef("STYLE", 0)),
		ShowNetName: r.Str("SHOWNETNAME") != "F",
		Pos:         m.readPoint(r, "LOCATION"),
		Rot:         convert.ComponentOrientation(r.IntDef("ORIENTATION", 0)),
		Font:        schema.FontRef(r.IntDef("FONTID", 0)),
		Prov:        schema.Provenance{Record: r.Index, Kind: "POWER_PORT"},
	}
}

// buildSheetSymbol converts an Altium SHEET_SYMBOL record (RECORD=15) and its
// owned children into a schema.SheetSymbol. The box LOCATION is the top-left
// corner (Y-up); XSIZE/YSIZE extend right and down. The owned children are the
// sheet entries (RECORD=16) plus the SHEET_NAME (RECORD=32) and FILE_NAME
// (RECORD=33) text records. Children are looked up by stream position exactly
// like buildComponent (binary stores OWNERINDEX = parent_stream_pos − 1).
func (m *mapper) buildSheetSymbol(streamPos int, r record.Record) *schema.SheetSymbol {
	loc := m.readPointFrac(r, "LOCATION")
	xs := m.scaleToNm(r.IntDef("XSIZE", 0), r.IntDef("XSIZE_FRAC", 0))
	ys := m.scaleToNm(r.IntDef("YSIZE", 0), r.IntDef("YSIZE_FRAC", 0))

	ss := &schema.SheetSymbol{
		Box:   schema.RectBox{Min: schema.Point{X: loc.X, Y: loc.Y - ys}, Max: schema.Point{X: loc.X + xs, Y: loc.Y}},
		Style: m.readStroke(r),
		Fill:  m.readFill(r),
		Prov:  schema.Provenance{Record: r.Index, Kind: "SHEET_SYMBOL"},
	}

	for _, c := range m.children[streamPos-1] {
		switch c.Type {
		case record.TypeSheetEntry:
			ss.Entries = append(ss.Entries, m.buildSheetEntry(c, loc, xs, ys))
		case record.TypeSheetName:
			ss.Name = c.UTF8Str("TEXT")
		case record.TypeFileName:
			ss.FileName = c.UTF8Str("TEXT")
		}
	}
	return ss
}

// buildSheetEntry converts an Altium SHEET_ENTRY record (RECORD=16) to a
// schema.SheetEntry. The entry sits on one edge of the parent box: SIDE 0/1
// select the left/right edge, 2/3 the top/bottom edge. DISTANCEFROMTOP locates
// it along that edge, measured (in units of 100 mil) from the top for vertical
// edges (left/right) or from the left for horizontal ones (top/bottom). loc is
// the box top-left corner (Y-up) and xs/ys its width/height.
func (m *mapper) buildSheetEntry(r record.Record, loc schema.Point, xs, ys schema.Length) schema.SheetEntry {
	// DISTANCEFROMTOP is in units of 100 mil (10× the decamil coordinate grid),
	// so scaleToNm's mils argument is the raw value ×10.
	dist := m.scaleToNm(r.IntDef("DISTANCEFROMTOP", 0)*10, r.IntDef("DISTANCEFROMTOP_FRAC1", 0))
	pos := schema.Point{X: loc.X, Y: loc.Y - dist} // SIDE 0: left edge
	switch r.IntDef("SIDE", 0) {
	case 1: // right edge
		pos = schema.Point{X: loc.X + xs, Y: loc.Y - dist}
	case 2: // top edge: dist runs from the left, Y at the box top
		pos = schema.Point{X: loc.X + dist, Y: loc.Y}
	case 3: // bottom edge: dist runs from the left, Y at the box bottom
		pos = schema.Point{X: loc.X + dist, Y: loc.Y - ys}
	}
	return schema.SheetEntry{
		Name:      r.UTF8Str("NAME"),
		Direction: portDirection(r.IntDef("IOTYPE", 0)),
		Pos:       pos,
	}
}

// buildPort converts an Altium PORT record (a hierarchical inter-sheet
// connector) to a schema.Port. The displayed text is in NAME.
func (m *mapper) buildPort(r record.Record) *schema.Port {
	name := r.UTF8Str("NAME")
	if name == "" {
		name = r.UTF8Str("TEXT")
	}
	// STYLE 0–3 are horizontal (None/Left/Right/Left&Right); 4–7 are vertical
	// (None/Top/Bottom/Top&Bottom). The port body runs WIDTH from LOCATION.
	return &schema.Port{
		Name:      name,
		Direction: portDirection(r.IntDef("IOTYPE", 0)),
		Pos:       m.readPoint(r, "LOCATION"),
		Width:     m.scaleToNm(r.IntDef("WIDTH", 0), 0),
		Vertical:  r.IntDef("STYLE", 0) >= 4,
		Just:      portJustification(r.IntDef("ALIGNMENT", 0)),
		Font:      schema.FontRef(r.IntDef("FONTID", 0)),
		Prov:      schema.Provenance{Record: r.Index, Kind: "PORT"},
	}
}

// portDirection maps an Altium port IOTYPE to a schema.PortDir.
func portDirection(io int) schema.PortDir {
	switch io {
	case 1:
		return schema.PortOutput
	case 2:
		return schema.PortInput
	case 3:
		return schema.PortBidi
	default:
		return schema.PortUnspecified
	}
}

// portJustification maps an Altium port ALIGNMENT (0 center, 1 left, 2 right)
// to a schema.Justify anchored on the vertical centre.
func portJustification(align int) schema.Justify {
	switch align {
	case 1:
		return schema.JustifyCenterLeft
	case 2:
		return schema.JustifyCenterRight
	default:
		return schema.JustifyCenterCenter
	}
}

func (m *mapper) buildText(r record.Record) *schema.Text {
	return &schema.Text{
		Content: r.UTF8Str("TEXT"),
		Pos:     m.readPoint(r, "LOCATION"),
		Rot:     convert.ComponentOrientation(r.IntDef("ORIENTATION", 0)),
		Just:    convert.AltiumJustification(r.IntDef("JUSTIFICATION", 0)),
		Font:    schema.FontRef(r.IntDef("FONTID", 0)),
		Prov:    schema.Provenance{Record: r.Index, Kind: record.TypeName[r.Type]},
	}
}

// ---------- Graphic primitives ----------

func (m *mapper) buildLine(r record.Record) schema.Line {
	return schema.Line{
		A:     m.readPointFrac(r, "LOCATION"),
		B:     m.readPointFrac(r, "CORNER"),
		Style: m.readStroke(r),
	}
}

func (m *mapper) buildRect(r record.Record) schema.Rect {
	fill := m.readFill(r)
	return schema.Rect{
		Box:   schema.RectBox{Min: m.readPointFrac(r, "LOCATION"), Max: m.readPointFrac(r, "CORNER")},
		Style: m.readStroke(r),
		Fill:  fill,
	}
}

func (m *mapper) buildEllipse(r record.Record) schema.Ellipse {
	cx := m.scaleToNm(r.IntDef("LOCATION.X", 0), r.IntDef("LOCATION.X_FRAC", 0))
	cy := m.scaleToNm(r.IntDef("LOCATION.Y", 0), r.IntDef("LOCATION.Y_FRAC", 0))
	rx := m.scaleToNm(r.IntDef("RADIUS", 0), r.IntDef("RADIUS_FRAC", 0))
	ry := m.scaleToNm(r.IntDef("SECONDARYRADIUS", 0), r.IntDef("SECONDARYRADIUS_FRAC", 0))
	if ry == 0 {
		ry = rx
	}
	fill := m.readFill(r)
	return schema.Ellipse{
		Center: schema.Point{X: cx, Y: cy},
		RX:     rx,
		RY:     ry,
		Style:  m.readStroke(r),
		Fill:   fill,
	}
}

func (m *mapper) buildArc(r record.Record) schema.Arc {
	cx := m.scaleToNm(r.IntDef("LOCATION.X", 0), r.IntDef("LOCATION.X_FRAC", 0))
	cy := m.scaleToNm(r.IntDef("LOCATION.Y", 0), r.IntDef("LOCATION.Y_FRAC", 0))
	rad := m.scaleToNm(r.IntDef("RADIUS", 0), r.IntDef("RADIUS_FRAC", 0))
	start := parseFloat(r.Str("STARTANGLE"))
	end := parseFloat(r.Str("ENDANGLE"))
	return schema.Arc{
		Center: schema.Point{X: cx, Y: cy},
		Radius: rad,
		Start:  start,
		End:    end,
		Style:  m.readStroke(r),
	}
}

func (m *mapper) buildEllArc(r record.Record) schema.EllArc {
	cx := m.scaleToNm(r.IntDef("LOCATION.X", 0), r.IntDef("LOCATION.X_FRAC", 0))
	cy := m.scaleToNm(r.IntDef("LOCATION.Y", 0), r.IntDef("LOCATION.Y_FRAC", 0))
	rx := m.scaleToNm(r.IntDef("RADIUS", 0), r.IntDef("RADIUS_FRAC", 0))
	ry := m.scaleToNm(r.IntDef("SECONDARYRADIUS", 0), r.IntDef("SECONDARYRADIUS_FRAC", 0))
	if ry == 0 {
		ry = rx
	}
	start := parseFloat(r.Str("STARTANGLE"))
	end := parseFloat(r.Str("ENDANGLE"))
	return schema.EllArc{
		Center: schema.Point{X: cx, Y: cy},
		RX:     rx, RY: ry,
		Start: start, End: end,
		Style: m.readStroke(r),
	}
}

func (m *mapper) buildPolyline(r record.Record) schema.Polyline {
	return schema.Polyline{
		Points: m.readLocationCount(r),
		Style:  m.readStroke(r),
	}
}

func (m *mapper) buildPolygon(r record.Record) schema.Polygon {
	fill := m.readFill(r)
	return schema.Polygon{
		Points: m.readLocationCount(r),
		Style:  m.readStroke(r),
		Fill:   fill,
	}
}

func (m *mapper) buildBezier(r record.Record) schema.Bezier {
	return schema.Bezier{
		Points: m.readLocationCount(r),
		Style:  m.readStroke(r),
	}
}

// ---------- Field helpers ----------

func (m *mapper) collectFields(owned []record.Record, anchor schema.Point, orient int) []schema.Field {
	var fields []schema.Field
	for _, child := range owned {
		if child.Type == record.TypeParameter {
			absPos := m.readPoint(child, "LOCATION")
			localPos := deRotatePoint(schema.Point{
				X: absPos.X - anchor.X,
				Y: absPos.Y - anchor.Y,
			}, orient)
			fields = append(fields, schema.Field{
				Name:    child.UTF8Str("NAME"),
				Value:   child.UTF8Str("TEXT"),
				Visible: !child.Bool("ISHIDDEN"),
				Font:    schema.FontRef(child.IntDef("FONTID", 0)),
				Pos:     localPos,
				Rot:     convert.ComponentOrientation(child.IntDef("ORIENTATION", 0)),
				Just:    convert.AltiumJustification(child.IntDef("JUSTIFICATION", 0)),
			})
		}
	}
	return fields
}

// ---------- Coordinate helpers ----------

// scaleToNm converts mils (in the file's native coordinate unit) to nanometres,
// applying m.coordScale to the integer part.  For binary CFB files coordScale=10
// because the binary format stores coordinates in decamils (10 mils per unit).
func (m *mapper) scaleToNm(mils, frac int) schema.Length {
	return convert.MilsToNm(mils*m.coordScale, frac)
}

func (m *mapper) readPoint(r record.Record, prefix string) schema.Point {
	x := m.scaleToNm(r.IntDef(prefix+".X", 0), 0)
	y := m.scaleToNm(r.IntDef(prefix+".Y", 0), 0)
	return schema.Point{X: x, Y: y}
}

func (m *mapper) readPointFrac(r record.Record, prefix string) schema.Point {
	x := m.scaleToNm(r.IntDef(prefix+".X", 0), r.IntDef(prefix+".X_FRAC", 0))
	y := m.scaleToNm(r.IntDef(prefix+".Y", 0), r.IntDef(prefix+".Y_FRAC", 0))
	return schema.Point{X: x, Y: y}
}

// readLocationCount reads Xn/Yn point sequences (section 5.7).
// Each point may carry _FRAC companions for sub-unit precision.
func (m *mapper) readLocationCount(r record.Record) []schema.Point {
	n := r.IntDef("LOCATIONCOUNT", 0)
	if n == 0 {
		return nil
	}
	pts := make([]schema.Point, 0, n)
	for i := 1; i <= n; i++ {
		s := strconv.Itoa(i)
		x := m.scaleToNm(r.IntDef("X"+s, 0), r.IntDef("X"+s+"_FRAC", 0))
		y := m.scaleToNm(r.IntDef("Y"+s, 0), r.IntDef("Y"+s+"_FRAC", 0))
		pts = append(pts, schema.Point{X: x, Y: y})
	}
	return pts
}

func (m *mapper) readStroke(r record.Record) schema.Stroke {
	w := m.scaleToNm(r.IntDef("LINEWIDTH", 1), 0)
	color := convert.BGRToColor(uint32(r.IntDef("COLOR", 0)))
	return schema.Stroke{Width: w, Color: color}
}

func (m *mapper) readFill(r record.Record) *schema.Color {
	// ISSOLID is the canonical fill flag (section 8); AREACOLOR without
	// ISSOLID means the color is present but the shape is not filled.
	if !r.Bool("ISSOLID") {
		return nil
	}
	c := convert.BGRToColor(uint32(r.IntDef("AREACOLOR", 0xFFFFFF)))
	return &c
}

// ---------- Utilities ----------

func subPoint(a, b schema.Point) schema.Point {
	return schema.Point{X: a.X - b.X, Y: a.Y - b.Y}
}

func parseFloat(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	v, _ := strconv.ParseFloat(s, 64)
	return v
}
