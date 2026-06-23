// Package pcbmapper converts a RawBoard from the pcbreader into a pcbschema.Board.
package pcbmapper

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/rveen/golib/formats/altium/altium/pcbreader"
	"github.com/rveen/golib/formats/altium/emit"
	"github.com/rveen/golib/formats/altium/pcbschema"
	"github.com/rveen/golib/formats/altium/schema"
)

// noNet is the Altium sentinel value for "unconnected" (0xFFFF).
const noNet = uint16(0xFFFF)

// polygonBoard marks a track/arc as belonging to the board outline, not a copper pour.
const polygonBoard = uint16(0xFFFE)

// altiumKeepoutLayer is Altium's KEEP_OUT_LAYER; tracks here are rule areas.
const altiumKeepoutLayer = uint8(56)

// altiumNetUnconnected is the NET property value meaning "no net" in text records.
const altiumNetUnconnected = 65535

// rawToNm converts a native PCB coordinate (0.1 µin = 2.54 nm) to nanometres.
func rawToNm(v int32) schema.Length {
	return schema.Length(v) * 254 / 100
}

// Map converts a RawBoard to a pcbschema.Board.
func Map(rb *pcbreader.RawBoard, sourceFile string) (*pcbschema.Board, *emit.Report, error) {
	rep := &emit.Report{}
	b := &pcbschema.Board{
		Meta: pcbschema.Meta{SourceFile: sourceFile},
	}

	buildNets(rb, b)
	buildComponents(rb, b, rep)
	buildTracks(rb, b)
	buildVias(rb, b)
	buildPads(rb, b)
	buildArcs(rb, b)
	buildFills(rb, b)
	buildTexts(rb, b)
	buildZones(rb, b, rep)
	buildZoneFills(rb, b)
	buildPolys(rb, b)
	buildCustomPads(rb, b)
	buildKeepouts(rb, b)
	buildBoardOutline(rb, b)
	buildThickness(rb, b)
	buildLayers(b)

	return b, rep, nil
}

// ---------- Nets ----------

func buildNets(rb *pcbreader.RawBoard, b *pcbschema.Board) {
	for i, r := range rb.NetRecs {
		b.Nets = append(b.Nets, &pcbschema.Net{
			Index: i,
			Name:  r.Str("NAME"),
		})
	}
}

// ---------- Components ----------

func buildComponents(rb *pcbreader.RawBoard, b *pcbschema.Board, rep *emit.Report) {
	for i, r := range rb.ComponentRecs {
		// Coordinates stored as "3475.0945mil" in X/Y keys.
		x := parseMilStr(r.Str("X"))
		y := parseMilStr(r.Str("Y"))
		rot, _ := strconv.ParseFloat(r.Str("ROTATION"), 64)
		layer := uint8(1)
		if strings.EqualFold(r.Str("LAYER"), "BOTTOM") {
			layer = 32
		}
		// Prefer SOURCEDESIGNATOR when DESIGNATOR is empty.
		desig := r.Str("DESIGNATOR")
		if desig == "" {
			desig = r.Str("SOURCEDESIGNATOR")
		}
		_ = rep
		b.Components = append(b.Components, &pcbschema.Component{
			Index:      i,
			Designator: desig,
			Pattern:    r.Str("PATTERN"),
			Layer:      layer,
			Position:   schema.Point{X: x, Y: y},
			Rotation:   rot,
			Prov:       schema.Provenance{Sheet: sourceOf(rb), Record: i, Kind: "component"},
		})
	}
}

// milsToNm converts mils + fractional 1/10000-mil to nm.
// Components6 text records store coordinates in whole mils.
func milsToNm(mils, frac int) schema.Length {
	return schema.Length((int64(mils)*10000+int64(frac))*254) / 100
}

func sourceOf(rb *pcbreader.RawBoard) string {
	if len(rb.BoardProps) > 0 {
		return rb.BoardProps[0].Str("FILENAME")
	}
	return ""
}

// ---------- Tracks ----------

func buildTracks(rb *pcbreader.RawBoard, b *pcbschema.Board) {
	for i, t := range rb.Tracks {
		// Skip pour-fill tracks (polygon != noNet) but keep board-outline tracks (polygonBoard).
		if t.Polygon != noNet && t.Polygon != polygonBoard {
			continue
		}
		// Keepout-layer (Altium 56) tracks become rule-area zones, not graphics.
		if t.Layer == altiumKeepoutLayer {
			continue
		}
		b.Tracks = append(b.Tracks, &pcbschema.Track{
			Layer:     t.Layer,
			Net:       t.Net,
			Component: t.Component,
			Start:     schema.Point{X: rawToNm(t.StartX), Y: rawToNm(t.StartY)},
			End:       schema.Point{X: rawToNm(t.EndX), Y: rawToNm(t.EndY)},
			Width:     rawToNm(t.Width),
			Prov:      schema.Provenance{Record: i, Kind: "track"},
		})
	}
}

// ---------- Vias ----------

func buildVias(rb *pcbreader.RawBoard, b *pcbschema.Board) {
	for i, v := range rb.Vias {
		b.Vias = append(b.Vias, &pcbschema.Via{
			Net:        v.Net,
			Position:   schema.Point{X: rawToNm(v.PosX), Y: rawToNm(v.PosY)},
			Diameter:   rawToNm(v.Diameter),
			HoleSize:   rawToNm(v.HoleSize),
			StartLayer: v.StartLayer,
			EndLayer:   v.EndLayer,
			Prov:       schema.Provenance{Record: i, Kind: "via"},
		})
	}
}

// ---------- Pads ----------

func buildPads(rb *pcbreader.RawBoard, b *pcbschema.Board) {
	for i, p := range rb.Pads {
		b.Pads = append(b.Pads, &pcbschema.Pad{
			Designator:   p.Designator,
			Layer:        p.Layer,
			Net:          p.Net,
			Component:    p.Component,
			Position:     schema.Point{X: rawToNm(p.PosX), Y: rawToNm(p.PosY)},
			TopSize:      schema.Size{W: rawToNm(p.TopSizeX), H: rawToNm(p.TopSizeY)},
			MidSize:      schema.Size{W: rawToNm(p.MidSizeX), H: rawToNm(p.MidSizeY)},
			BotSize:      schema.Size{W: rawToNm(p.BotSizeX), H: rawToNm(p.BotSizeY)},
			HoleSize:     rawToNm(p.HoleSize),
			TopShape:     pcbschema.PadShape(p.TopShape),
			BotShape:     pcbschema.PadShape(p.BotShape),
			Rotation:     p.Rotation,
			Plated:       p.Plated,
			AltShape:     pcbschema.PadShape(p.AltShape),
			CornerRadius: p.CornerRadius,
			Prov:         schema.Provenance{Record: i, Kind: "pad"},
		})
	}
}

// ---------- Arcs ----------

func buildArcs(rb *pcbreader.RawBoard, b *pcbschema.Board) {
	for i, a := range rb.Arcs {
		// Skip pour-fill arcs (polygon != noNet) but keep board-outline arcs (polygonBoard).
		if a.Polygon != noNet && a.Polygon != polygonBoard {
			continue
		}
		b.Arcs = append(b.Arcs, &pcbschema.Arc{
			Layer:      a.Layer,
			Net:        a.Net,
			Component:  a.Component,
			Center:     schema.Point{X: rawToNm(a.CenterX), Y: rawToNm(a.CenterY)},
			Radius:     rawToNm(a.Radius),
			StartAngle: a.StartAngle,
			EndAngle:   a.EndAngle,
			Width:      rawToNm(a.Width),
			Prov:       schema.Provenance{Record: i, Kind: "arc"},
		})
	}
}

// ---------- Fills ----------

func buildFills(rb *pcbreader.RawBoard, b *pcbschema.Board) {
	for i, f := range rb.Fills {
		b.Fills = append(b.Fills, &pcbschema.Fill{
			Layer:     f.Layer,
			Net:       f.Net,
			Component: f.Component,
			Pos1:      schema.Point{X: rawToNm(f.Pos1X), Y: rawToNm(f.Pos1Y)},
			Pos2:      schema.Point{X: rawToNm(f.Pos2X), Y: rawToNm(f.Pos2Y)},
			Rotation:  f.Rotation,
			Prov:      schema.Provenance{Record: i, Kind: "fill"},
		})
	}
}

// ---------- Texts ----------

// altiumTextTrueType is ALTIUM_TEXT_TYPE::TRUETYPE.
const altiumTextTrueType = 1

func buildTexts(rb *pcbreader.RawBoard, b *pcbschema.Board) {
	for i, t := range rb.Texts {
		height := rawToNm(int32(t.Height))
		strokeW := rawToNm(int32(t.StrokeWidth))
		// Altium TrueType glyph height overshoots the cap height; KiCad scales it
		// down to fit. KiCad would use 0.63 for a genuinely-loaded Arial face, but
		// since Arial is not bundled it substitutes a metric-compatible font and
		// applies 0.5 — which is what the reference output uses, so we do the same.
		// Stroke width is scaled by the same factor to preserve proportions and keep
		// designators inside their component boxes.
		if t.FontType == altiumTextTrueType {
			const factor = 0.5
			height = schema.Length(float64(height) * factor)
			strokeW = schema.Length(float64(strokeW) * factor)
		}
		b.Texts = append(b.Texts, &pcbschema.PcbText{
			Layer:        t.Layer,
			Component:    t.Component,
			Position:     schema.Point{X: rawToNm(t.PosX), Y: rawToNm(t.PosY)},
			Height:       height,
			StrokeWidth:  strokeW,
			Rotation:     t.Rotation,
			Mirrored:     t.Mirrored,
			IsComment:    t.IsComment,
			IsDesignator: t.IsDesignator,
			Text:         t.Text,
			Prov:         schema.Provenance{Record: i, Kind: "text"},
		})
	}
}

// ---------- Zones (Polygons6 text records) ----------

func buildZones(rb *pcbreader.RawBoard, b *pcbschema.Board, rep *emit.Report) {
	for i, r := range rb.PolygonRecs {
		layerStr := r.Str("LAYER")
		layerName := altiumLayerNameToKicad(layerStr)

		// NET is a 0-based net index in Polygons6; altiumNetUnconnected (65535) = no net.
		netRaw := r.IntDef("NET", altiumNetUnconnected)
		netIdx := -1
		netName := ""
		if netRaw != altiumNetUnconnected {
			netIdx = netRaw
			if netIdx >= 0 && netIdx < len(b.Nets) {
				netName = b.Nets[netIdx].Name
			}
		}

		pourIndex := r.IntDef("POURINDEX", 0)
		hatchStyle := r.Str("HATCHSTYLE")
		gridSize := parseMilStr(r.Str("GRIDSIZE"))
		trackWidth := parseMilStr(r.Str("TRACKWIDTH"))

		// For non-solid hatch styles, compute the hatch gap: gridSize - trackWidth.
		// For "None" (no copper) use a very large gap so fill is effectively empty.
		var hatchGap schema.Length
		switch hatchStyle {
		case "None":
			hatchGap = 100 * 25400 // 100 mil — effectively no copper
		case "45Degree", "90Degree", "Horizontal", "Vertical":
			hatchGap = gridSize - trackWidth
			if hatchGap <= 0 {
				hatchGap = trackWidth
			}
		}

		// Vertices are stored as VX0="1050.5137mil" VY0="..." until keys are absent.
		var verts []schema.Point
		for j := 0; ; j++ {
			xStr := r.Str(fmt.Sprintf("VX%d", j))
			yStr := r.Str(fmt.Sprintf("VY%d", j))
			if xStr == "" || yStr == "" {
				break
			}
			x := parseMilStr(xStr)
			y := parseMilStr(yStr)
			verts = append(verts, schema.Point{X: x, Y: y})
		}
		if len(verts) == 0 {
			continue
		}
		_ = rep
		b.Zones = append(b.Zones, &pcbschema.Zone{
			Layer:      layerName,
			Net:        netIdx,
			NetName:    netName,
			Vertices:   verts,
			Priority:   pourIndex,
			HatchStyle: hatchStyle,
			HatchGap:   hatchGap,
			TrackWidth: trackWidth,
			Prov:       schema.Provenance{Record: i, Kind: "zone"},
		})
	}
}

// parseMilStr parses strings like "1050.5137mil" and returns nm.
func parseMilStr(s string) schema.Length {
	s = strings.TrimSuffix(s, "mil")
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0
	}
	// mils → nm: 1 mil = 25400 nm
	return schema.Length(f * 25400)
}

// altiumLayerNameToKicad converts Board6/Polygons6 text layer names to KiCad names.
// Examples: "TOP" → "F.Cu", "BOTTOM" → "B.Cu", "MID1" → "In1.Cu", "MID2" → "In2.Cu"
func altiumLayerNameToKicad(s string) string {
	switch strings.ToUpper(s) {
	case "TOP":
		return "F.Cu"
	case "BOTTOM":
		return "B.Cu"
	}
	upper := strings.ToUpper(s)
	if strings.HasPrefix(upper, "MID") {
		n, err := strconv.Atoi(s[3:])
		if err == nil && n >= 1 {
			return fmt.Sprintf("In%d.Cu", n)
		}
	}
	// Fallback: use the name as-is (will likely not match a KiCad layer)
	return s
}

// ---------- Board outline (from Regions) ----------

// boardOutlineLayers is the set of Altium layer IDs used for board outlines.
// Mechanical layer 1 (id 57) is the conventional board-outline layer.
// Layer 56 (keepout) and layer 74 (multi-layer) sometimes carry it too.
var boardOutlineLayers = map[uint8]bool{
	57: true, // Mechanical 1
}

func buildBoardOutline(rb *pcbreader.RawBoard, b *pcbschema.Board) {
	for i, reg := range rb.Regions {
		// Regions6 is a duplicate of ShapeBasedRegions6; use the shape-based copy
		// (and BoardRegions) for outline geometry so edges are not doubled.
		if reg.Storage == "Regions6" {
			continue
		}
		if !boardOutlineLayers[reg.Layer] && !reg.IsBoardCutout {
			continue
		}
		verts := reg.Vertices
		if len(verts) < 2 {
			continue
		}
		// Convert vertex list to sequential track segments on Edge.Cuts.
		for j := 0; j < len(verts); j++ {
			next := (j + 1) % len(verts)
			sx := rawToNm(verts[j][0])
			sy := rawToNm(verts[j][1])
			ex := rawToNm(verts[next][0])
			ey := rawToNm(verts[next][1])
			b.BoardOutline = append(b.BoardOutline, &pcbschema.Track{
				Layer:     57, // Edge.Cuts in emitter
				Net:       noNet,
				Component: noNet,
				Start:     schema.Point{X: sx, Y: sy},
				End:       schema.Point{X: ex, Y: ey},
				Width:     0,
				Prov:      schema.Provenance{Record: i, Kind: "board_outline"},
			})
		}
	}
}

// ---------- Zone fills (pre-computed copper from Regions6 / ShapeBasedRegions6) ----------

func buildZoneFills(rb *pcbreader.RawBoard, b *pcbschema.Board) {
	for _, reg := range rb.Regions {
		// Zone (polygon-pour) fills come exclusively from Regions6; the KiCad
		// importer uses ShapeBasedRegions6 (a duplicate) only for board shapes,
		// pads and rule areas. Reading both would double every fill.
		if reg.Storage != "Regions6" {
			continue
		}
		// Only board-level copper fill regions linked to a polygon pour.
		// Keepout and board-cutout regions are not copper fill.
		if reg.IsKeepout || reg.IsBoardCutout {
			continue
		}
		if reg.Polygon == noNet || reg.Polygon == polygonBoard || reg.Component != noNet {
			continue
		}
		idx := int(reg.Polygon)
		if idx >= len(b.Zones) {
			continue
		}
		verts := make([]schema.Point, 0, len(reg.Vertices))
		for _, v := range reg.Vertices {
			verts = append(verts, schema.Point{X: rawToNm(v[0]), Y: rawToNm(v[1])})
		}
		if len(verts) < 3 {
			continue
		}
		holes := make([][]schema.Point, 0, len(reg.Holes))
		for _, h := range reg.Holes {
			hv := make([]schema.Point, 0, len(h))
			for _, v := range h {
				hv = append(hv, schema.Point{X: rawToNm(v[0]), Y: rawToNm(v[1])})
			}
			if len(hv) >= 3 {
				holes = append(holes, hv)
			}
		}
		b.Zones[idx].Fills = append(b.Zones[idx].Fills, pcbschema.ZoneFill{Vertices: verts, Holes: holes})
	}
}

// ---------- Graphic polygons (non-pour regions) ----------

// buildPolys maps regions that are not copper pours, board cutouts, or keepouts
// into graphic polygons (gr_poly / fp_poly). These are filled shapes on
// silkscreen, fabrication, mechanical, or copper layers that do not belong to a
// polygon pour.
func buildPolys(rb *pcbreader.RawBoard, b *pcbschema.Board) {
	for i, reg := range rb.Regions {
		// Graphic polygons come from ShapeBasedRegions6 only (Regions6 is a
		// duplicate reserved for zone fills).
		if reg.Storage != "ShapeBasedRegions6" {
			continue
		}
		if reg.IsKeepout || reg.IsBoardCutout {
			continue
		}
		// Component-owned copper regions become custom pads, not graphic polys.
		if reg.Component != noNet && (reg.Layer == 1 || reg.Layer == 32) {
			continue
		}
		// Pour-fill regions (linked to a polygon) are handled by buildZoneFills.
		if reg.Polygon != noNet && reg.Polygon != polygonBoard {
			continue
		}
		// Board-outline layers are handled by buildBoardOutline.
		if boardOutlineLayers[reg.Layer] {
			continue
		}
		if len(reg.Vertices) < 3 {
			continue
		}
		verts := make([]schema.Point, 0, len(reg.Vertices))
		for _, v := range reg.Vertices {
			verts = append(verts, schema.Point{X: rawToNm(v[0]), Y: rawToNm(v[1])})
		}
		b.Polys = append(b.Polys, &pcbschema.Poly{
			Layer:     reg.Layer,
			Component: reg.Component,
			Vertices:  verts,
			Width:     0,
			Filled:    true,
			Prov:      schema.Provenance{Record: i, Kind: "poly"},
		})
	}
}

// ---------- Custom pads (component-owned copper regions) ----------

// buildCustomPads converts component-owned copper regions from ShapeBasedRegions6
// into custom pads, mirroring KiCad's own Altium importer
// (ConvertShapeBasedRegions6ToFootprintItemOnLayer). Each such region becomes a
// custom pad: a tiny circle anchor at the first vertex plus a filled polygon
// primitive carrying the (possibly arc-segmented) outline.
func buildCustomPads(rb *pcbreader.RawBoard, b *pcbschema.Board) {
	for i, reg := range rb.Regions {
		if reg.Storage != "ShapeBasedRegions6" {
			continue
		}
		if reg.IsKeepout || reg.IsBoardCutout {
			continue
		}
		if reg.Component == noNet || (reg.Layer != 1 && reg.Layer != 32) {
			continue
		}
		// Pour fills are handled as zones, not pads.
		if reg.Polygon != noNet && reg.Polygon != polygonBoard {
			continue
		}
		outline := regionOutline(reg)
		if len(outline) < 3 {
			continue
		}
		b.CustomPads = append(b.CustomPads, &pcbschema.CustomPad{
			Component: reg.Component,
			Net:       reg.Net,
			Layer:     reg.Layer,
			Anchor:    schema.Point{X: rawToNm(reg.Vertices[0][0]), Y: rawToNm(reg.Vertices[0][1])},
			Outline:   outline,
			Prov:      schema.Provenance{Record: i, Kind: "custom_pad"},
		})
	}
}

// regionOutline converts an extended region's vertex/arc list into a sequence of
// custom-pad outline entries, dropping the trailing closing vertex and folding
// arc-endpoint vertices into the preceding arc (matching KiCad's chain builder).
func regionOutline(reg pcbreader.RawRegion) []pcbschema.PadOutlineEntry {
	n := len(reg.Vertices)
	if n == 0 {
		return nil
	}
	// Extended outlines repeat the first vertex at the end to close the loop.
	if n > 1 && reg.Vertices[n-1] == reg.Vertices[0] {
		n--
	}
	pt := func(idx int) schema.Point {
		v := reg.Vertices[idx%n]
		return schema.Point{X: rawToNm(v[0]), Y: rawToNm(v[1])}
	}
	var out []pcbschema.PadOutlineEntry
	var haveLast bool
	var last schema.Point
	hasArc := len(reg.VertexArcs) == len(reg.Vertices)
	for i := 0; i < n; i++ {
		if hasArc && reg.VertexArcs[i].IsArc {
			a := reg.VertexArcs[i]
			start := pt(i)
			end := pt(i + 1)
			cx, cy := float64(rawToNm(a.CX)), float64(rawToNm(a.CY))
			r := float64(rawToNm(a.Radius))
			sweep := math.Mod(a.EndAngle-a.StartAngle, 360)
			if sweep < 0 {
				sweep += 360
			}
			midAng := (a.StartAngle + sweep/2) * math.Pi / 180
			mid := schema.Point{
				X: schema.Length(math.Round(cx + r*math.Cos(midAng))),
				Y: schema.Length(math.Round(cy + r*math.Sin(midAng))),
			}
			out = append(out, pcbschema.PadOutlineEntry{IsArc: true, Pt: start, Mid: mid, End: end})
			last, haveLast = end, true
		} else {
			p := pt(i)
			// Skip a straight vertex coincident with the previous arc endpoint.
			if haveLast && p == last {
				continue
			}
			out = append(out, pcbschema.PadOutlineEntry{Pt: p})
			last, haveLast = p, true
		}
	}
	return out
}

// ---------- Keepout rule areas ----------

// buildKeepouts converts tracks on the Altium keepout layer into rule-area zones,
// mirroring KiCad's importer (HelperPcpShapeAsBoardKeepoutRegion): each track's
// stroke is expanded into a rounded-end stadium polygon.
func buildKeepouts(rb *pcbreader.RawBoard, b *pcbschema.Board) {
	for i, t := range rb.Tracks {
		if t.Layer != altiumKeepoutLayer {
			continue
		}
		s := schema.Point{X: rawToNm(t.StartX), Y: rawToNm(t.StartY)}
		e := schema.Point{X: rawToNm(t.EndX), Y: rawToNm(t.EndY)}
		half := float64(rawToNm(t.Width)) / 2
		outline := stadiumOutline(s, e, half)
		if len(outline) < 3 {
			continue
		}
		b.Keepouts = append(b.Keepouts, &pcbschema.Keepout{
			Outline: outline,
			Prov:    schema.Provenance{Record: i, Kind: "keepout"},
		})
	}
}

// stadiumOutline expands a line segment of half-width h into a closed polygon
// with rounded ends approximated by three points per cap (45°/90°/135°), matching
// the point layout produced by KiCad's TransformShapeToPolygon.
func stadiumOutline(s, e schema.Point, h float64) []schema.Point {
	dx := float64(e.X - s.X)
	dy := float64(e.Y - s.Y)
	length := math.Hypot(dx, dy)
	if length == 0 || h <= 0 {
		return nil
	}
	ux, uy := dx/length, dy/length // unit direction s->e
	px, py := -uy, ux              // left normal
	const c = 0.70710678118        // cos45 = sin45
	mk := func(base schema.Point, ax, ay float64) schema.Point {
		return schema.Point{X: base.X + schema.Length(math.Round(ax)), Y: base.Y + schema.Length(math.Round(ay))}
	}
	return []schema.Point{
		// End cap (outward = +u): -45, tip, +45.
		mk(e, h*(ux*c-px*c), h*(uy*c-py*c)),
		mk(e, h*ux, h*uy),
		mk(e, h*(ux*c+px*c), h*(uy*c+py*c)),
		mk(e, h*px, h*py), // end top corner
		mk(s, h*px, h*py), // start top corner
		// Start cap (outward = -u): +45, tip, -45.
		mk(s, h*(-ux*c+px*c), h*(-uy*c+py*c)),
		mk(s, -h*ux, -h*uy),
		mk(s, h*(-ux*c-px*c), h*(-uy*c-py*c)),
		mk(s, -h*px, -h*py), // start bottom corner
		mk(e, -h*px, -h*py), // end bottom corner
	}
}

// ---------- Board thickness ----------

func buildThickness(rb *pcbreader.RawBoard, b *pcbschema.Board) {
	if len(rb.BoardProps) == 0 {
		b.Meta.Thickness = 1600000 // 1.6 mm default
		return
	}
	// BOARDTHICKNESS is stored in native units (0.1 µin = 2.54 nm) in Board6.
	// Default 629921 ≈ 1.6 mm (1 mil = 10000 native units; 62.992 mil = 629921 units).
	thick := rb.BoardProps[0].IntDef("BOARDTHICKNESS", 629921)
	b.Meta.Thickness = rawToNm(int32(thick))
}

// ---------- Layer table ----------

func buildLayers(b *pcbschema.Board) {
	seen := map[uint8]bool{}
	add := func(id uint8) {
		if seen[id] {
			return
		}
		seen[id] = true
		num, name, typ := kicadLayerFromAltium(id)
		if num < 0 {
			return
		}
		b.Layers = append(b.Layers, &pcbschema.Layer{
			AltiumID:  int(id),
			KiCadID:   num,
			KiCadName: name,
			Type:      typ,
		})
	}
	for _, t := range b.Tracks {
		add(t.Layer)
	}
	for _, a := range b.Arcs {
		add(a.Layer)
	}
	for _, p := range b.Pads {
		add(p.Layer)
	}
	for _, f := range b.Fills {
		add(f.Layer)
	}
	for _, t := range b.Texts {
		add(t.Layer)
	}
	// Always include the standard copper + mask + silk layers.
	for _, id := range []uint8{1, 32, 33, 34, 35, 36, 37, 38} {
		add(id)
	}

	// Scan zone layer names for inner copper layers (e.g. "In1.Cu").
	// These are not in the binary records so need to be added separately.
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
	// Ensure all inner copper layers from 1 to maxInner are present (no gaps).
	// Altium inner copper ID = KiCad ID + 1.
	for i := 1; i <= maxInner; i++ {
		altID := uint8(i + 1) // Altium layer ID for InN.Cu = N+1
		add(altID)
	}
}

// kicadLayerFromAltium maps an Altium v6 layer byte to (KiCad num, name, type).
// KiCad layer numbering: 0=F.Cu, 1-30=In1..In30.Cu, 31=B.Cu,
// 32=B.Adhes, 33=F.Adhes, 34=B.Paste, 35=F.Paste, 36=B.SilkS, 37=F.SilkS,
// 38=B.Mask, 39=F.Mask, 40=Dwgs.User, 41=Cmts.User, 42=Eco1.User, 43=Eco2.User,
// 44=Edge.Cuts, 45=Margin, 46=B.CrtYd, 47=F.CrtYd, 48=B.Fab, 49=F.Fab,
// 50=User.1 .. 63=User.14.
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
		n := int(a) - 56 // Mech 1..9 → n=1..9
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
