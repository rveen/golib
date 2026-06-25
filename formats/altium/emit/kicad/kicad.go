// Package kicad emits KiCad 7+ S-expression schematics (.kicad_sch) from a
// schema.Schematic.
//
// Y-axis: KiCad Y increases downward; schema Y increases upward.
// Flip is applied exactly once in this package via w.ky().
// Symbol geometry (lib_symbols) is stored in KiCad coordinates.
//
// All coordinates are in mm (KiCad native); conversion: nm / 1e6.
//
// Pin rotation convention (confirmed from real KiCad files):
//
//	DirLeft  → 0°   (connection at left,  stub extends rightward toward body)
//	DirRight → 180° (connection at right, stub extends leftward  toward body)
//	DirUp    → 270° (connection above body, stub extends downward toward body)
//	DirDown  → 90°  (connection below body, stub extends upward  toward body)
package kicad

import (
	"crypto/sha256"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"

	"github.com/rveen/golib/formats/altium/convert"
	"github.com/rveen/golib/formats/altium/emit"
	"github.com/rveen/golib/formats/altium/schema"
)

const version = "20230121"

// Emitter implements emit.Emitter for KiCad schematic output.
type Emitter struct{}

func (Emitter) Name() string { return "kicad" }

// Emit produces one .kicad_sch artifact per sheet.
func (Emitter) Emit(s *schema.Schematic, _ any) ([]emit.Artifact, *emit.Report, error) {
	rep := &emit.Report{}
	var artifacts []emit.Artifact
	for i, sh := range s.Sheets {
		name := sh.Name
		if name == "" {
			name = fmt.Sprintf("sheet%d", i+1)
		}
		data := renderSheet(sh, s.Symbols, rep)
		artifacts = append(artifacts, emit.Artifact{
			Name: name + ".kicad_sch",
			Data: []byte(data),
		})
	}
	return artifacts, rep, nil
}

// ---------- Sheet renderer ----------

func renderSheet(sh *schema.Sheet, syms map[schema.SymbolID]*schema.Symbol, rep *emit.Report) string {
	w := &sexprWriter{pageH: mm(convert.PaperDims(sh.Paper).H)}
	w.open("kicad_sch")
	w.attr("version", version)
	w.attr("generator", `"schconv"`)
	w.writeUUID(sheetUUID(sh.Name))
	writePaper(w, sh.Paper)

	// lib_symbols block — sorted for deterministic output.
	symIDs := make([]string, 0, len(syms))
	for id := range syms {
		symIDs = append(symIDs, string(id))
	}
	sort.Strings(symIDs)
	// Collect unique power port types (by sanitized net name).
	ppLibByName := map[string]schema.PowerStyle{}
	for _, pp := range sh.PowerPorts {
		n := sanitizeName(pp.NetName)
		if _, ok := ppLibByName[n]; !ok {
			ppLibByName[n] = pp.Style
		}
	}
	ppNames := make([]string, 0, len(ppLibByName))
	for n := range ppLibByName {
		ppNames = append(ppNames, n)
	}
	sort.Strings(ppNames)
	w.open("lib_symbols")
	for _, id := range symIDs {
		writeLibSymbol(w, syms[schema.SymbolID(id)], rep)
	}
	for _, n := range ppNames {
		writePowerLibSymbol(w, n, ppLibByName[n])
	}
	w.close()

	// Sheet-level graphics.
	for _, g := range sh.Graphics {
		writeSheetGraphic(w, g)
	}
	// Free texts.
	for _, t := range sh.Texts {
		writeText(w, sh, t)
	}
	// Connectivity.
	for _, wire := range sh.Wires {
		writeWire(w, wire)
	}
	for _, bus := range sh.Buses {
		writeBus(w, bus)
	}
	for _, j := range sh.Junctions {
		writeJunction(w, j)
	}
	for _, nl := range sh.NetLabels {
		writeNetLabel(w, sh, nl)
	}
	connPts := connectionPoints(sh)
	for _, p := range sh.Ports {
		writePort(w, sh, p, connPts)
	}
	projName := sh.Name
	if projName == "" {
		projName = "schconv"
	}
	rootUUID := sheetUUID(sh.Name)
	for _, pp := range sh.PowerPorts {
		writePowerPortInstance(w, sh, pp, projName, rootUUID)
	}
	// Instances.
	for _, comp := range sh.Components {
		sym, ok := syms[comp.Symbol]
		if !ok {
			rep.Add(emit.Warn, comp.Prov, "symbol %s not found for %q", comp.Symbol, comp.Designator)
			continue
		}
		writeSymbolInstance(w, sh, comp, sym, projName, rootUUID)
	}
	// Hierarchical sheet symbols (references to child sheets).
	for _, ss := range sh.SubSheets {
		writeSheetSymbol(w, ss)
	}

	// Sheet instances: required for KiCad to treat the sheet as the root and to
	// assign a page number.
	w.open("sheet_instances")
	w.line(`(path "/" (page "1"))`)
	w.close()

	w.close()
	return w.String()
}

// ---------- lib_symbols ----------

// symLocalName returns the base name used for sub-symbols and lib_id.
// It includes the first 8 characters of the symbol hash to guarantee uniqueness
// when multiple symbols share the same LibRef.
func symLocalName(sym *schema.Symbol) string {
	base := sanitizeName(sym.LibRef)
	if base == "" {
		base = "unnamed"
	}
	id := string(sym.ID)
	if len(id) > 8 {
		id = id[:8]
	}
	return base + "_" + id
}

func symLibID(sym *schema.Symbol) string {
	return "converted:" + symLocalName(sym)
}

func writeLibSymbol(w *sexprWriter, sym *schema.Symbol, rep *emit.Report) {
	localName := symLocalName(sym)
	libID := "converted:" + localName

	// KiCad controls name/number visibility primarily at the symbol level via
	// pin_names/pin_numbers; per-pin hide in effects is not reliably respected.
	// Derive global visibility from whether every pin wants the text hidden.
	allNamesHidden := len(sym.Pins) > 0
	allNumbersHidden := len(sym.Pins) > 0
	for _, p := range sym.Pins {
		if p.NameVisible {
			allNamesHidden = false
		}
		if p.NumberVisible {
			allNumbersHidden = false
		}
	}

	w.open("symbol", q(libID))
	if allNamesHidden {
		w.line("(pin_names (offset 0) hide)")
	} else {
		w.line("(pin_names (offset 1.016))")
	}
	if allNumbersHidden {
		w.line("(pin_numbers hide)")
	}
	w.line("(in_bom yes)")
	w.line("(on_board yes)")
	writeProp(w, "Reference", "?", 0, 3.81, false)
	writeProp(w, "Value", sym.LibRef, 0, -3.81, false)

	// Body graphics in sub-symbol localName_0_1.
	w.open("symbol", q(localName+"_0_1"))
	for _, g := range sym.Graphics {
		writeSymbolGraphic(w, g, rep)
	}
	w.close()

	// Pins in sub-symbol localName_1_1.
	w.open("symbol", q(localName+"_1_1"))
	for _, p := range sym.Pins {
		writePin(w, p)
	}
	w.close()

	w.close()
}

func writePin(w *sexprWriter, p *schema.Pin) {
	elec := pinElecType(p.Electrical)
	rot := pinRotation(p.Orientation)
	// p.Position is the body-attachment end (Altium LOCATION).
	// KiCad (at x y angle) expects the wire-connection end (pin tip).
	cx, cy := pinConnectionEnd(p)
	px := slx(cx)
	py := sly(cy)
	length := mm(p.PinLength)
	if p.PinLength > 0 && length < 0.001 {
		length = 2.54
	}
	if p.Hidden {
		w.open("pin", elec, "line", "hide")
	} else {
		w.open("pin", elec, "line")
	}
	w.line(fmt.Sprintf("(at %s %s %d)", f(px), f(py), rot))
	w.line(fmt.Sprintf("(length %s)", f(length)))
	pinEffects := func(visible bool) string {
		if visible {
			return "(effects (font (size 1.27 1.27)))"
		}
		return "(effects (font (size 1.27 1.27)) hide)"
	}
	w.open("name", q(convert.OverbarAltiumToKicad(p.Name)))
	w.line(pinEffects(p.NameVisible))
	w.close()
	w.open("number", q(p.Number))
	w.line(pinEffects(p.NumberVisible))
	w.close()
	w.close()
}

func writeSymbolGraphic(w *sexprWriter, g schema.Graphic, rep *emit.Report) {
	switch v := g.(type) {
	case schema.Line:
		w.open("polyline")
		w.line(fmt.Sprintf("(pts (xy %s %s) (xy %s %s))",
			f(slx(v.A.X)), f(sly(v.A.Y)), f(slx(v.B.X)), f(sly(v.B.Y))))
		writeStroke(w, v.Style)
		w.line("(fill (type none))")
		w.close()

	case schema.Rect:
		w.open("rectangle")
		w.line(fmt.Sprintf("(start %s %s)", f(slx(v.Box.Min.X)), f(sly(v.Box.Min.Y))))
		w.line(fmt.Sprintf("(end %s %s)", f(slx(v.Box.Max.X)), f(sly(v.Box.Max.Y))))
		writeStroke(w, v.Style)
		writeFill(w, v.Fill)
		w.close()

	case schema.Ellipse:
		if v.RX == v.RY {
			w.open("circle")
			w.line(fmt.Sprintf("(center %s %s)", f(slx(v.Center.X)), f(sly(v.Center.Y))))
			w.line(fmt.Sprintf("(radius %s)", f(mm(v.RX))))
			writeStroke(w, v.Style)
			writeFill(w, v.Fill)
			w.close()
		} else {
			writeEllipseApprox(w, v)
		}

	case schema.Arc:
		writeArcGraphic(w, v)

	case schema.EllArc:
		// Approximate as polyline.
		rep.Add(emit.Info, schema.Provenance{Kind: "kicad"}, "EllArc approximated as polyline")
		writeEllArcApprox(w, v)

	case schema.Polyline:
		w.open("polyline")
		w.line("(pts " + ptsStr(v.Points) + ")")
		writeStroke(w, v.Style)
		w.line("(fill (type none))")
		w.close()

	case schema.Polygon:
		pts := v.Points
		if len(pts) > 0 {
			pts = append(pts, pts[0]) // close
		}
		w.open("polyline")
		w.line("(pts " + ptsStr(pts) + ")")
		writeStroke(w, v.Style)
		writeFill(w, v.Fill)
		w.close()

	default:
		rep.Add(emit.Warn, schema.Provenance{Kind: "kicad"}, "unsupported symbol graphic %T", g)
	}
}

func writeArcGraphic(w *sexprWriter, a schema.Arc) {
	cx := slx(a.Center.X)
	cy := sly(a.Center.Y)
	r := mm(a.Radius)
	// The library frame is Y-up like schema, so angles stay CCW (no negation).
	startDeg := a.Start
	endDeg := a.End
	midDeg := (startDeg + endDeg) / 2

	sx := cx + r*math.Cos(deg2rad(startDeg))
	sy := cy + r*math.Sin(deg2rad(startDeg))
	ex := cx + r*math.Cos(deg2rad(endDeg))
	ey := cy + r*math.Sin(deg2rad(endDeg))
	mx := cx + r*math.Cos(deg2rad(midDeg))
	my := cy + r*math.Sin(deg2rad(midDeg))

	w.open("arc")
	w.line(fmt.Sprintf("(start %s %s)", f(sx), f(sy)))
	w.line(fmt.Sprintf("(mid %s %s)", f(mx), f(my)))
	w.line(fmt.Sprintf("(end %s %s)", f(ex), f(ey)))
	writeStroke(w, a.Style)
	w.line("(fill (type none))")
	w.close()
}

func writeEllipseApprox(w *sexprWriter, e schema.Ellipse) {
	cx := slx(e.Center.X)
	cy := sly(e.Center.Y)
	rx, ry := mm(e.RX), mm(e.RY)
	const n = 36
	var pts []schema.Point
	for i := 0; i <= n; i++ {
		rad := float64(i) * 2 * math.Pi / n
		pts = append(pts, schema.Point{
			X: schema.Length((cx + rx*math.Cos(rad)) * 1e6),
			Y: schema.Length((cy + ry*math.Sin(rad)) * 1e6),
		})
	}
	w.open("polyline")
	w.line("(pts " + ptsStr(pts) + ")")
	writeStroke(w, e.Style)
	writeFill(w, e.Fill)
	w.close()
}

func writeEllArcApprox(w *sexprWriter, e schema.EllArc) {
	cx := slx(e.Center.X)
	cy := sly(e.Center.Y)
	rx, ry := mm(e.RX), mm(e.RY)
	startRad := deg2rad(e.Start)
	endRad := deg2rad(e.End)
	if endRad < startRad {
		endRad += 2 * math.Pi
	}
	const n = 18
	var pts []schema.Point
	for i := 0; i <= n; i++ {
		t := startRad + float64(i)*(endRad-startRad)/n
		pts = append(pts, schema.Point{
			X: schema.Length((cx + rx*math.Cos(t)) * 1e6),
			Y: schema.Length((cy + ry*math.Sin(t)) * 1e6),
		})
	}
	w.open("polyline")
	w.line("(pts " + ptsStr(pts) + ")")
	writeStroke(w, e.Style)
	w.line("(fill (type none))")
	w.close()
}

// ---------- Sheet-level graphics ----------

func writeSheetGraphic(w *sexprWriter, g schema.Graphic) {
	switch v := g.(type) {
	case schema.Line:
		w.open("polyline")
		w.line(fmt.Sprintf("(pts (xy %s %s) (xy %s %s))",
			f(w.kx(v.A.X)), f(w.ky(v.A.Y)), f(w.kx(v.B.X)), f(w.ky(v.B.Y))))
		writeStroke(w, v.Style)
		w.line("(fill (type none))")
		w.close()
	case schema.Rect:
		// Emit as four polylines.
		corners := [4]schema.Point{
			v.Box.Min,
			{X: v.Box.Max.X, Y: v.Box.Min.Y},
			v.Box.Max,
			{X: v.Box.Min.X, Y: v.Box.Max.Y},
		}
		for i := 0; i < 4; i++ {
			a, b := corners[i], corners[(i+1)%4]
			w.open("polyline")
			w.line(fmt.Sprintf("(pts (xy %s %s) (xy %s %s))",
				f(w.kx(a.X)), f(w.ky(a.Y)), f(w.kx(b.X)), f(w.ky(b.Y))))
			writeStroke(w, v.Style)
			w.line("(fill (type none))")
			w.close()
		}
	}
}

// ---------- Hierarchical sheet symbols ----------

// writeSheetSymbol emits an Altium sheet symbol as a KiCad (sheet …) block: the
// box, its Sheetname/Sheetfile properties, and one (pin …) per sheet entry.
//
// KiCad's (at …) is the box top-left corner. In the schema (Y-up) that corner
// is (Box.Min.X, Box.Max.Y); after the ky flip it becomes the visually top-left
// point of the box in the KiCad (Y-down) sheet frame.
func writeSheetSymbol(w *sexprWriter, ss *schema.SheetSymbol) {
	atX, atY := w.kx(ss.Box.Min.X), w.ky(ss.Box.Max.Y)
	width := mm(ss.Box.Max.X - ss.Box.Min.X)
	height := mm(ss.Box.Max.Y - ss.Box.Min.Y)

	w.open("sheet")
	w.line(fmt.Sprintf("(at %s %s)", f(atX), f(atY)))
	w.line(fmt.Sprintf("(size %s %s)", f(width), f(height)))
	w.line("(exclude_from_sim no)")
	w.line("(in_bom yes)")
	w.line("(on_board yes)")
	w.line("(dnp no)")
	w.line("(fields_autoplaced yes)")
	writeSheetStroke(w, ss.Style)
	writeSheetFill(w, ss.Fill)
	w.writeUUID(makeUUID("sheet:" + ss.Name + ":" + ss.FileName))

	// The sheet name label sits just above the box top edge; the file name is
	// hidden, anchored at the corner. KiCad re-places these (fields_autoplaced).
	name := ss.Name
	if name == "" {
		name = "Sheet"
	}
	file := ss.FileName
	if file != "" && !strings.HasSuffix(strings.ToLower(file), ".kicad_sch") {
		file = strings.TrimSuffix(file, ".SchDoc")
		file = strings.TrimSuffix(file, ".schdoc") + ".kicad_sch"
	}
	writeSheetProp(w, "Sheetname", name, atX, atY-0.508, false)
	writeSheetProp(w, "Sheetfile", file, atX, atY, true)

	for _, e := range ss.Entries {
		writeSheetPin(w, ss, e)
	}
	w.close()
}

// writeSheetProp emits a sheet (property …) with KiCad's left-bottom-justified,
// non-name-showing layout used for Sheetname/Sheetfile.
func writeSheetProp(w *sexprWriter, name, value string, x, y float64, hide bool) {
	w.open("property", q(name), q(value))
	w.line(fmt.Sprintf("(at %s %s 0)", f(x), f(y)))
	if hide {
		w.line("(hide yes)")
	}
	w.line("(show_name no)")
	w.line("(do_not_autoplace no)")
	w.line("(effects (font (size 1.27 1.27)) (justify left bottom))")
	w.close()
}

// writeSheetPin emits one (pin …) of a sheet symbol. The pin angle and text
// justification follow the box edge the entry sits on, with the label always
// pointing away from the box: left edge → angle 180/left, right edge → angle
// 0/right, top edge → angle 90/right, bottom edge → angle 270/left.
func writeSheetPin(w *sexprWriter, ss *schema.SheetSymbol, e schema.SheetEntry) {
	angle, hjust := 180, -1 // left edge
	switch {
	case e.Pos.X == ss.Box.Max.X: // right edge
		angle, hjust = 0, +1
	case e.Pos.Y == ss.Box.Max.Y: // top edge
		angle, hjust = 90, +1
	case e.Pos.Y == ss.Box.Min.Y: // bottom edge
		angle, hjust = 270, -1
	}
	w.open("pin", q(convert.OverbarAltiumToKicad(e.Name)), sheetPinShape(e.Direction))
	w.line(fmt.Sprintf("(at %s %s %d)", f(w.kx(e.Pos.X)), f(w.ky(e.Pos.Y)), angle))
	w.writeUUID(makeUUID(fmt.Sprintf("sheetpin:%s:%d:%d:%s", ss.Name, e.Pos.X, e.Pos.Y, e.Name)))
	w.line(fmt.Sprintf("(effects (font (size 1.27 1.27))%s)", justifyClause(hjust, 0)))
	w.close()
}

// sheetPinShape maps a schema.PortDir to a KiCad sheet-pin electrical type.
// KiCad uses the same shape tokens as hierarchical labels.
func sheetPinShape(d schema.PortDir) string { return portShape(d) }

// writeSheetStroke emits a sheet border stroke, carrying the Altium border color
// so the box keeps its source appearance. KiCad's sheet stroke color alpha is a
// 0–1 float (1 = opaque).
func writeSheetStroke(w *sexprWriter, s schema.Stroke) {
	wMM := mm(s.Width)
	if wMM < 0.001 {
		wMM = 0
	}
	c := s.Color
	w.open("stroke")
	w.line(fmt.Sprintf("(width %s)", f(wMM)))
	w.line("(type solid)")
	w.line(fmt.Sprintf("(color %d %d %d 1)", c.R, c.G, c.B))
	w.close()
}

// writeSheetFill emits a sheet fill. A nil fill means no background; otherwise
// the Altium area color is carried through (alpha as a 0–1 float).
func writeSheetFill(w *sexprWriter, fill *schema.Color) {
	if fill == nil {
		w.line("(fill (type none))")
		return
	}
	w.open("fill")
	w.line(fmt.Sprintf("(color %d %d %d 1)", fill.R, fill.G, fill.B))
	w.close()
}

// ---------- Connectivity ----------

func writeWire(w *sexprWriter, wire *schema.Wire) {
	for i := 0; i+1 < len(wire.Points); i++ {
		a, b := wire.Points[i], wire.Points[i+1]
		w.open("wire")
		w.line(fmt.Sprintf("(pts (xy %s %s) (xy %s %s))",
			f(w.kx(a.X)), f(w.ky(a.Y)), f(w.kx(b.X)), f(w.ky(b.Y))))
		w.line("(stroke (width 0) (type default))")
		w.writeUUID(makeUUID(fmt.Sprintf("wire:%d:%d:%d:%d", a.X, a.Y, b.X, b.Y)))
		w.close()
	}
}

func writeBus(w *sexprWriter, bus *schema.Bus) {
	for i := 0; i+1 < len(bus.Points); i++ {
		a, b := bus.Points[i], bus.Points[i+1]
		w.open("bus")
		w.line(fmt.Sprintf("(pts (xy %s %s) (xy %s %s))",
			f(w.kx(a.X)), f(w.ky(a.Y)), f(w.kx(b.X)), f(w.ky(b.Y))))
		w.line("(stroke (width 0) (type bus))")
		w.writeUUID(makeUUID(fmt.Sprintf("bus:%d:%d:%d:%d", a.X, a.Y, b.X, b.Y)))
		w.close()
	}
}

func writeJunction(w *sexprWriter, j schema.Point) {
	w.open("junction")
	w.line(fmt.Sprintf("(at %s %s)", f(w.kx(j.X)), f(w.ky(j.Y))))
	w.line("(diameter 0)")
	w.line("(color 0 0 0 0)")
	w.writeUUID(makeUUID(fmt.Sprintf("jct:%d:%d", j.X, j.Y)))
	w.close()
}

func writeNetLabel(w *sexprWriter, sh *schema.Sheet, nl *schema.NetLabel) {
	angle, h, v := convert.TextPositioning(nl.Just, nl.Rot)
	w.open("label", q(convert.OverbarAltiumToKicad(nl.Text)))
	w.line(fmt.Sprintf("(at %s %s %d)", f(w.kx(nl.Pos.X)), f(w.ky(nl.Pos.Y)), angle))
	w.line(fmt.Sprintf("(effects (font %s)%s)", fontSize(sh.FontHeight(nl.Font)), justifyClause(h, v)))
	w.writeUUID(makeUUID(fmt.Sprintf("nl:%d:%d:%s", nl.Pos.X, nl.Pos.Y, nl.Text)))
	w.close()
}

// connectionPoints collects every wire and bus vertex on the sheet. A port
// connects at whichever of its two ends coincides with one of these points.
func connectionPoints(sh *schema.Sheet) map[schema.Point]bool {
	pts := make(map[schema.Point]bool)
	for _, wire := range sh.Wires {
		for _, pt := range wire.Points {
			pts[pt] = true
		}
	}
	for _, bus := range sh.Buses {
		for _, pt := range bus.Points {
			pts[pt] = true
		}
	}
	return pts
}

// writePort emits an Altium port as a KiCad hierarchical_label. The port name
// becomes the label text; the data-flow direction becomes the label shape.
//
// An Altium port has a body that runs Width from its LOCATION (Pos). The
// electrical connection — and thus the KiCad label anchor — is at whichever
// body end touches a wire, exactly as KiCad's own Altium importer determines it.
// The Altium ALIGNMENT is only the text alignment within the body and does NOT
// indicate the connection side, so it is not used here. The label then points
// away from the wire: an end-connected horizontal port points left (angle 180,
// right-justified); a start-connected one points right (angle 0, left-justified).
func writePort(w *sexprWriter, sh *schema.Sheet, p *schema.Port, connPts map[schema.Point]bool) {
	start := p.Pos
	end := p.Pos
	if p.Vertical {
		end.Y -= p.Width
	} else {
		end.X += p.Width
	}
	// Prefer the end terminus only when it (and not the start) meets a wire;
	// otherwise anchor at the start. This reproduces the observed placement and
	// gives a stable fallback when neither or both ends are connected.
	useEnd := connPts[end] && !connPts[start]

	conn := start
	angle, h := 0, -1
	switch {
	case p.Vertical && useEnd:
		conn, angle, h = end, 270, +1
	case p.Vertical:
		conn, angle, h = start, 90, -1
	case useEnd:
		conn, angle, h = end, 180, +1
	}

	w.open("hierarchical_label", q(convert.OverbarAltiumToKicad(p.Name)))
	w.line(fmt.Sprintf("(shape %s)", portShape(p.Direction)))
	w.line(fmt.Sprintf("(at %s %s %d)", f(w.kx(conn.X)), f(w.ky(conn.Y)), angle))
	w.line(fmt.Sprintf("(effects (font %s)%s)", fontSize(sh.FontHeight(p.Font)), justifyClause(h, 0)))
	w.writeUUID(makeUUID(fmt.Sprintf("port:%d:%d:%s", p.Pos.X, p.Pos.Y, p.Name)))
	w.close()
}

// portShape maps a schema.PortDir to a KiCad hierarchical_label shape token.
func portShape(d schema.PortDir) string {
	switch d {
	case schema.PortInput:
		return "input"
	case schema.PortOutput:
		return "output"
	case schema.PortBidi:
		return "bidirectional"
	default:
		return "passive"
	}
}

// justifyClause formats a KiCad " (justify …)" suffix from KiCad-signed
// alignment values (h: -1 left/+1 right, v: -1 top/+1 bottom). It returns the
// empty string when both axes are centered (KiCad's default), in which case no
// justify clause is emitted.
func justifyClause(h, v int) string {
	toks := make([]string, 0, 2)
	switch {
	case h < 0:
		toks = append(toks, "left")
	case h > 0:
		toks = append(toks, "right")
	}
	switch {
	case v < 0:
		toks = append(toks, "top")
	case v > 0:
		toks = append(toks, "bottom")
	}
	if len(toks) == 0 {
		return ""
	}
	return " (justify " + strings.Join(toks, " ") + ")"
}

// ---------- Power port symbols ----------

// powerLibID returns the KiCad lib_id string for a power port.
func powerLibID(netName string) string { return "power:" + sanitizeName(netName) }

// writePowerLibSymbol defines one power port symbol in the lib_symbols block.
// The body geometry is in the KiCad symbol-library Y-up frame (same as schema):
// the symbol body extends toward −Y so that after KiCad's lib→sheet Y-flip
// it appears below the connection point for the standard downward orientation
// (Altium ORIENTATION=3 / pp.Rot=270°). The instance placement angle rotates
// the symbol for other orientations.
func writePowerLibSymbol(w *sexprWriter, netName string, style schema.PowerStyle) {
	libID := powerLibID(netName)
	base := sanitizeName(netName)

	w.open("symbol", q(libID))
	w.line("(power)")
	w.line("(pin_names (offset 0) hide)")
	w.line("(pin_numbers hide)")
	w.line("(in_bom no)")
	w.line("(on_board yes)")
	writeProp(w, "Reference", "#PWR", 0, 0, true)
	writeProp(w, "Value", netName, 0, -2.032, false) // below bars, after Y-flip → visually below

	// Body graphics in the _0_1 sub-symbol.
	w.open("symbol", q(base+"_0_1"))
	writePowerBodyGraphic(w, style)
	w.close()

	// Connection pin in the _1_1 sub-symbol.
	// at (0,0) angle=270 → direction from body to tip is 270° CW in Y-up = upward.
	w.open("symbol", q(base+"_1_1"))
	w.open("pin", "power_in", "line")
	w.line("(at 0 0 270)")
	w.line("(length 0)")
	w.open("name", q(""))
	w.line("(effects (font (size 1.27 1.27)) hide)")
	w.close()
	w.open("number", q("1"))
	w.line("(effects (font (size 1.27 1.27)) hide)")
	w.close()
	w.close() // pin
	w.close() // _1_1

	w.close() // symbol
}

// writePowerBodyGraphic emits polylines for a power port symbol body.
// All coordinates are in mm in the lib Y-up frame with body extending toward −Y.
func writePowerBodyGraphic(w *sexprWriter, style schema.PowerStyle) {
	const stemLen = 0.762 // 30 mil
	const b1h = 1.27      // 50 mil half-width
	const b2h = 0.889     // 35 mil
	const b3h = 0.508     // 20 mil
	const b2off = 1.143   // 45 mil offset from connection
	const b3off = 1.524   // 60 mil

	const e1h = 1.524 // earth: 60 mil
	const e2h = 1.143 // 45 mil
	const e3h = 0.762 // 30 mil
	const e4h = 0.381 // 15 mil
	const e2off = 1.143
	const e3off = 1.524
	const e4off = 1.905

	ppLine := func(x1, y1, x2, y2 float64) {
		w.open("polyline")
		w.line(fmt.Sprintf("(pts (xy %s %s) (xy %s %s))", f(x1), f(y1), f(x2), f(y2)))
		w.line("(stroke (width 0) (type default))")
		w.line("(fill (type none))")
		w.close()
	}

	switch style {
	case schema.PowerStyleGND:
		ppLine(0, 0, 0, -stemLen)
		ppLine(-b1h, -stemLen, b1h, -stemLen)
		ppLine(-b2h, -b2off, b2h, -b2off)
		ppLine(-b3h, -b3off, b3h, -b3off)
	case schema.PowerStyleEarth:
		ppLine(0, 0, 0, -stemLen)
		ppLine(-e1h, -stemLen, e1h, -stemLen)
		ppLine(-e2h, -e2off, e2h, -e2off)
		ppLine(-e3h, -e3off, e3h, -e3off)
		ppLine(-e4h, -e4off, e4h, -e4off)
	default: // Bar, Arrow, Tee — single horizontal bar
		ppLine(0, 0, 0, -stemLen)
		ppLine(-b1h, -stemLen, b1h, -stemLen)
	}
}

// powerLabelDistance returns, in nanometres, how far the net-name label sits
// from the connection point along the port's pointing direction. It clears the
// style's graphic body (see writePowerBodyGraphic) plus a margin for the text.
func powerLabelDistance(style schema.PowerStyle) float64 {
	const margin = 1.524e6 // 60 mil
	switch style {
	case schema.PowerStyleGND:
		return 1.524e6 + margin // bottom bar at -b3off
	case schema.PowerStyleEarth:
		return 1.905e6 + margin // bottom bar at -e4off
	default: // Bar, Arrow, Tee — single bar at -stemLen
		return 0.762e6 + margin
	}
}

// writeInstances emits the (instances …) block that binds a symbol placement to
// a reference designator on the root sheet. Without this block KiCad treats the
// symbol as unannotated and ignores the Reference property text.
func writeInstances(w *sexprWriter, projName, rootUUID, ref string, unit int) {
	w.open("instances")
	w.open("project", q(projName))
	w.open("path", q("/"+rootUUID))
	w.line(fmt.Sprintf("(reference %s) (unit %d)", q(ref), unit))
	w.close() // path
	w.close() // project
	w.close() // instances
}

// writePowerPortInstance emits a power port as a proper KiCad power symbol instance.
func writePowerPortInstance(w *sexprWriter, sh *schema.Sheet, pp *schema.PowerPort, projName, rootUUID string) {
	libID := powerLibID(pp.NetName)
	x := w.kx(pp.Pos.X)
	y := w.ky(pp.Pos.Y)
	// KiCad placement angle: the lib_symbol body is at −Y in lib Y-up, so after
	// the lib→sheet Y-flip it points down (the +Y screen direction) at angle 0 —
	// the same as ORIENTATION=3 (pp.Rot=270). The body must end up pointing along
	// pp.Rot, which the lib→sheet flip turns into kicadAngle = pp.Rot + 90:
	// ORIENTATION=0→90°, 1→180°, 2→270°, 3→0°. The earlier (270−pp.Rot) form
	// happened to agree for the vertical orientations (90/270) but was 180° off
	// for the horizontal ones (0/180), flipping e.g. GND_HS1/HS2/HS3/LS_DRV.
	kicadAngle := ((int(pp.Rot)+90)%360 + 360) % 360

	w.open("symbol")
	w.line(fmt.Sprintf("(lib_id %s)", q(libID)))
	w.line(fmt.Sprintf("(at %s %s %d)", f(x), f(y), kicadAngle))
	w.line("(unit 1)")
	w.line("(in_bom no)")
	w.line("(on_board yes)")
	w.writeUUID(makeUUID(fmt.Sprintf("pp:%d:%d:%s", pp.Pos.X, pp.Pos.Y, pp.NetName)))

	// Reference property: always hidden.
	writePropAt(w, "Reference", "#PWR", x, y, 0, 0, 0, schema.DefaultFontHeight, true)
	// Value property: always hidden. The net name is shown instead as a standalone
	// (text) element below (see writePowerPortLabel). A power symbol's value field
	// is rendered at its stored angle by KiCad but with the instance rotation added
	// by Kicanvas, so for a rotated port the two viewers disagree (vertical vs
	// horizontal). A standalone text has no parent symbol, so its angle is absolute
	// in both viewers — the net name then reads horizontally everywhere while the
	// power symbol keeps its global-net (#PWR) connectivity.
	writePropAt(w, "Value", convert.OverbarAltiumToKicad(pp.NetName), x, y, 0, 0, 0, sh.FontHeight(pp.Font), true)

	w.open("pin", q("1"))
	w.writeUUID(makeUUID(fmt.Sprintf("pp_pin:%d:%d:%s", pp.Pos.X, pp.Pos.Y, pp.NetName)))
	w.close()

	writeInstances(w, projName, rootUUID, "#PWR", 1)

	w.close() // symbol

	if pp.ShowNetName {
		writePowerPortLabel(w, sh, pp)
	}
}

// writePowerPortLabel emits the power port's net name as a standalone schematic
// (text) element, always horizontal. The anchor sits beyond the symbol graphic
// in the port's pointing direction (pp.Rot); the justification then extends the
// text further outward so its near edge clears the graphic: a port pointing
// right/left is left/right-justified, one pointing up/down is bottom/top-justified.
func writePowerPortLabel(w *sexprWriter, sh *schema.Sheet, pp *schema.PowerPort) {
	dist := powerLabelDistance(pp.Style)
	rad := deg2rad(float64(pp.Rot))
	lx := pp.Pos.X + schema.Length(math.Round(math.Cos(rad)*dist))
	ly := pp.Pos.Y + schema.Length(math.Round(math.Sin(rad)*dist))
	h, v := powerLabelJustify(pp.Rot)

	w.open("text", q(convert.OverbarAltiumToKicad(pp.NetName)))
	w.line(fmt.Sprintf("(at %s %s 0)", f(w.kx(lx)), f(w.ky(ly))))
	w.line(fmt.Sprintf("(effects (font %s)%s)", fontSize(sh.FontHeight(pp.Font)), justifyClause(h, v)))
	w.writeUUID(makeUUID(fmt.Sprintf("pp_lbl:%d:%d:%s", pp.Pos.X, pp.Pos.Y, pp.NetName)))
	w.close()
}

// powerLabelJustify returns the KiCad-signed justification (h: -1 left/+1 right,
// v: -1 top/+1 bottom) that anchors a horizontal label by the edge nearest the
// power symbol, so the text extends away from the graphic in screen space. The
// argument is the Altium pointing direction (Y-up); the +Y up vs KiCad's +Y down
// flip is why pointing up (90°) maps to "bottom" and pointing down (270°) to "top".
func powerLabelJustify(rot schema.Angle) (h, v int) {
	switch ((int(rot) % 360) + 360) % 360 {
	case 0: // points right → text to the right, anchored at its left edge
		return -1, 0
	case 180: // points left → text to the left, anchored at its right edge
		return +1, 0
	case 90: // points up → text above, anchored at its bottom edge
		return 0, +1
	default: // 270, points down → text below, anchored at its top edge
		return 0, -1
	}
}

func writeText(w *sexprWriter, sh *schema.Sheet, t *schema.Text) {
	angle, h, v := convert.TextPositioning(t.Just, t.Rot)
	w.open("text", q(t.Content))
	w.line(fmt.Sprintf("(at %s %s %d)", f(w.kx(t.Pos.X)), f(w.ky(t.Pos.Y)), angle))
	w.line(fmt.Sprintf("(effects (font %s)%s)", fontSize(sh.FontHeight(t.Font)), justifyClause(h, v)))
	w.writeUUID(makeUUID(fmt.Sprintf("txt:%d:%d:%s", t.Pos.X, t.Pos.Y, t.Content)))
	w.close()
}

// fontSize formats a KiCad (size H H) clause from an em height in nanometres.
func fontSize(h schema.Length) string {
	v := mm(h)
	return fmt.Sprintf("(size %s %s)", f(v), f(v))
}

// ---------- Symbol instance ----------

func writeSymbolInstance(w *sexprWriter, sh *schema.Sheet, comp *schema.Component, sym *schema.Symbol, projName, rootUUID string) {
	libID := symLibID(sym)
	x := w.kx(comp.Position.X)
	y := w.ky(comp.Position.Y)

	w.open("symbol")
	w.line(fmt.Sprintf("(lib_id %s)", q(libID)))
	// The lib_symbol is emitted in KiCad's Y-up library frame (see slx/sly), so
	// KiCad's own instance rotation reproduces the Altium orientation directly:
	// the placement angle is just the schema rotation (CCW), normalized to 0–359.
	kicadRot := ((int(comp.Rotation) % 360) + 360) % 360
	w.line(fmt.Sprintf("(at %s %s %d)", f(x), f(y), kicadRot))
	// Note: no (mirror …) is emitted. The mapper de-rotates each component's
	// children into a canonical local frame but does not de-mirror them, so a
	// mirrored component already bakes its reflected geometry into its own
	// (geometry-hashed, hence distinct) lib_symbol. Re-rotation alone then
	// reproduces the true absolute pin positions; emitting a KiCad mirror on
	// top would double-mirror the part.
	//
	// Each converted lib_symbol is a self-contained, single-unit definition:
	// every pin lands in the localName_1_1 sub-symbol and the body graphics in
	// localName_0_1. The original Altium multi-unit split (CURRENTPARTID) is
	// flattened away — each placed part becomes its own geometry-hashed symbol.
	// So the instance must always reference unit 1; emitting comp.Unit (e.g. 2)
	// makes KiCad/Kicanvas look for a non-existent localName_2_1 sub-symbol and
	// silently drop the pins (and their wire stubs).
	const instUnit = 1
	w.line(fmt.Sprintf("(unit %d)", instUnit))
	w.line("(in_bom yes)")
	w.line("(on_board yes)")
	w.line("(dnp no)")
	w.writeUUID(makeUUID(fmt.Sprintf("comp:%s:%d", comp.Designator, comp.Prov.Record)))

	// Field text angle/justification are absolute in the KiCad file (KiCad does
	// not re-rotate property text by the symbol's placement angle). Altium also
	// stores field orientations in absolute schematic space, so the field's own
	// orientation maps straight through convert.TextPositioning — no counter-
	// rotation by the component rotation is required.
	dx, dy := w.localToKicad(comp, comp.DesignatorPos)
	fieldPropAt(w, "Reference", comp.Designator, dx, dy, comp.DesignatorJust, comp.DesignatorRot, comp.Rotation, sh.FontHeight(comp.DesignatorFont), comp.Designator == "")

	valField := comp.ValueField()
	val, valFont := sym.LibRef, schema.FontRef(0)
	vx, vy := x+2.54, y+2.54 // fallback when no value field
	valJust := schema.JustifyBottomLeft
	valRot := schema.Angle(0)
	valHide := false
	if valField != nil {
		val, valFont = valField.Value, valField.Font
		vx, vy = w.localToKicad(comp, valField.Pos)
		valJust, valRot = valField.Just, valField.Rot
		valHide = !valField.Visible
	}
	fieldPropAt(w, "Value", val, vx, vy, valJust, valRot, comp.Rotation, sh.FontHeight(valFont), valHide)

	// Remaining component parameters become KiCad properties, honouring each
	// field's Altium visibility. The value/comment field is already emitted
	// above; skip it and any unnamed or reserved fields. KiCad requires unique
	// property names, so de-duplicate.
	seen := map[string]bool{"Reference": true, "Value": true}
	for i := range comp.Fields {
		fld := &comp.Fields[i]
		if fld == valField || fld.Name == "" {
			continue
		}
		name := fld.Name
		if seen[name] {
			continue
		}
		seen[name] = true
		fx, fy := w.localToKicad(comp, fld.Pos)
		fieldPropAt(w, name, fld.Value, fx, fy, fld.Just, fld.Rot, comp.Rotation, sh.FontHeight(fld.Font), !fld.Visible)
	}

	// Per-pin instance entries. KiCad (and Kicanvas) draw a symbol's pins — and
	// their connecting stubs — from the (pin …) children of the *instance*, not
	// from the lib_symbol directly: each instance pin is matched by number to the
	// lib_symbol definition to obtain its geometry. Omitting these makes the pins
	// (and stubs) silently invisible even though the lib_symbol defines them.
	for _, p := range sym.Pins {
		w.open("pin", q(p.Number))
		w.writeUUID(makeUUID(fmt.Sprintf("comppin:%s:%d:%s", comp.Designator, comp.Prov.Record, p.Number)))
		w.close()
	}

	writeInstances(w, projName, rootUUID, comp.Designator, instUnit)

	w.close()
}

// ---------- Property helpers ----------

func writeProp(w *sexprWriter, name, value string, x, y float64, hide bool) {
	w.open("property", q(name), q(value))
	w.line(fmt.Sprintf("(at %s %s 0)", f(x), f(y)))
	eff := "(font (size 1.27 1.27))"
	if hide {
		w.line(fmt.Sprintf("(effects %s (hide))", eff))
	} else {
		w.line(fmt.Sprintf("(effects %s)", eff))
	}
	w.close()
}

// writePropAt emits a symbol property. angle is the absolute text angle (0 or
// 90 — KiCad never stores 180°/270° for fields); hjust/vjust are KiCad-signed
// alignment values (see justifyClause).
func writePropAt(w *sexprWriter, name, value string, x, y float64, angle, hjust, vjust int, h schema.Length, hide bool) {
	w.open("property", q(name), q(value))
	w.line(fmt.Sprintf("(at %s %s %d)", f(x), f(y), angle))
	eff := "(font " + fontSize(h) + ")" + justifyClause(hjust, vjust)
	if hide {
		w.line(fmt.Sprintf("(effects %s (hide))", eff))
	} else {
		w.line(fmt.Sprintf("(effects %s)", eff))
	}
	w.close()
}

// fieldPropAt computes the KiCad angle/justification for a symbol-instance text
// field and emits it as a property. It first derives the absolute appearance
// from the Altium justification + orientation, then compensates for the symbol
// instance rotation (KiCad re-rotates field text by the placement angle).
func fieldPropAt(w *sexprWriter, name, value string, x, y float64, just schema.Justify, orient, instRot schema.Angle, h schema.Length, hide bool) {
	angle, hj, vj := convert.TextPositioning(just, orient)
	angle, hj, vj = convert.CompensateFieldForInstanceRotation(angle, hj, vj, int(instRot))
	writePropAt(w, name, value, x, y, angle, hj, vj, h, hide)
}

func writeStroke(w *sexprWriter, s schema.Stroke) {
	wMM := mm(s.Width)
	if wMM < 0.001 {
		wMM = 0
	}
	w.line(fmt.Sprintf("(stroke (width %s) (type default))", f(wMM)))
}

func writeFill(w *sexprWriter, fill *schema.Color) {
	if fill == nil {
		w.line("(fill (type none))")
	} else {
		w.line("(fill (type background))")
	}
}

func writePaper(w *sexprWriter, p schema.Paper) {
	if p.Std == schema.PaperCustom && p.Custom != nil {
		w.b.WriteString(w.indent() + fmt.Sprintf("(paper \"User\" %s %s)\n",
			f(float64(p.Custom.W)/1e6), f(float64(p.Custom.H)/1e6)))
		return
	}
	w.attr("paper", q(stdPaperName(p.Std)))
}

func stdPaperName(std schema.PaperStd) string {
	names := map[schema.PaperStd]string{
		schema.PaperA4:     "A4",
		schema.PaperA3:     "A3",
		schema.PaperA2:     "A2",
		schema.PaperA1:     "A1",
		schema.PaperA0:     "A0",
		schema.PaperA:      "A",
		schema.PaperB:      "B",
		schema.PaperC:      "C",
		schema.PaperD:      "D",
		schema.PaperE:      "E",
		schema.PaperLetter: "USLetter",
		schema.PaperLegal:  "USLegal",
	}
	if n, ok := names[std]; ok {
		return n
	}
	return "A4"
}

// ---------- Coordinate and formatting ----------

func mm(nm schema.Length) float64 { return float64(nm) / 1e6 }

// kx/ky map schema coordinates (Y-up) to the KiCad *sheet* frame (Y-down).
// Use these only for sheet-level geometry: instance origins, wires, junctions,
// labels, sheet graphics.
//
// The Y flip is taken about the page height (w.pageH) rather than about 0, so
// that schema Y=0 (bottom of the Altium sheet) maps to the bottom of the KiCad
// page and the content lands inside KiCad's page frame. Flipping about 0 instead
// would place all content at negative Y — one sheet-height above the frame.
func (w *sexprWriter) kx(x schema.Length) float64 { return mm(x) }
func (w *sexprWriter) ky(y schema.Length) float64 { return w.pageH - mm(y) }

// reRotatePoint applies a component rotation (degrees CCW, multiples of 90) to
// a local-frame point, reversing the de-rotation done by the mapper. This
// converts a stored local-frame label position back to a component-relative
// vector that, when added to the component anchor, gives absolute sheet coords.
func reRotatePoint(p schema.Point, rot schema.Angle) schema.Point {
	switch int(rot+0.5) % 360 {
	case 90:
		return schema.Point{X: -p.Y, Y: p.X}
	case 180:
		return schema.Point{X: -p.X, Y: -p.Y}
	case 270:
		return schema.Point{X: p.Y, Y: -p.X}
	default:
		return p
	}
}

// localToKicad converts a component-local-frame label position to absolute
// KiCad sheet coordinates by re-applying the component rotation and adding the
// anchor, then mapping through kx/ky.
func (w *sexprWriter) localToKicad(comp *schema.Component, local schema.Point) (float64, float64) {
	rel := reRotatePoint(local, comp.Rotation)
	abs := schema.Point{X: comp.Position.X + rel.X, Y: comp.Position.Y + rel.Y}
	return w.kx(abs.X), w.ky(abs.Y)
}

// slx/sly map schema-local coordinates to the KiCad *library symbol* frame.
// KiCad's symbol-library frame is Y-up, exactly like our schema-local frame, so
// no Y flip is applied here. KiCad applies the lib→sheet Y flip and the instance
// rotation itself when it places the symbol. Use these for everything emitted
// inside a lib_symbol: pins and body graphics.
func slx(x schema.Length) float64 { return mm(x) }
func sly(y schema.Length) float64 { return mm(y) }

func deg2rad(d float64) float64 { return d * math.Pi / 180 }

func f(v float64) string {
	s := fmt.Sprintf("%.4f", v)
	if strings.Contains(s, ".") {
		s = strings.TrimRight(s, "0")
		s = strings.TrimRight(s, ".")
	}
	if s == "-0" {
		return "0"
	}
	return s
}

func ptsStr(pts []schema.Point) string {
	var b strings.Builder
	for _, p := range pts {
		fmt.Fprintf(&b, "(xy %s %s) ", f(slx(p.X)), f(sly(p.Y)))
	}
	return strings.TrimSpace(b.String())
}

// pinConnectionEnd returns the wire-connection end of the pin in schema coords.
// p.Position is the body-attachment end; the connection end is offset by
// PinLength in the pin's outward direction.
func pinConnectionEnd(p *schema.Pin) (schema.Length, schema.Length) {
	x, y, l := p.Position.X, p.Position.Y, p.PinLength
	switch p.Orientation {
	case schema.DirRight:
		return x + l, y
	case schema.DirLeft:
		return x - l, y
	case schema.DirUp:
		return x, y + l
	case schema.DirDown:
		return x, y - l
	}
	return x, y
}

// ---------- Pin rotation table ----------

// pinRotTable maps a schema (Y-up) pin direction to the KiCad library pin
// angle. The angle is the direction the pin stub extends FROM the connection
// end TOWARD the symbol body, measured CCW from +X in the Y-up library frame.
var pinRotTable = [4]int{
	schema.DirRight: 180, // stub goes left  (−X) from connection toward body
	schema.DirUp:    270, // stub goes down  (−Y) from connection toward body
	schema.DirLeft:  0,   // stub goes right (+X) from connection toward body
	schema.DirDown:  90,  // stub goes up    (+Y) from connection toward body
}

func pinRotation(d schema.Dir4) int {
	if int(d) < len(pinRotTable) {
		return pinRotTable[d]
	}
	return 0
}

// ---------- Pin electrical type ----------

var pinElecNames = [8]string{
	schema.PinInput:         "input",
	schema.PinBidi:          "bidirectional",
	schema.PinOutput:        "output",
	schema.PinOpenCollector: "open_collector",
	schema.PinPassive:       "passive",
	schema.PinHiZ:           "tri_state",
	schema.PinOpenEmitter:   "open_emitter",
	schema.PinPower:         "power_in",
}

func pinElecType(t schema.PinType) string {
	if int(t) < len(pinElecNames) {
		return pinElecNames[t]
	}
	return "passive"
}

// ---------- UUID generation ----------

// makeUUID produces a deterministic RFC-4122-shaped UUID from the input string
// using the first 16 bytes of its SHA-256 hash.
func makeUUID(s string) string {
	h := sha256.Sum256([]byte(s))
	// Set version 4 and variant bits.
	h[6] = (h[6] & 0x0f) | 0x40
	h[8] = (h[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		h[0:4], h[4:6], h[6:8], h[8:10], h[10:16])
}

func sheetUUID(name string) string { return makeUUID("sheet:" + name) }

// ---------- Misc ----------

var nonAlnum = regexp.MustCompile(`[^A-Za-z0-9_]`)

func sanitizeName(s string) string {
	return nonAlnum.ReplaceAllString(strings.TrimSpace(s), "_")
}

func q(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}

// ---------- S-expression writer ----------

type sexprWriter struct {
	b     strings.Builder
	depth int
	pageH float64 // KiCad page height in mm, for the sheet-frame Y flip
}

func (w *sexprWriter) indent() string { return strings.Repeat("\t", w.depth) }

func (w *sexprWriter) open(name string, args ...string) {
	w.b.WriteString(w.indent() + "(" + name)
	for _, a := range args {
		w.b.WriteString(" " + a)
	}
	w.b.WriteString("\n")
	w.depth++
}

func (w *sexprWriter) close() {
	w.depth--
	w.b.WriteString(w.indent() + ")\n")
}

func (w *sexprWriter) attr(name, value string) {
	w.b.WriteString(w.indent() + "(" + name + " " + value + ")\n")
}

func (w *sexprWriter) line(s string) {
	w.b.WriteString(w.indent() + s + "\n")
}

func (w *sexprWriter) writeUUID(u string) {
	w.b.WriteString(w.indent() + `(uuid "` + u + `")` + "\n")
}

func (w *sexprWriter) String() string { return w.b.String() }
