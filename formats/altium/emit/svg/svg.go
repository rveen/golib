// Package svg emits one SVG file per sheet from a schema.Schematic.
// It is the primary geometry oracle: render and visually inspect before
// trusting the KiCad output.  Uses only stdlib — no dependencies.
//
// Coordinate system: schema Y is up-positive; SVG Y is down-positive.
// We flip Y by negating it and offsetting by the sheet height so that
// the origin appears at the bottom-left of the viewport.
package svg

import (
	"fmt"
	"math"
	"strings"

	"github.com/rveen/golib/formats/altium/convert"
	"github.com/rveen/golib/formats/altium/emit"
	"github.com/rveen/golib/formats/altium/schema"
)

const nmPerPx = 25400 // 1 px = 1 mil = 25 400 nm

// fontPx is the fallback SVG font size in pixels, used for symbol pin
// name/number text (which carries no font reference). Sheet text that does
// reference the font table is sized via fontPxOf instead.
const fontPx = 50

const (
	wireColor   = "#149e14"
	wireWidthPx = 200_000.0 / nmPerPx // 0.2 mm expressed in SVG pixels
)

// Emitter implements emit.Emitter for SVG output.
type Emitter struct{}

func (Emitter) Name() string { return "svg" }

// Emit produces one SVG artifact per sheet.
func (Emitter) Emit(s *schema.Schematic, _ any) ([]emit.Artifact, *emit.Report, error) {
	rep := &emit.Report{}
	var artifacts []emit.Artifact
	for _, sh := range s.Sheets {
		name := sh.Name
		if name == "" {
			name = "sheet"
		}
		data := renderSheet(sh, s.Symbols, rep)
		artifacts = append(artifacts, emit.Artifact{
			Name: name + ".svg",
			Data: []byte(data),
		})
	}
	return artifacts, rep, nil
}

// ---------- Sheet renderer ----------

func renderSheet(sh *schema.Sheet, syms map[schema.SymbolID]*schema.Symbol, rep *emit.Report) string {
	// Determine viewport from paper size or a sensible default.
	wNm, hNm := sheetDims(sh.Paper)
	wPx := nmToPx(wNm)
	hPx := nmToPx(hNm)

	b := &builder{}
	b.writef(`<?xml version="1.0" encoding="UTF-8"?>`)
	b.writef(`<svg xmlns="http://www.w3.org/2000/svg" width="%gpx" height="%gpx" viewBox="0 0 %g %g">`,
		wPx, hPx, wPx, hPx)
	b.writef(`<rect width="%g" height="%g" fill="white"/>`, wPx, hPx)

	// Sheet template: page border and title block.
	b.renderFrame(sh, wPx, hPx)

	// Sheet-level graphics.
	for _, g := range sh.Graphics {
		b.renderGraphic(g, hPx)
	}

	// Wires.
	for _, w := range sh.Wires {
		b.renderPolyline(w.Points, fmt.Sprintf("stroke:%s;stroke-width:%g;fill:none", wireColor, wireWidthPx), hPx)
	}
	// Buses.
	for _, bus := range sh.Buses {
		b.renderPolyline(bus.Points, "stroke:navy;stroke-width:3;fill:none", hPx)
	}
	// Junctions.
	for _, j := range sh.Junctions {
		cx, cy := flipPt(j, hPx)
		b.writef(`<circle cx="%g" cy="%g" r="10" fill="%s"/>`, cx, cy, wireColor)
	}
	// Net labels.
	for _, nl := range sh.NetLabels {
		x, y := flipPt(nl.Pos, hPx)
		b.writef(`<text x="%g" y="%g" font-size="%g" fill="#333">%s</text>`,
			x, y-10, fontPxOf(sh, nl.Font), xmlEsc(nl.Text))
	}
	// Power ports.
	for _, pp := range sh.PowerPorts {
		b.renderPowerPort(sh, pp, hPx)
	}
	// Free texts.
	for _, t := range sh.Texts {
		x, y := flipPt(t.Pos, hPx)
		b.writef(`<text x="%g" y="%g" font-size="%g" fill="black">%s</text>`,
			x, y-10, fontPxOf(sh, t.Font), xmlEsc(t.Content))
	}

	// Components.
	for _, comp := range sh.Components {
		sym, ok := syms[comp.Symbol]
		if !ok {
			rep.Add(emit.Warn, comp.Prov, "symbol %s not found", comp.Symbol)
			continue
		}
		b.renderComponent(sh, comp, sym, hPx)
	}

	b.writef(`</svg>`)
	return b.String()
}

// fontPxOf resolves a font reference to an SVG font size in pixels.
func fontPxOf(sh *schema.Sheet, ref schema.FontRef) float64 {
	return nmToPx(sh.FontHeight(ref))
}

// renderPowerPort draws the graphical symbol for a power port (GND, VCC, etc.).
// Base geometry is defined with the symbol body extending rightward (+X).
// An SVG rotation of −pp.Rot maps the body to the correct visual direction.
func (b *builder) renderPowerPort(sh *schema.Sheet, pp *schema.PowerPort, hPx float64) {
	cx, cy := flipPt(pp.Pos, hPx)
	sw := wireWidthPx
	style := fmt.Sprintf(`stroke="purple" stroke-width="%.3f" fill="none"`, sw)

	// Sizes in SVG pixels (1 px = 1 mil).
	const stem = 30.0
	const b1h = 50.0
	const b2h = 35.0
	const b3h = 20.0
	const b2off = 45.0
	const b3off = 60.0
	const e1h = 60.0
	const e2h = 45.0
	const e3h = 30.0
	const e4h = 15.0
	const e2off = 45.0
	const e3off = 60.0
	const e4off = 75.0

	// Open a group: translate to connection point, then rotate body into place.
	// Schema rotation is CCW; SVG uses CW for positive angles, so negate.
	b.writef(`<g transform="translate(%g,%g) rotate(%g)" %s>`, cx, cy, -pp.Rot, style)

	switch pp.Style {
	case schema.PowerStyleGND:
		b.writef(`<line x1="0" y1="0" x2="%g" y2="0"/>`, stem)
		b.writef(`<line x1="%g" y1="%g" x2="%g" y2="%g"/>`, stem, -b1h, stem, b1h)
		b.writef(`<line x1="%g" y1="%g" x2="%g" y2="%g"/>`, b2off, -b2h, b2off, b2h)
		b.writef(`<line x1="%g" y1="%g" x2="%g" y2="%g"/>`, b3off, -b3h, b3off, b3h)
	case schema.PowerStyleEarth:
		b.writef(`<line x1="0" y1="0" x2="%g" y2="0"/>`, stem)
		b.writef(`<line x1="%g" y1="%g" x2="%g" y2="%g"/>`, stem, -e1h, stem, e1h)
		b.writef(`<line x1="%g" y1="%g" x2="%g" y2="%g"/>`, e2off, -e2h, e2off, e2h)
		b.writef(`<line x1="%g" y1="%g" x2="%g" y2="%g"/>`, e3off, -e3h, e3off, e3h)
		b.writef(`<line x1="%g" y1="%g" x2="%g" y2="%g"/>`, e4off, -e4h, e4off, e4h)
	default: // Bar, Arrow, Tee — single bar
		b.writef(`<line x1="0" y1="0" x2="%g" y2="0"/>`, stem)
		b.writef(`<line x1="%g" y1="%g" x2="%g" y2="%g"/>`, stem, -b1h, stem, b1h)
	}

	b.writef(`</g>`)

	if pp.ShowNetName {
		fp := fontPxOf(sh, pp.Font)
		b.writef(`<text x="%g" y="%g" font-size="%g" fill="purple">%s</text>`,
			cx, cy-fp, fp, xmlEsc(pp.NetName))
	}
}

// frameMarginPx is the inset of the drawing border from the page edge (200 mil).
const frameMarginPx = 200

// renderFrame draws the sheet template: an outer page border, an inner drawing
// border inset by frameMarginPx, and a simple title block in the lower-right
// corner showing the sheet name and paper size.
func (b *builder) renderFrame(sh *schema.Sheet, wPx, hPx float64) {
	const frameStyle = "stroke:black;stroke-width:5;fill:none"

	// Outer page border and inner drawing border.
	b.writef(`<rect x="0" y="0" width="%g" height="%g" style="%s"/>`, wPx, hPx, frameStyle)
	innerW := wPx - 2*frameMarginPx
	innerH := hPx - 2*frameMarginPx
	if innerW <= 0 || innerH <= 0 {
		return
	}
	b.writef(`<rect x="%g" y="%g" width="%g" height="%g" style="%s"/>`,
		float64(frameMarginPx), float64(frameMarginPx), innerW, innerH, frameStyle)

	// Title block: a box anchored to the lower-right inner corner.
	const tbW, tbH = 3500.0, 800.0
	tbX := wPx - frameMarginPx - tbW
	tbY := hPx - frameMarginPx - tbH
	if tbX < frameMarginPx || tbY < frameMarginPx {
		return // page too small for a title block
	}
	b.writef(`<rect x="%g" y="%g" width="%g" height="%g" style="%s"/>`, tbX, tbY, tbW, tbH, frameStyle)
	b.writef(`<line x1="%g" y1="%g" x2="%g" y2="%g" style="%s"/>`, tbX, tbY+tbH/2, tbX+tbW, tbY+tbH/2, frameStyle)

	title := sh.Name
	if title == "" {
		title = sh.FileName
	}
	b.writef(`<text x="%g" y="%g" font-size="120" fill="black">%s</text>`,
		tbX+80, tbY+tbH/2-40, xmlEsc(title))
	b.writef(`<text x="%g" y="%g" font-size="100" fill="black">%s</text>`,
		tbX+80, tbY+tbH-40, xmlEsc(paperLabel(sh.Paper)))
}

// paperLabel returns a short human-readable description of the paper size.
func paperLabel(p schema.Paper) string {
	d := convert.PaperDims(p)
	return fmt.Sprintf("%d x %d mil", d.W/nmPerPx, d.H/nmPerPx)
}

// renderComponent applies the component's transform and renders the symbol.
func (b *builder) renderComponent(sh *schema.Sheet, comp *schema.Component, sym *schema.Symbol, hPx float64) {
	cx, cy := flipPt(comp.Position, hPx)

	// SVG transform: translate to instance position, then rotate, then mirror.
	// Schema rotation is CCW degrees. SVG rotates CW, so negate.
	// Y flip already happened via flipPt, but rotation sense flips with Y,
	// so we use -rotation (schema CCW → SVG CW after Y-flip = same visual direction).
	var transforms []string
	transforms = append(transforms, fmt.Sprintf("translate(%g,%g)", cx, cy))
	if comp.Rotation != 0 {
		transforms = append(transforms, fmt.Sprintf("rotate(%g)", -comp.Rotation))
	}
	if comp.Mirrored {
		transforms = append(transforms, "scale(-1,1)")
	}

	b.writef(`<g transform="%s" opacity="0.9">`, strings.Join(transforms, " "))

	// Symbol graphics.
	for _, g := range sym.Graphics {
		b.renderGraphicLocal(g)
	}

	// Pins.
	for _, p := range sym.Pins {
		b.renderPin(p, comp.Rotation)
	}

	// Designator label at its stored position (component-local frame).
	// Counter-rotate around that position so the text reads horizontally,
	// matching Altium's behaviour regardless of component rotation.
	if comp.Designator != "" {
		lx, ly := localPt(comp.DesignatorPos)
		dfp := fontPxOf(sh, comp.DesignatorFont)
		b.writef(`<text x="%g" y="%g" transform="rotate(%g,%g,%g)" font-size="%g" fill="#006464">%s</text>`,
			lx, ly, comp.Rotation, lx, ly, dfp, xmlEsc(comp.Designator))
	}

	// Value/comment label at its stored position, also always horizontal.
	val, valFont := sym.LibRef, schema.FontRef(0)
	var valPos schema.Point
	if vf := comp.ValueField(); vf != nil {
		val, valFont, valPos = vf.Value, vf.Font, vf.Pos
	}
	if val != "" {
		lx, ly := localPt(valPos)
		vfp := fontPxOf(sh, valFont)
		b.writef(`<text x="%g" y="%g" transform="rotate(%g,%g,%g)" font-size="%g" fill="#7a3e00">%s</text>`,
			lx, ly, comp.Rotation, lx, ly, vfp, xmlEsc(val))
	}

	b.writef(`</g>`)
}

// renderPin draws a pin in the symbol's local frame.
// rot is the component's schema rotation (degrees CCW); it is used to
// counter-rotate text so labels are always horizontal and left-to-right.
func (b *builder) renderPin(p *schema.Pin, rot float64) {
	px := nmToPx(p.Position.X)
	py := -nmToPx(p.Position.Y) // local Y-flip
	length := nmToPx(p.PinLength)

	// p.Position is the body-attachment end (spec §9.4: "point where the pin
	// attaches to the body"). The stub extends outward to the wire-connection
	// end, so (px+dx, py+dy) is the connection point.
	// SVG Y is down-positive; Altium Y is up-positive — hence DirUp uses −dy.
	var dx, dy float64
	switch p.Orientation {
	case schema.DirRight:
		dx = length
	case schema.DirLeft:
		dx = -length
	case schema.DirUp:
		dy = -length // up in Altium = negative Y in SVG
	case schema.DirDown:
		dy = length
	}

	if length != 0 {
		b.writef(`<line x1="%g" y1="%g" x2="%g" y2="%g" stroke="%s" stroke-width="%g"/>`,
			px, py, px+dx, py+dy, wireColor, wireWidthPx)
	}

	// Text elements counter-rotate by +rot around their own anchor so that
	// the component group's rotate(-rot) is cancelled and they read normally.
	if p.NameVisible && p.Name != "" {
		// Pin name goes INSIDE the symbol body: anchor at the body-attachment
		// end and extend inward (opposite to the stub direction).
		const nameMargin = fontPx * 0.3
		var nx, ny float64
		var anchor string
		switch p.Orientation {
		case schema.DirLeft: // stub goes left, body is to the right
			nx, ny, anchor = px+nameMargin, py-float64(fontPx)/2, "start"
		case schema.DirRight: // stub goes right, body is to the left
			nx, ny, anchor = px-nameMargin, py-float64(fontPx)/2, "end"
		case schema.DirUp: // stub goes up on screen, body is below
			nx, ny, anchor = px, py+float64(fontPx)*0.8, "middle"
		default: // DirDown: stub goes down, body is above
			nx, ny, anchor = px, py-nameMargin, "middle"
		}
		b.writef(`<text x="%g" y="%g" transform="rotate(%g,%g,%g)" font-size="%d" text-anchor="%s" fill="darkblue">%s</text>`,
			nx, ny, rot, nx, ny, fontPx*3/4, anchor, xmlEsc(p.Name))
	}
	if p.NumberVisible && p.Number != "" {
		nx := (px + px + dx) / 2
		ny := (py+py+dy)/2 - float64(fontPx)/4
		b.writef(`<text x="%g" y="%g" transform="rotate(%g,%g,%g)" font-size="%d" fill="teal">%s</text>`,
			nx, ny, rot, nx, ny, fontPx*5/8, xmlEsc(p.Number))
	}
}

// ---------- Graphic rendering (sheet-level, absolute coordinates) ----------

func (b *builder) renderGraphic(g schema.Graphic, hPx float64) {
	switch v := g.(type) {
	case schema.Line:
		x1, y1 := flipPt(v.A, hPx)
		x2, y2 := flipPt(v.B, hPx)
		b.writef(`<line x1="%g" y1="%g" x2="%g" y2="%g" style="%s"/>`,
			x1, y1, x2, y2, strokeStyle(v.Style))
	case schema.Rect:
		rx, ry := flipPt(v.Box.Min, hPx)
		mx, my := flipPt(v.Box.Max, hPx)
		x, y, w, h := rectNorm(rx, ry, mx, my)
		b.writef(`<rect x="%g" y="%g" width="%g" height="%g" style="%s"/>`,
			x, y, w, h, rectStyle(v.Style, v.Fill))
	case schema.Ellipse:
		cx, cy := flipPt(v.Center, hPx)
		b.writef(`<ellipse cx="%g" cy="%g" rx="%g" ry="%g" style="%s"/>`,
			cx, cy, nmToPx(v.RX), nmToPx(v.RY), rectStyle(v.Style, v.Fill))
	case schema.Arc:
		b.renderArcAbsolute(v, hPx)
	case schema.Polyline:
		b.renderPolyline(v.Points, strokeStyle(v.Style), hPx)
	case schema.Polygon:
		b.renderPolygonAbsolute(v, hPx)
	}
}

// renderGraphicLocal renders graphics in the symbol local frame (Y already flipped).
func (b *builder) renderGraphicLocal(g schema.Graphic) {
	switch v := g.(type) {
	case schema.Line:
		x1, y1 := localPt(v.A)
		x2, y2 := localPt(v.B)
		b.writef(`<line x1="%g" y1="%g" x2="%g" y2="%g" style="%s"/>`,
			x1, y1, x2, y2, strokeStyle(v.Style))
	case schema.Rect:
		rx, ry := localPt(v.Box.Min)
		mx, my := localPt(v.Box.Max)
		x, y, w, h := rectNorm(rx, ry, mx, my)
		b.writef(`<rect x="%g" y="%g" width="%g" height="%g" style="%s"/>`,
			x, y, w, h, rectStyle(v.Style, v.Fill))
	case schema.Ellipse:
		cx, cy := localPt(v.Center)
		b.writef(`<ellipse cx="%g" cy="%g" rx="%g" ry="%g" style="%s"/>`,
			cx, cy, nmToPx(v.RX), nmToPx(v.RY), rectStyle(v.Style, v.Fill))
	case schema.Arc:
		b.renderArcLocal(v)
	case schema.EllArc:
		b.renderEllArcLocal(v)
	case schema.Polyline:
		b.renderPolylineLocal(v.Points, strokeStyle(v.Style))
	case schema.Polygon:
		b.renderPolygonLocal(v)
	}
}

func (b *builder) renderPolyline(pts []schema.Point, style string, hPx float64) {
	if len(pts) == 0 {
		return
	}
	var sb strings.Builder
	for _, p := range pts {
		x, y := flipPt(p, hPx)
		fmt.Fprintf(&sb, "%g,%g ", x, y)
	}
	b.writef(`<polyline points="%s" style="%s"/>`, strings.TrimSpace(sb.String()), style)
}

func (b *builder) renderPolylineLocal(pts []schema.Point, style string) {
	if len(pts) == 0 {
		return
	}
	var sb strings.Builder
	for _, p := range pts {
		x, y := localPt(p)
		fmt.Fprintf(&sb, "%g,%g ", x, y)
	}
	b.writef(`<polyline points="%s" style="%s"/>`, strings.TrimSpace(sb.String()), style)
}

func (b *builder) renderPolygonAbsolute(v schema.Polygon, hPx float64) {
	var sb strings.Builder
	for _, p := range v.Points {
		x, y := flipPt(p, hPx)
		fmt.Fprintf(&sb, "%g,%g ", x, y)
	}
	b.writef(`<polygon points="%s" style="%s"/>`, strings.TrimSpace(sb.String()), rectStyle(v.Style, v.Fill))
}

func (b *builder) renderPolygonLocal(v schema.Polygon) {
	var sb strings.Builder
	for _, p := range v.Points {
		x, y := localPt(p)
		fmt.Fprintf(&sb, "%g,%g ", x, y)
	}
	b.writef(`<polygon points="%s" style="%s"/>`, strings.TrimSpace(sb.String()), rectStyle(v.Style, v.Fill))
}

// renderArcAbsolute renders a schema.Arc at absolute (sheet) coordinates.
func (b *builder) renderArcAbsolute(a schema.Arc, hPx float64) {
	cx, cy := flipPt(a.Center, hPx)
	r := nmToPx(a.Radius)
	// Altium arc: StartAngle and EndAngle in degrees CCW from positive X.
	// After Y-flip, CCW becomes CW, so flip angles: svgAngle = -angle.
	startDeg := -a.Start
	endDeg := -a.End
	path := arcPath(cx, cy, r, r, startDeg, endDeg)
	b.writef(`<path d="%s" style="%s"/>`, path, strokeStyle(a.Style))
}

func (b *builder) renderArcLocal(a schema.Arc) {
	cx, cy := localPt(a.Center)
	r := nmToPx(a.Radius)
	startDeg := -a.Start
	endDeg := -a.End
	path := arcPath(cx, cy, r, r, startDeg, endDeg)
	b.writef(`<path d="%s" style="%s"/>`, path, strokeStyle(a.Style))
}

func (b *builder) renderEllArcLocal(a schema.EllArc) {
	cx, cy := localPt(a.Center)
	rx := nmToPx(a.RX)
	ry := nmToPx(a.RY)
	startDeg := -a.Start
	endDeg := -a.End
	path := arcPath(cx, cy, rx, ry, startDeg, endDeg)
	b.writef(`<path d="%s" style="%s"/>`, path, strokeStyle(a.Style))
}

// arcPath builds an SVG arc path from centre + radii + start/end angles (degrees).
func arcPath(cx, cy, rx, ry, startDeg, endDeg float64) string {
	// Normalise so we always go from start to end in the short direction.
	for endDeg < startDeg {
		endDeg += 360
	}
	largeArc := 0
	if endDeg-startDeg > 180 {
		largeArc = 1
	}
	sx := cx + rx*math.Cos(startDeg*math.Pi/180)
	sy := cy + ry*math.Sin(startDeg*math.Pi/180)
	ex := cx + rx*math.Cos(endDeg*math.Pi/180)
	ey := cy + ry*math.Sin(endDeg*math.Pi/180)
	return fmt.Sprintf("M%g,%g A%g,%g 0 %d,1 %g,%g", sx, sy, rx, ry, largeArc, ex, ey)
}

// ---------- Coordinate helpers ----------

// flipPt converts a schema point to SVG pixel coordinates.
// Schema: Y-up, nm. SVG: Y-down, px, origin top-left.
func flipPt(p schema.Point, hPx float64) (float64, float64) {
	return nmToPx(p.X), hPx - nmToPx(p.Y)
}

// localPt converts a symbol-local schema point to SVG local px (Y already flipped).
func localPt(p schema.Point) (float64, float64) {
	return nmToPx(p.X), -nmToPx(p.Y)
}

func nmToPx(nm schema.Length) float64 {
	return float64(nm) / float64(nmPerPx)
}

func sheetDims(p schema.Paper) (schema.Length, schema.Length) {
	d := convert.PaperDims(p)
	return d.W, d.H
}

func rectNorm(x1, y1, x2, y2 float64) (x, y, w, h float64) {
	if x2 < x1 {
		x1, x2 = x2, x1
	}
	if y2 < y1 {
		y1, y2 = y2, y1
	}
	return x1, y1, x2 - x1, y2 - y1
}

func strokeStyle(s schema.Stroke) string {
	w := nmToPx(s.Width)
	if w <= 0 {
		w = 0.5
	}
	return fmt.Sprintf("stroke:#%02x%02x%02x;stroke-width:%g;fill:none",
		s.Color.R, s.Color.G, s.Color.B, w)
}

func rectStyle(s schema.Stroke, fill *schema.Color) string {
	base := fmt.Sprintf("stroke:#%02x%02x%02x;stroke-width:%g",
		s.Color.R, s.Color.G, s.Color.B, nmToPx(s.Width))
	if fill != nil {
		return base + fmt.Sprintf(";fill:#%02x%02x%02x", fill.R, fill.G, fill.B)
	}
	return base + ";fill:none"
}

func xmlEsc(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}

// ---------- String builder ----------

type builder struct{ strings.Builder }

func (b *builder) writef(format string, args ...any) {
	fmt.Fprintf(&b.Builder, format+"\n", args...)
}
