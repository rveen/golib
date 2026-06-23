// Package kicadpcb emits KiCad 7+ S-expression PCB files (.kicad_pcb) from a
// pcbschema.Board.
//
// Y-axis: Altium Y increases upward; KiCad PCB Y increases downward.
// Conversion: ky(y) = -mm(y). No page-relative offset needed (PCB uses
// absolute coordinates).
//
// All coordinates are in mm (KiCad native); conversion: nm / 1e6.
package kicadpcb

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/rveen/golib/formats/altium/emit"
	"github.com/rveen/golib/formats/altium/pcbschema"
	"github.com/rveen/golib/formats/altium/schema"
)

const version = "20221018"

// noNet is the Altium sentinel for unconnected.
const noNet = uint16(0xFFFF)

// Emitter produces a .kicad_pcb artifact from a pcbschema.Board.
type Emitter struct{}

func (Emitter) Name() string { return "kicadpcb" }

// Emit converts board to a single .kicad_pcb artifact.
func (Emitter) Emit(b *pcbschema.Board, _ any) ([]emit.Artifact, *emit.Report, error) {
	rep := &emit.Report{}
	data := renderBoard(b, rep)
	name := "board.kicad_pcb"
	if b.Meta.SourceFile != "" {
		base := b.Meta.SourceFile
		if i := strings.LastIndexAny(base, "/\\"); i >= 0 {
			base = base[i+1:]
		}
		if ext := strings.LastIndex(base, "."); ext >= 0 {
			base = base[:ext]
		}
		name = base + ".kicad_pcb"
	}
	return []emit.Artifact{{Name: name, Data: []byte(data)}}, rep, nil
}

// ---------- Board renderer ----------

// A4 page dimensions in mm (KiCad default), used for board centering.
const (
	pageWidthMM  = 297.0
	pageHeightMM = 210.0
)

// setCenterOffset computes the global page-centering offset (offX, offY) so the
// board's edge bounding box is centered on the sheet, matching KiCad's importer
// (altium_pcb.cpp: "center board"). Coordinates are evaluated in offset-free
// KiCad space (x = mm(x), y = -mm(y)).
func setCenterOffset(b *pcbschema.Board) {
	offX, offY = 0, 0
	if len(b.BoardOutline) == 0 {
		return
	}
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	acc := func(x, y schema.Length) {
		fx, fy := mm(x), -mm(y)
		minX, maxX = math.Min(minX, fx), math.Max(maxX, fx)
		minY, maxY = math.Min(minY, fy), math.Max(maxY, fy)
	}
	for _, seg := range b.BoardOutline {
		acc(seg.Start.X, seg.Start.Y)
		acc(seg.End.X, seg.End.Y)
	}
	bbw, bbh := maxX-minX, maxY-minY
	offX = (pageWidthMM-bbw)/2 - minX
	offY = (pageHeightMM-bbh)/2 - minY
}

func renderBoard(b *pcbschema.Board, rep *emit.Report) string {
	setCenterOffset(b)
	w := &sexprWriter{}
	w.open("kicad_pcb")
	w.attr("version", version)
	w.attr("generator", q("pcbconv"))

	w.open("general")
	w.attr("thickness", f4(mm(b.Meta.Thickness)))
	w.close()

	w.attr("paper", q("A4"))

	// Layer table
	writeLayerTable(w, b)

	// Net declarations
	w.line(`(net 0 "")`)
	for _, n := range b.Nets {
		w.line(fmt.Sprintf("(net %d %s)", n.Index+1, q(n.Name)))
	}

	// Build per-component indices for footprint output.
	padsByComp := make(map[int][]*pcbschema.Pad)
	for _, p := range b.Pads {
		if p.Component != noNet {
			padsByComp[int(p.Component)] = append(padsByComp[int(p.Component)], p)
		}
	}
	textsByComp := make(map[int][]*pcbschema.PcbText)
	for _, t := range b.Texts {
		if t.Component != noNet {
			textsByComp[int(t.Component)] = append(textsByComp[int(t.Component)], t)
		}
	}
	arcsByComp := make(map[int][]*pcbschema.Arc)
	for _, a := range b.Arcs {
		if a.Component != noNet {
			arcsByComp[int(a.Component)] = append(arcsByComp[int(a.Component)], a)
		}
	}
	tracksByComp := make(map[int][]*pcbschema.Track)
	for _, t := range b.Tracks {
		if t.Component != noNet {
			tracksByComp[int(t.Component)] = append(tracksByComp[int(t.Component)], t)
		}
	}
	fillsByComp := make(map[int][]*pcbschema.Fill)
	for _, f := range b.Fills {
		if f.Component != noNet {
			fillsByComp[int(f.Component)] = append(fillsByComp[int(f.Component)], f)
		}
	}
	polysByComp := make(map[int][]*pcbschema.Poly)
	for _, p := range b.Polys {
		if p.Component != noNet {
			polysByComp[int(p.Component)] = append(polysByComp[int(p.Component)], p)
		}
	}
	customPadsByComp := make(map[int][]*pcbschema.CustomPad)
	for _, p := range b.CustomPads {
		if p.Component != noNet {
			customPadsByComp[int(p.Component)] = append(customPadsByComp[int(p.Component)], p)
		}
	}

	// Footprints
	for _, comp := range b.Components {
		writeFootprint(w, comp, footprintItems{
			pads:       padsByComp[comp.Index],
			texts:      textsByComp[comp.Index],
			arcs:       arcsByComp[comp.Index],
			tracks:     tracksByComp[comp.Index],
			fills:      fillsByComp[comp.Index],
			polys:      polysByComp[comp.Index],
			customPads: customPadsByComp[comp.Index],
		}, b.Nets)
	}

	// Free pads (no owning component) become their own empty footprints.
	for _, p := range b.Pads {
		if p.Component == noNet {
			writeFreePadFootprint(w, p, b.Nets)
		}
	}

	// Board-level tracks: copper → segment, non-copper → gr_line.
	for _, t := range b.Tracks {
		if t.Component != noNet {
			continue
		}
		if _, _, typ := kicadLayerFromAltium(t.Layer); typ == "signal" {
			writeSegment(w, t, b.Nets)
		} else {
			writeGrLineTrack(w, t)
		}
	}

	// Vias
	for _, v := range b.Vias {
		writeVia(w, v, b.Nets)
	}

	// Board-level arcs
	for _, a := range b.Arcs {
		if a.Component == noNet {
			writeArc(w, a, b.Nets)
		}
	}

	// Board-level fills (as gr_rect)
	for _, f := range b.Fills {
		if f.Component == noNet {
			writeFill(w, f)
		}
	}

	// Board-level graphic polygons (non-pour regions on graphic layers)
	for _, p := range b.Polys {
		if p.Component == noNet {
			writeGrPoly(w, p)
		}
	}

	// Board-level texts
	for _, t := range b.Texts {
		if t.Component == noNet {
			writeGrText(w, t)
		}
	}

	// Board outline
	for _, seg := range b.BoardOutline {
		writeGrLine(w, seg)
	}

	// Copper zones
	for _, z := range b.Zones {
		writeZone(w, z, b.Nets)
	}

	// Keepout rule areas (from keepout-layer tracks)
	if len(b.Keepouts) > 0 {
		layers := copperLayerNames(b)
		for _, k := range b.Keepouts {
			writeKeepout(w, k, layers)
		}
	}

	_ = rep

	w.close()
	return w.String()
}

// ---------- Layer table ----------

// standardNonCopperLayers is the complete KiCad non-copper layer set (IDs 32-58).
// KiCad requires these to be declared even if unused. Layer aliases are omitted
// since they are optional and vary by project.
var standardNonCopperLayers = []struct {
	id   int
	name string
}{
	{32, "B.Adhes"},
	{33, "F.Adhes"},
	{34, "B.Paste"},
	{35, "F.Paste"},
	{36, "B.SilkS"},
	{37, "F.SilkS"},
	{38, "B.Mask"},
	{39, "F.Mask"},
	{40, "Dwgs.User"},
	{41, "Cmts.User"},
	{42, "Eco1.User"},
	{43, "Eco2.User"},
	{44, "Edge.Cuts"},
	{45, "Margin"},
	{46, "B.CrtYd"},
	{47, "F.CrtYd"},
	{48, "B.Fab"},
	{49, "F.Fab"},
	{50, "User.1"},
	{51, "User.2"},
	{52, "User.3"},
	{53, "User.4"},
	{54, "User.5"},
	{55, "User.6"},
	{56, "User.7"},
	{57, "User.8"},
	{58, "User.9"},
}

func writeLayerTable(w *sexprWriter, b *pcbschema.Board) {
	// Determine the maximum inner copper layer used (KiCad IDs 1–30).
	maxInner := 0
	for _, l := range b.Layers {
		if l.KiCadID >= 1 && l.KiCadID <= 30 && l.KiCadID > maxInner {
			maxInner = l.KiCadID
		}
	}
	// Also check zone layer names for inner copper not in binary records.
	for _, z := range b.Zones {
		var n int
		if _, err := fmt.Sscanf(z.Layer, "In%d.Cu", &n); err == nil && n > maxInner {
			maxInner = n
		}
	}

	w.open("layers")
	// F.Cu
	w.line(fmt.Sprintf("(0 %s signal)", q("F.Cu")))
	// Inner copper: all layers 1..maxInner (no gaps).
	for i := 1; i <= maxInner; i++ {
		w.line(fmt.Sprintf("(%d %s signal)", i, q(fmt.Sprintf("In%d.Cu", i))))
	}
	// B.Cu
	w.line(fmt.Sprintf("(31 %s signal)", q("B.Cu")))
	// Standard non-copper layers.
	for _, l := range standardNonCopperLayers {
		w.line(fmt.Sprintf("(%d %s user)", l.id, q(l.name)))
	}
	w.close()
}

// ---------- Net helpers ----------

func kicadNet(altNet uint16) int {
	if altNet == noNet {
		return 0
	}
	return int(altNet) + 1
}

func netName(altNet uint16, nets []*pcbschema.Net) string {
	if altNet == noNet || int(altNet) >= len(nets) {
		return ""
	}
	return nets[altNet].Name
}

// ---------- Footprint ----------

func writeFpText(w *sexprWriter, role, text string, t *pcbschema.PcbText, comp *pcbschema.Component, layer string, hide bool, posOverride *[2]float64) {
	var rx, ry float64
	rot := comp.Rotation
	if t != nil {
		// Position is absolute; make relative to component and un-rotate.
		rx, ry = fpLocal(comp, t.Position.X, t.Position.Y)
		rot = t.Rotation
		if t.Layer != 0 {
			if _, lname, _ := kicadLayerFromAltium(t.Layer); lname != "" {
				layer = lname
			}
		}
	}
	// A caller-supplied position (used to centre the designator on the component
	// box) overrides Altium's stored text offset. With KiCad's default centred
	// justification this places the text centred on that point.
	if posOverride != nil {
		rx, ry = posOverride[0], posOverride[1]
	}
	h := 1.0
	sw := 0.15
	if t != nil && t.Height > 0 {
		h = mm(t.Height)
		sw = mm(t.StrokeWidth)
	}
	hideStr := ""
	if hide {
		hideStr = " hide"
	}
	w.open("fp_text", role, q(text),
		fmt.Sprintf("(at %s %s %s)", f4(rx), f4(ry), f4(rot)),
		fmt.Sprintf("(layer %s)%s", q(layer), hideStr),
	)
	w.line(fmt.Sprintf("(effects (font (size %s %s) (thickness %s)))", f4(h), f4(h), f4(sw)))
	w.close()
}

// footprintBoxCenter returns the centre of the component's body box in local
// (footprint) coordinates. It uses the silkscreen / graphic outline (tracks and
// arcs) when present — that is the visible component rectangle — and otherwise
// falls back to the pad bounding box. ok is false when there is nothing to bound.
func footprintBoxCenter(comp *pcbschema.Component, items footprintItems) (cx, cy float64, ok bool) {
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	acc := func(x, y float64) {
		minX, maxX = math.Min(minX, x), math.Max(maxX, x)
		minY, maxY = math.Min(minY, y), math.Max(maxY, y)
	}
	for _, t := range items.tracks {
		sx, sy := fpLocal(comp, t.Start.X, t.Start.Y)
		ex, ey := fpLocal(comp, t.End.X, t.End.Y)
		acc(sx, sy)
		acc(ex, ey)
	}
	for _, a := range items.arcs {
		ax, ay := fpLocal(comp, a.Center.X, a.Center.Y)
		r := mm(a.Radius)
		acc(ax-r, ay-r)
		acc(ax+r, ay+r)
	}
	if math.IsInf(minX, 1) {
		for _, p := range items.pads {
			px, py := fpLocal(comp, p.Position.X, p.Position.Y)
			acc(px, py)
		}
	}
	if math.IsInf(minX, 1) {
		return 0, 0, false
	}
	return (minX + maxX) / 2, (minY + maxY) / 2, true
}

// footprintItems bundles the per-component primitives that go inside a footprint.
type footprintItems struct {
	pads       []*pcbschema.Pad
	texts      []*pcbschema.PcbText
	arcs       []*pcbschema.Arc
	tracks     []*pcbschema.Track
	fills      []*pcbschema.Fill
	polys      []*pcbschema.Poly
	customPads []*pcbschema.CustomPad
}

func writeFootprint(w *sexprWriter, comp *pcbschema.Component, items footprintItems, nets []*pcbschema.Net) {
	layerName := "F.Cu"
	if comp.Layer == 32 {
		layerName = "B.Cu"
	}
	w.open("footprint", q(comp.Pattern), fmt.Sprintf("(layer %s)", q(layerName)),
		fmt.Sprintf("(at %s %s %s)", f4(kx(comp.Position.X)), f4(ky(comp.Position.Y)), f4(comp.Rotation)))

	// Reference and value as fp_text elements.
	silkLayer := "F.SilkS"
	if comp.Layer == 32 {
		silkLayer = "B.SilkS"
	}
	ref := comp.Designator
	val := ""
	var refText, valText *pcbschema.PcbText
	for _, t := range items.texts {
		if t.IsDesignator {
			ref = t.Text
			refText = t
		}
		if t.IsComment {
			val = t.Text
			valText = t
		}
	}
	// Centre the reference designator on the component body box.
	var refPos *[2]float64
	if cx, cy, ok := footprintBoxCenter(comp, items); ok {
		refPos = &[2]float64{cx, cy}
	}
	writeFpText(w, "reference", ref, refText, comp, silkLayer, false, refPos)
	writeFpText(w, "value", val, valText, comp, silkLayer, true, nil)

	// Pads
	for _, p := range items.pads {
		writePadInFootprint(w, p, comp, nets)
	}
	// Custom pads (component-owned copper regions)
	for _, p := range items.customPads {
		writeCustomPadInFootprint(w, p, comp, nets)
	}
	// Graphical lines (silkscreen, courtyard, fabrication outlines)
	for _, t := range items.tracks {
		writeLineInFootprint(w, t, comp)
	}
	// Graphical arcs / circles
	for _, a := range items.arcs {
		writeArcInFootprint(w, a, comp)
	}
	// Graphical polygons (fills and non-pour regions)
	for _, f := range items.fills {
		writeFillInFootprint(w, f, comp)
	}
	for _, p := range items.polys {
		writePolyInFootprint(w, p, comp)
	}

	w.close()
}

// holePadAttrs applies KiCad's NPTH rules to a through-hole pad. Unplated holes
// become np_thru_hole with a blank designator, their copper size clamped up to at
// least the drill diameter (KiCad forbids size < drill), on the F&B copper +
// all-mask layer set, and with no net. Plated holes keep their name/size/net on
// the all-copper layer set.
func holePadAttrs(p *pcbschema.Pad, sz schema.Size) (padType, desig string, w, h schema.Length, layers string, hasNet bool) {
	if p.Plated {
		return "thru_hole", p.Designator, sz.W, sz.H, ` "*.Cu" "*.Mask"`, true
	}
	w, h = sz.W, sz.H
	if w < p.HoleSize {
		w = p.HoleSize
	}
	if h < p.HoleSize {
		h = p.HoleSize
	}
	return "np_thru_hole", "", w, h, ` "F&B.Cu" "*.Mask"`, false
}

func writePadInFootprint(w *sexprWriter, p *pcbschema.Pad, comp *pcbschema.Component, nets []*pcbschema.Net) {
	shape, shapeExtra := padShape(p)

	// Pad position is relative to the component (un-rotated local frame) in
	// KiCad; our stored position is absolute. The pad angle stays absolute,
	// matching KiCad's own importer.
	relX, relY := fpLocal(comp, p.Position.X, p.Position.Y)

	sz := p.TopSize
	if sz.W == 0 && sz.H == 0 {
		sz = p.BotSize
	}

	if p.HoleSize > 0 {
		padType, desig, sw, sh, layers, hasNet := holePadAttrs(p, sz)
		netStr := ""
		if hasNet {
			netStr = fmt.Sprintf(" (net %d %s)", kicadNet(p.Net), q(netName(p.Net, nets)))
		}
		w.line(fmt.Sprintf(`(pad %s %s %s (at %s %s %s) (size %s %s) (drill %s) (layers%s)%s%s)`,
			q(desig), padType, shape,
			f4(relX), f4(relY), f4(p.Rotation),
			f4(mm(sw)), f4(mm(sh)),
			f4(mm(p.HoleSize)),
			layers, netStr, shapeExtra,
		))
		return
	}
	w.line(fmt.Sprintf(`(pad %s smd %s (at %s %s %s) (size %s %s) (layers%s) (net %d %s)%s)`,
		q(p.Designator), shape,
		f4(relX), f4(relY), f4(p.Rotation),
		f4(mm(sz.W)), f4(mm(sz.H)),
		padLayers(p, comp.Layer),
		kicadNet(p.Net), q(netName(p.Net, nets)), shapeExtra,
	))
}

// writeCustomPadInFootprint emits a component-owned copper region as a KiCad
// custom pad: a tiny circle anchor plus a filled polygon primitive. The polygon
// preserves arc segments so rounded copper shapes render faithfully. This mirrors
// KiCad's own importer (ConvertShapeBasedRegions6ToFootprintItemOnLayer).
func writeCustomPadInFootprint(w *sexprWriter, p *pcbschema.CustomPad, comp *pcbschema.Component, nets []*pcbschema.Net) {
	layers := ` "F.Cu" "F.Paste" "F.Mask"`
	if p.Layer == 32 {
		layers = ` "B.Cu" "B.Paste" "B.Mask"`
	}
	ax, ay := fpLocal(comp, p.Anchor.X, p.Anchor.Y)
	// prim converts an absolute outline point to pad-primitive coordinates
	// (footprint-local, relative to the pad anchor).
	prim := func(pt schema.Point) (float64, float64) {
		x, y := fpLocal(comp, pt.X, pt.Y)
		return x - ax, y - ay
	}

	netStr := ""
	if p.Net != noNet && int(p.Net) < len(nets) {
		netStr = fmt.Sprintf(" (net %d %s)", kicadNet(p.Net), q(nets[p.Net].Name))
	}
	w.open("pad", q(""), "smd", "custom",
		fmt.Sprintf("(at %s %s)", f4(ax), f4(ay)),
		"(size 0.000001 0.000001)",
		fmt.Sprintf("(layers%s)%s", layers, netStr),
	)
	w.line("(options (clearance outline) (anchor circle))")
	w.open("primitives")
	w.open("gr_poly")
	w.open("pts")
	for _, e := range p.Outline {
		if e.IsArc {
			sx, sy := prim(e.Pt)
			mx, my := prim(e.Mid)
			ex, ey := prim(e.End)
			w.line(fmt.Sprintf("(arc (start %s %s) (mid %s %s) (end %s %s))",
				f4(sx), f4(sy), f4(mx), f4(my), f4(ex), f4(ey)))
		} else {
			x, y := prim(e.Pt)
			w.line(fmt.Sprintf("(xy %s %s)", f4(x), f4(y)))
		}
	}
	w.close() // pts
	w.line("(width 0) (fill yes)")
	w.close() // gr_poly
	w.close() // primitives
	w.close() // pad
}

func padShapeName(s pcbschema.PadShape) string {
	switch s {
	case pcbschema.PadShapeCircle:
		return "circle"
	case pcbschema.PadShapeRect:
		return "rect"
	case pcbschema.PadShapeOctagonal:
		return "oval"
	default:
		return "circle"
	}
}

// padShape resolves a pad's KiCad shape name and any extra shape attribute
// (e.g. roundrect_rratio), mirroring KiCad's Altium importer: a CIRCLE pad whose
// padstack alt-shape is ROUNDRECT becomes a roundrect with ratio CornerRadius/200;
// otherwise a CIRCLE with unequal width/height becomes an oval.
func padShape(p *pcbschema.Pad) (name, extra string) {
	switch p.TopShape {
	case pcbschema.PadShapeRect:
		return "rect", ""
	case pcbschema.PadShapeCircle:
		if p.AltShape == pcbschema.PadShapeRounded {
			return "roundrect", fmt.Sprintf(" (roundrect_rratio %s)", f4(float64(p.CornerRadius)/200))
		}
		if p.TopSize.W != p.TopSize.H {
			return "oval", ""
		}
		return "circle", ""
	default:
		return padShapeName(p.TopShape), ""
	}
}

func padLayers(p *pcbschema.Pad, compLayer uint8) string {
	if p.HoleSize > 0 {
		return ` "*.Cu" "*.Mask"`
	}
	if compLayer == 32 { // bottom
		return ` "B.Cu" "B.Paste" "B.Mask"`
	}
	return ` "F.Cu" "F.Paste" "F.Mask"`
}

func writeArcInFootprint(w *sexprWriter, a *pcbschema.Arc, comp *pcbschema.Component) {
	_, layerName, _ := kicadLayerFromAltium(a.Layer)
	// Arc coordinates relative to component position, Y-flipped.
	// Relative to the component origin — no global page offset (see fpLocal).
	relCX := mm(a.Center.X - comp.Position.X)
	relCY := -mm(a.Center.Y - comp.Position.Y)
	r := mm(a.Radius)
	// A full-circle "arc" (Altium start=0, end=360) must be a circle, not an arc:
	// a degenerate arc with start==end crashes KiCad. (cf. altium_pcb.cpp:3203)
	if isFullCircle(a) {
		ccx, ccy := fpRot(comp, relCX, relCY)
		ecx, ecy := fpRot(comp, relCX, relCY-r)
		w.line(fmt.Sprintf("(fp_circle (center %s %s) (end %s %s) (stroke (width %s) (type solid)) (fill none) (layer %s))",
			f4(ccx), f4(ccy), f4(ecx), f4(ecy),
			f4(mm(a.Width)), q(layerName),
		))
		return
	}
	startRad := -deg2rad(a.StartAngle)
	endRad := -deg2rad(a.EndAngle)
	midRad := (startRad + endRad) / 2
	sx, sy := fpRot(comp, relCX+r*math.Cos(startRad), relCY+r*math.Sin(startRad))
	mx, my := fpRot(comp, relCX+r*math.Cos(midRad), relCY+r*math.Sin(midRad))
	ex, ey := fpRot(comp, relCX+r*math.Cos(endRad), relCY+r*math.Sin(endRad))
	w.line(fmt.Sprintf("(fp_arc (start %s %s) (mid %s %s) (end %s %s) (width %s) (layer %s))",
		f4(sx), f4(sy), f4(mx), f4(my), f4(ex), f4(ey),
		f4(mm(a.Width)), q(layerName),
	))
}

// fpRot rotates a component-relative (Y-flipped, mm) coordinate into the
// footprint's local frame. KiCad re-applies the footprint orientation on load
// using R(θ)=[cosθ, sinθ; -sinθ, cosθ] (clockwise in the Y-down file frame), so
// children must be stored pre-rotated by the inverse of that, which is the
// standard CCW matrix below. Without this, every child of a rotated footprint
// lands at the wrong absolute position.
func fpRot(comp *pcbschema.Component, rx, ry float64) (float64, float64) {
	if comp.Rotation == 0 {
		return rx, ry
	}
	th := deg2rad(comp.Rotation)
	c, s := math.Cos(th), math.Sin(th)
	return c*rx - s*ry, s*rx + c*ry
}

// fpLocal converts an absolute board coordinate to a footprint-local KiCad
// coordinate (relative to the component origin, Y-flipped, and un-rotated by the
// footprint orientation). This matches the convention KiCad's own importer uses
// for pads, text and graphics inside footprints.
func fpLocal(comp *pcbschema.Component, x, y schema.Length) (float64, float64) {
	// Footprint-local coordinates are relative to the component origin, so the
	// global page offset must not be applied here (use mm directly, not kx/ky).
	return fpRot(comp, mm(x-comp.Position.X), -mm(y-comp.Position.Y))
}

// writeLineInFootprint emits a component-owned track as an fp_line (silk/fab outline).
func writeLineInFootprint(w *sexprWriter, t *pcbschema.Track, comp *pcbschema.Component) {
	_, layerName, _ := kicadLayerFromAltium(t.Layer)
	if layerName == "" {
		return
	}
	sx, sy := fpLocal(comp, t.Start.X, t.Start.Y)
	ex, ey := fpLocal(comp, t.End.X, t.End.Y)
	w.line(fmt.Sprintf("(fp_line (start %s %s) (end %s %s) (stroke (width %s) (type solid)) (layer %s))",
		f4(sx), f4(sy), f4(ex), f4(ey),
		f4(mm(t.Width)), q(layerName),
	))
}

// writeFillInFootprint emits a component-owned rectangular fill as an fp_poly.
func writeFillInFootprint(w *sexprWriter, f *pcbschema.Fill, comp *pcbschema.Component) {
	_, layerName, _ := kicadLayerFromAltium(f.Layer)
	if layerName == "" {
		return
	}
	// Rotate each corner as a full point: a rotated rectangle's corners are not
	// the cross-product of independently rotated x/y extents.
	ax, ay := fpLocal(comp, f.Pos1.X, f.Pos1.Y)
	bx, by := fpLocal(comp, f.Pos2.X, f.Pos1.Y)
	cx, cy := fpLocal(comp, f.Pos2.X, f.Pos2.Y)
	dx, dy := fpLocal(comp, f.Pos1.X, f.Pos2.Y)
	w.open("fp_poly")
	w.line(fmt.Sprintf("(pts (xy %s %s) (xy %s %s) (xy %s %s) (xy %s %s))",
		f4(ax), f4(ay), f4(bx), f4(by), f4(cx), f4(cy), f4(dx), f4(dy)))
	w.line(fmt.Sprintf("(stroke (width 0) (type solid)) (fill solid) (layer %s)", q(layerName)))
	w.close()
}

// writePolyInFootprint emits a component-owned graphic polygon as an fp_poly.
func writePolyInFootprint(w *sexprWriter, p *pcbschema.Poly, comp *pcbschema.Component) {
	_, layerName, _ := kicadLayerFromAltium(p.Layer)
	if layerName == "" || len(p.Vertices) < 3 {
		return
	}
	fillStr := "none"
	if p.Filled {
		fillStr = "solid"
	}
	var b strings.Builder
	for _, v := range p.Vertices {
		vx, vy := fpLocal(comp, v.X, v.Y)
		fmt.Fprintf(&b, " (xy %s %s)", f4(vx), f4(vy))
	}
	w.open("fp_poly")
	w.line("(pts" + b.String() + ")")
	w.line(fmt.Sprintf("(stroke (width %s) (type solid)) (fill %s) (layer %s)", f4(mm(p.Width)), fillStr, q(layerName)))
	w.close()
}

// ---------- Segment ----------

func writeSegment(w *sexprWriter, t *pcbschema.Track, nets []*pcbschema.Net) {
	_, layerName, _ := kicadLayerFromAltium(t.Layer)
	netNum := kicadNet(t.Net)
	w.line(fmt.Sprintf("(segment (start %s %s) (end %s %s) (width %s) (layer %s) (net %d))",
		f4(kx(t.Start.X)), f4(ky(t.Start.Y)),
		f4(kx(t.End.X)), f4(ky(t.End.Y)),
		f4(mm(t.Width)),
		q(layerName),
		netNum,
	))
}

// ---------- Via ----------

func writeVia(w *sexprWriter, v *pcbschema.Via, nets []*pcbschema.Net) {
	_, startName, _ := kicadLayerFromAltium(v.StartLayer)
	_, endName, _ := kicadLayerFromAltium(v.EndLayer)
	netNum := kicadNet(v.Net)
	w.line(fmt.Sprintf("(via (at %s %s) (size %s) (drill %s) (layers %s %s) (net %d))",
		f4(kx(v.Position.X)), f4(ky(v.Position.Y)),
		f4(mm(v.Diameter)),
		f4(mm(v.HoleSize)),
		q(startName), q(endName),
		netNum,
	))
}

// ---------- Arc ----------

// writeArc emits a (segment/arc ...) for a board-level arc.
// KiCad 7+ arc format: (arc (start X Y) (mid X Y) (end X Y) ...).
func writeArc(w *sexprWriter, a *pcbschema.Arc, nets []*pcbschema.Net) {
	_, layerName, _ := kicadLayerFromAltium(a.Layer)
	netNum := kicadNet(a.Net)
	// A full-circle "arc" (Altium start=0, end=360) must be a circle, not an arc:
	// a degenerate arc with start==end crashes KiCad. (cf. altium_pcb.cpp:3203)
	if isFullCircle(a) {
		cx := kx(a.Center.X)
		cy := ky(a.Center.Y)
		r := mm(a.Radius)
		w.line(fmt.Sprintf("(gr_circle (center %s %s) (end %s %s) (stroke (width %s) (type solid)) (fill none) (layer %s))",
			f4(cx), f4(cy), f4(cx), f4(cy-r),
			f4(mm(a.Width)), q(layerName),
		))
		return
	}
	sx, sy, mx, my, ex, ey := arcPoints(a)
	w.line(fmt.Sprintf("(arc (start %s %s) (mid %s %s) (end %s %s) (width %s) (layer %s) (net %d))",
		f4(sx), f4(sy),
		f4(mx), f4(my),
		f4(ex), f4(ey),
		f4(mm(a.Width)),
		q(layerName),
		netNum,
	))
}

// isFullCircle reports whether an arc spans a full 360° (Altium stores these as
// startangle=0, endangle=360). Such arcs must be emitted as circles; a degenerate
// arc whose start and end points coincide crashes KiCad's geometry engine.
func isFullCircle(a *pcbschema.Arc) bool {
	return math.Abs(math.Abs(a.EndAngle-a.StartAngle)-360) < 0.01
}

// arcPoints computes start, mid, end in KiCad mm coordinates (Y-down).
func arcPoints(a *pcbschema.Arc) (sx, sy, mx, my, ex, ey float64) {
	cx := kx(a.Center.X)
	cy := ky(a.Center.Y)
	r := mm(a.Radius)

	// Altium angles: CCW in Y-up. In KiCad Y-down the arc is CW.
	// Negate angles to convert CCW-Y-up → CW-Y-down.
	startRad := -deg2rad(a.StartAngle)
	endRad := -deg2rad(a.EndAngle)
	midRad := (startRad + endRad) / 2

	sx = cx + r*math.Cos(startRad)
	sy = cy + r*math.Sin(startRad)
	mx = cx + r*math.Cos(midRad)
	my = cy + r*math.Sin(midRad)
	ex = cx + r*math.Cos(endRad)
	ey = cy + r*math.Sin(endRad)
	return
}

func deg2rad(d float64) float64 { return d * math.Pi / 180 }

// ---------- Fill ----------

func writeFill(w *sexprWriter, f *pcbschema.Fill) {
	_, layerName, _ := kicadLayerFromAltium(f.Layer)
	w.line(fmt.Sprintf("(gr_rect (start %s %s) (end %s %s) (width 0) (layer %s))",
		f4(kx(f.Pos1.X)), f4(ky(f.Pos1.Y)),
		f4(kx(f.Pos2.X)), f4(ky(f.Pos2.Y)),
		q(layerName),
	))
}

// ---------- Board-level text ----------

func writeGrText(w *sexprWriter, t *pcbschema.PcbText) {
	_, layerName, _ := kicadLayerFromAltium(t.Layer)
	w.line(fmt.Sprintf("(gr_text %s (at %s %s %s) (layer %s) (effects (font (size %s %s) (thickness %s))))",
		q(t.Text),
		f4(kx(t.Position.X)), f4(ky(t.Position.Y)), f4(t.Rotation),
		q(layerName),
		f4(mm(t.Height)), f4(mm(t.Height)),
		f4(mm(t.StrokeWidth)),
	))
}

// ---------- Board outline ----------

func writeGrLine(w *sexprWriter, t *pcbschema.Track) {
	// Edge.Cuts board outline: zero width is invalid in KiCad; use a thin default.
	width := mm(t.Width)
	if width <= 0 {
		width = 0.05
	}
	w.line(fmt.Sprintf("(gr_line (start %s %s) (end %s %s) (stroke (width %s) (type solid)) (layer %s))",
		f4(kx(t.Start.X)), f4(ky(t.Start.Y)),
		f4(kx(t.End.X)), f4(ky(t.End.Y)),
		f4(width),
		q("Edge.Cuts"),
	))
}

// writeGrLineTrack emits a board-level track on a non-copper layer as a gr_line
// (KiCad segments are only valid on copper layers).
func writeGrLineTrack(w *sexprWriter, t *pcbschema.Track) {
	_, layerName, _ := kicadLayerFromAltium(t.Layer)
	if layerName == "" {
		return
	}
	w.line(fmt.Sprintf("(gr_line (start %s %s) (end %s %s) (stroke (width %s) (type solid)) (layer %s))",
		f4(kx(t.Start.X)), f4(ky(t.Start.Y)),
		f4(kx(t.End.X)), f4(ky(t.End.Y)),
		f4(mm(t.Width)),
		q(layerName),
	))
}

// writeGrPoly emits a board-level graphic polygon as a gr_poly.
func writeGrPoly(w *sexprWriter, p *pcbschema.Poly) {
	_, layerName, _ := kicadLayerFromAltium(p.Layer)
	if layerName == "" || len(p.Vertices) < 3 {
		return
	}
	fillStr := "none"
	if p.Filled {
		fillStr = "solid"
	}
	var b strings.Builder
	for _, v := range p.Vertices {
		fmt.Fprintf(&b, " (xy %s %s)", f4(kx(v.X)), f4(ky(v.Y)))
	}
	w.open("gr_poly")
	w.line("(pts" + b.String() + ")")
	w.line(fmt.Sprintf("(stroke (width %s) (type solid)) (fill %s) (layer %s)", f4(mm(p.Width)), fillStr, q(layerName)))
	w.close()
}

// ---------- Zone ----------

func writeZone(w *sexprWriter, z *pcbschema.Zone, nets []*pcbschema.Net) {
	if len(z.Vertices) < 3 {
		return
	}
	netNum := 0
	if z.Net >= 0 {
		netNum = z.Net + 1
	}
	w.open("zone",
		fmt.Sprintf("(net %d)", netNum),
		fmt.Sprintf("(net_name %s)", q(z.NetName)),
		fmt.Sprintf("(layer %s)", q(z.Layer)),
		"(hatch edge 0.508)",
	)
	if z.Priority > 0 {
		w.attr("priority", strconv.Itoa(z.Priority))
	}
	w.open("connect_pads")
	w.attr("clearance", "0.508")
	w.close()
	w.line("(min_thickness 0.254)")

	// Emit fill mode: solid by default; hatch pattern for non-solid Altium styles.
	switch z.HatchStyle {
	case "45Degree", "90Degree", "Horizontal", "Vertical", "None":
		w.open("fill", "yes")
		w.line("(mode hatched)")
		w.attr("hatch_thickness", f4(mm(z.TrackWidth)))
		w.attr("hatch_gap", f4(mm(z.HatchGap)))
		if z.HatchStyle == "45Degree" {
			w.attr("hatch_orientation", "45")
		}
		w.close()
	default:
		w.line("(fill yes)")
	}

	// Zone boundary (from Polygons6 outline).
	w.open("polygon")
	w.line("(pts" + ptsStr(z.Vertices) + ")")
	w.close()

	// Cached fill geometry (Regions6/ShapeBasedRegions6). Altium stores each filled
	// region as an outer outline plus separate hole contours (anti-pads/thermal
	// clearances). KiCad's filled_polygon expects a single contour per island with
	// holes woven in via a zero-width slit, so we fracture each region. If any region
	// fails to fracture cleanly, we emit no cached fill for the whole zone (KiCad then
	// regenerates the fill from the zone outline on load/refill) rather than risk
	// self-intersecting geometry or copper shorts.
	contours := make([][]schema.Point, 0, len(z.Fills))
	fillsOK := true
	for _, fill := range z.Fills {
		if len(fill.Vertices) < 3 {
			continue
		}
		contour, ok := fractureFill(fill.Vertices, fill.Holes)
		if !ok {
			fillsOK = false
			break
		}
		contours = append(contours, contour)
	}
	if fillsOK {
		for _, contour := range contours {
			if len(contour) < 3 {
				continue
			}
			w.open("filled_polygon")
			w.line(fmt.Sprintf("(layer %s)", q(z.Layer)))
			w.line("(pts" + ptsStr(contour) + ")")
			w.close()
		}
	}
	w.close()
}

// copperLayerNames returns the KiCad copper layer names present on the board
// (F.Cu, In1..InN.Cu, B.Cu), used to span keepout rule areas across all copper.
func copperLayerNames(b *pcbschema.Board) []string {
	maxInner := 0
	for _, l := range b.Layers {
		if l.KiCadID >= 1 && l.KiCadID <= 30 && l.KiCadID > maxInner {
			maxInner = l.KiCadID
		}
	}
	for _, z := range b.Zones {
		var n int
		if _, err := fmt.Sscanf(z.Layer, "In%d.Cu", &n); err == nil && n > maxInner {
			maxInner = n
		}
	}
	names := []string{"F.Cu"}
	for i := 1; i <= maxInner; i++ {
		names = append(names, fmt.Sprintf("In%d.Cu", i))
	}
	return append(names, "B.Cu")
}

// writeKeepout emits a rule-area zone spanning all copper layers with copper
// items disallowed, matching KiCad's Altium keepout import.
func writeKeepout(w *sexprWriter, k *pcbschema.Keepout, copperLayers []string) {
	if len(k.Outline) < 3 {
		return
	}
	var lb strings.Builder
	for _, n := range copperLayers {
		lb.WriteString(" " + q(n))
	}
	w.open("zone",
		"(net 0)",
		"(net_name \"\")",
		fmt.Sprintf("(layers%s)", lb.String()),
		"(hatch edge 0.508)",
	)
	w.open("connect_pads")
	w.attr("clearance", "0")
	w.close()
	w.line("(min_thickness 0.254) (filled_areas_thickness no)")
	w.line("(keepout (tracks not_allowed) (vias not_allowed) (pads not_allowed) (copperpour not_allowed) (footprints allowed))")
	w.open("polygon")
	w.line("(pts" + ptsStr(k.Outline) + ")")
	w.close()
	w.close()
}

// writeFreePadFootprint emits a component-less pad as its own empty footprint,
// mirroring KiCad's handling of free Altium pads.
func writeFreePadFootprint(w *sexprWriter, p *pcbschema.Pad, nets []*pcbschema.Net) {
	layerName, silk := "F.Cu", "F.SilkS"
	if p.Layer == 32 {
		layerName, silk = "B.Cu", "B.SilkS"
	}
	w.open("footprint", q(""), fmt.Sprintf("(layer %s)", q(layerName)),
		fmt.Sprintf("(at %s %s)", f4(kx(p.Position.X)), f4(ky(p.Position.Y))))
	w.open("fp_text", "reference", q(""), "(at 0 0)", fmt.Sprintf("(layer %s)", q(silk)))
	w.line("(effects (font (size 1.2700 1.2700) (thickness 0.1500)))")
	w.close()
	w.open("fp_text", "value", q(""), "(at 0 0)", fmt.Sprintf("(layer %s)", q(silk)))
	w.line("(effects (font (size 1.2700 1.2700) (thickness 0.1500)))")
	w.close()

	sz := p.TopSize
	if sz.W == 0 && sz.H == 0 {
		sz = p.BotSize
	}
	shape, shapeExtra := padShape(p)
	if p.HoleSize > 0 {
		padType, desig, sw, sh, layers, hasNet := holePadAttrs(p, sz)
		netStr := ""
		if hasNet && p.Net != noNet && int(p.Net) < len(nets) {
			netStr = fmt.Sprintf(" (net %d %s)", kicadNet(p.Net), q(nets[p.Net].Name))
		}
		w.line(fmt.Sprintf(`(pad %s %s %s (at 0 0 %s) (size %s %s) (drill %s) (layers%s)%s%s)`,
			q(desig), padType, shape, f4(p.Rotation),
			f4(mm(sw)), f4(mm(sh)), f4(mm(p.HoleSize)), layers, netStr, shapeExtra))
	} else {
		netStr := ""
		if p.Net != noNet && int(p.Net) < len(nets) {
			netStr = fmt.Sprintf(" (net %d %s)", kicadNet(p.Net), q(nets[p.Net].Name))
		}
		w.line(fmt.Sprintf(`(pad %s smd %s (at 0 0 %s) (size %s %s) (layers%s)%s%s)`,
			q(p.Designator), shape, f4(p.Rotation),
			f4(mm(sz.W)), f4(mm(sz.H)), padLayers(p, p.Layer), netStr, shapeExtra))
	}
	w.close()
}

func ptsStr(verts []schema.Point) string {
	var b strings.Builder
	for _, v := range verts {
		b.WriteString(fmt.Sprintf(" (xy %s %s)", f4(kx(v.X)), f4(ky(v.Y))))
	}
	return b.String()
}

// ---------- Layer mapping ----------

// kicadLayerFromAltium maps an Altium v6 layer byte to (KiCad num, name, type).
// Must stay in sync with pcbmapper.kicadLayerFromAltium.
func kicadLayerFromAltium(a uint8) (num int, name, typ string) {
	switch {
	case a == 1:
		return 0, "F.Cu", "signal"
	case a >= 2 && a <= 31:
		n := int(a) - 1
		return n, fmt.Sprintf("In%d.Cu", n), "signal"
	case a == 32:
		return 31, "B.Cu", "signal"
	case a == 33:
		return 37, "F.SilkS", "user"
	case a == 34:
		return 36, "B.SilkS", "user"
	case a == 35:
		return 35, "F.Paste", "user"
	case a == 36:
		return 34, "B.Paste", "user"
	case a == 37:
		return 39, "F.Mask", "user"
	case a == 38:
		return 38, "B.Mask", "user"
	case a >= 39 && a <= 54:
		// Internal plane layers: not supported as routed copper in KiCad.
		return -1, "", ""
	case a == 56:
		return 45, "Margin", "user"
	// Mechanical 1-9 → User.1-9 (KiCad 50-58).
	case a >= 57 && a <= 65:
		n := int(a) - 56
		return 49 + n, fmt.Sprintf("User.%d", n), "user"
	// Mechanical 10-16 → KiCad convention from KiCad Altium importer.
	case a == 66:
		return 40, "Dwgs.User", "user"
	case a == 67:
		return 43, "Eco2.User", "user"
	case a == 68:
		return 49, "F.Fab", "user"
	case a == 69:
		return 48, "B.Fab", "user"
	case a == 70:
		return 41, "Cmts.User", "user"
	case a == 71:
		return 42, "Eco1.User", "user"
	case a == 72:
		return 45, "Margin", "user"
	case a == 74:
		return -1, "*.Cu", "signal"
	default:
		return 40, "Dwgs.User", "user"
	}
}

// ---------- Coordinate conversion ----------

func mm(nm schema.Length) float64 { return float64(nm) / 1e6 }

// offX/offY are the global page-centering offset (mm) added to every absolute
// coordinate, mirroring KiCad's importer which centers the board on the sheet.
// They are set per-render by centerOffset and only apply to absolute positions;
// footprint-local geometry uses mm() directly so the offset cancels.
var offX, offY float64

func kx(x schema.Length) float64 { return mm(x) + offX }
func ky(y schema.Length) float64 { return -mm(y) + offY }

// ---------- Formatting ----------

func f4(v float64) string {
	return strconv.FormatFloat(v, 'f', 4, 64)
}

func q(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", `\r`)
	return `"` + s + `"`
}

// ---------- S-expression writer ----------

type sexprWriter struct {
	b     strings.Builder
	depth int
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

func (w *sexprWriter) String() string { return w.b.String() }
