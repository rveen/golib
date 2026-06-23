// Package symcat emits a symbol-catalog SVG: one cell per unique symbol,
// arranged in a grid, with a red cross at each symbol's origin point.
// Each symbol is scaled to fill its cell, so even tiny or very large symbols
// are legible at a glance.
package symcat

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/rveen/golib/formats/altium/emit"
	"github.com/rveen/golib/formats/altium/schema"
)

const nmPerPx = 25400

const (
	cellW       = 280.0 // fixed output cell width (px)
	cellH       = 260.0 // fixed output cell height (px), includes label footer
	labelH      = 24.0  // height reserved for the LibRef label
	cellPad     = 18.0  // padding between cell border and symbol bbox
	gridCols    = 4
	crossLen    = 8.0 // half-length of origin cross arms (px, fixed, not scaled)
	labelFontPx = 12
)

// Emitter produces a symbol-catalog SVG.
type Emitter struct{}

func (Emitter) Name() string { return "symcat" }

// Emit renders one SVG containing all unique symbols laid out in a grid.
// Each symbol is scaled to fill its cell; the cross at origin is always the
// same size so it is visible regardless of symbol scale.
func (Emitter) Emit(s *schema.Schematic, _ any) ([]emit.Artifact, *emit.Report, error) {
	rep := &emit.Report{}

	syms := sortedSymbols(s.Symbols)
	if len(syms) == 0 {
		return nil, rep, nil
	}

	bbs := make([]bbox, len(syms))
	for i, sym := range syms {
		bbs[i] = symBBox(sym)
	}

	cols := gridCols
	rows := (len(syms) + cols - 1) / cols
	svgW := float64(cols) * cellW
	svgH := float64(rows) * cellH

	b := &builder{}
	b.writef(`<?xml version="1.0" encoding="UTF-8"?>`)
	b.writef(`<svg xmlns="http://www.w3.org/2000/svg" width="%gpx" height="%gpx" viewBox="0 0 %g %g">`,
		svgW, svgH, svgW, svgH)
	b.writef(`<rect width="%g" height="%g" fill="white"/>`, svgW, svgH)

	// Drawing area within each cell (above the label footer).
	drawW := cellW - 2*cellPad
	drawH := cellH - labelH - 2*cellPad

	for i, sym := range syms {
		col := i % cols
		row := i / cols
		cellX := float64(col) * cellW
		cellY := float64(row) * cellH

		// Cell border.
		b.writef(`<rect x="%g" y="%g" width="%g" height="%g" fill="none" stroke="#ccc" stroke-width="0.5"/>`,
			cellX, cellY, cellW, cellH)

		// Scale the symbol so its bounding box fills the drawing area.
		bb := bbs[i]
		sf := fitScale(bb, drawW, drawH)

		// Where the symbol origin (0,0) lands in SVG coordinates.
		// The bbox centre maps to the drawing-area centre; origin floats from there.
		drawCx := cellX + cellW/2
		drawCy := cellY + cellPad + drawH/2
		ox := drawCx - sf*nmToPx(bb.midX())
		oy := drawCy + sf*nmToPx(bb.midY()) // + because lp() flips Y

		// Symbol geometry (rendered in local px, then scaled + translated).
		b.writef(`<g transform="translate(%g,%g) scale(%g,%g)">`, ox, oy, sf, sf)
		b.renderSymbolGraphics(sym, sf)
		b.writef(`</g>`)

		// Origin cross — drawn in cell coordinates so size is always crossLen px.
		b.writef(`<line x1="%g" y1="%g" x2="%g" y2="%g" stroke="red" stroke-width="1"/>`,
			ox-crossLen, oy, ox+crossLen, oy)
		b.writef(`<line x1="%g" y1="%g" x2="%g" y2="%g" stroke="red" stroke-width="1"/>`,
			ox, oy-crossLen, ox, oy+crossLen)

		// LibRef label centred at the bottom of the cell.
		b.writef(`<text x="%g" y="%g" font-size="%d" text-anchor="middle" font-family="monospace" fill="#333">%s</text>`,
			cellX+cellW/2, cellY+cellH-labelH/4, labelFontPx, xmlEsc(sym.LibRef))
	}

	b.writef(`</svg>`)
	return []emit.Artifact{{Name: "symbols.svg", Data: []byte(b.String())}}, rep, nil
}

// fitScale returns the uniform scale factor that makes the symbol's bounding
// box fit within a drawW × drawH pixel area, with some margin.
func fitScale(bb bbox, drawW, drawH float64) float64 {
	w, h := bb.wPx(), bb.hPx()
	if w == 0 && h == 0 {
		return 1.0
	}
	var sf float64
	if w == 0 {
		sf = drawH / h
	} else if h == 0 {
		sf = drawW / w
	} else {
		sf = math.Min(drawW/w, drawH/h)
	}
	return sf * 0.85 // leave a small margin around the bbox
}

// ---------- Symbol rendering ----------

func (b *builder) renderSymbolGraphics(sym *schema.Symbol, sf float64) {
	for _, g := range sym.Graphics {
		b.renderGraphic(g)
	}
	for _, p := range sym.Pins {
		b.renderPin(p, sf)
	}
}

func (b *builder) renderGraphic(g schema.Graphic) {
	switch v := g.(type) {
	case schema.Line:
		x1, y1 := lp(v.A)
		x2, y2 := lp(v.B)
		b.writef(`<line x1="%g" y1="%g" x2="%g" y2="%g" style="%s"/>`, x1, y1, x2, y2, strokeCSS(v.Style))
	case schema.Rect:
		ax, ay := lp(v.Box.Min)
		bx, by := lp(v.Box.Max)
		x, y, w, h := rectNorm(ax, ay, bx, by)
		b.writef(`<rect x="%g" y="%g" width="%g" height="%g" style="%s"/>`, x, y, w, h, fillCSS(v.Style, v.Fill))
	case schema.Ellipse:
		cx, cy := lp(v.Center)
		b.writef(`<ellipse cx="%g" cy="%g" rx="%g" ry="%g" style="%s"/>`,
			cx, cy, nmToPx(v.RX), nmToPx(v.RY), fillCSS(v.Style, v.Fill))
	case schema.Arc:
		cx, cy := lp(v.Center)
		b.renderArc(cx, cy, nmToPx(v.Radius), nmToPx(v.Radius), v.Start, v.End, strokeCSS(v.Style))
	case schema.EllArc:
		cx, cy := lp(v.Center)
		b.renderArc(cx, cy, nmToPx(v.RX), nmToPx(v.RY), v.Start, v.End, strokeCSS(v.Style))
	case schema.Polyline:
		b.renderPolyline(v.Points, strokeCSS(v.Style))
	case schema.Polygon:
		b.renderPolygon(v.Points, fillCSS(v.Style, v.Fill))
	case schema.Bezier:
		b.renderPolyline(v.Points, strokeCSS(v.Style)) // approximate as polyline
	}
}

func (b *builder) renderPin(p *schema.Pin, sf float64) {
	px, py := lp(p.Position)
	length := nmToPx(p.PinLength)

	var dx, dy float64
	switch p.Orientation {
	case schema.DirRight:
		dx = length
	case schema.DirLeft:
		dx = -length
	case schema.DirUp:
		dy = -length
	case schema.DirDown:
		dy = length
	}

	if length != 0 {
		b.writef(`<line x1="%g" y1="%g" x2="%g" y2="%g" stroke="#444" stroke-width="0.5"/>`,
			px, py, px+dx, py+dy)
	}
	// Dot at the connection (wire-attachment) end.
	b.writef(`<circle cx="%g" cy="%g" r="2" fill="#444"/>`, px+dx, py+dy)

	// Text sizes in local coords so they appear at a fixed output size after
	// the enclosing scale(sf,sf) group transform: localSize = outputPx / sf.
	numSz := 7.0 / sf  // pin number target: 7 px output
	nameSz := 8.0 / sf // pin name target: 8 px output

	// Pin number at the midpoint of the stub, offset perpendicular to it.
	if p.Number != "" {
		mx, my := px+dx/2, py+dy/2
		var ox, oy float64
		switch p.Orientation {
		case schema.DirRight, schema.DirLeft:
			oy = -numSz * 0.6 // above the stub
		case schema.DirUp, schema.DirDown:
			ox = numSz * 0.5 // to the right of the stub
		}
		b.writef(`<text x="%g" y="%g" font-size="%g" text-anchor="middle" fill="#888">%s</text>`,
			mx+ox, my+oy, numSz, xmlEsc(p.Number))
	}

	// Pin name just beyond the connection end, extending outward from the body.
	if p.Name != "" {
		cx, cy := px+dx, py+dy
		var anchor string
		var ox, oy float64
		switch p.Orientation {
		case schema.DirRight:
			anchor, ox = "start", nameSz*0.3
		case schema.DirLeft:
			anchor, ox = "end", -nameSz*0.3
		case schema.DirUp:
			anchor, oy = "middle", -nameSz*0.4
		case schema.DirDown:
			anchor, oy = "middle", nameSz*1.1
		}
		b.writef(`<text x="%g" y="%g" font-size="%g" text-anchor="%s" fill="#00a">%s</text>`,
			cx+ox, cy+oy, nameSz, anchor, xmlEsc(p.Name))
	}
}

func (b *builder) renderArc(cx, cy, rx, ry, start, end float64, style string) {
	// Altium angles: CCW from +X. After local Y-flip, CCW becomes CW, so negate.
	s, e := -start, -end
	for e < s {
		e += 360
	}
	largeArc := 0
	if e-s > 180 {
		largeArc = 1
	}
	sx := cx + rx*math.Cos(s*math.Pi/180)
	sy := cy + ry*math.Sin(s*math.Pi/180)
	ex := cx + rx*math.Cos(e*math.Pi/180)
	ey := cy + ry*math.Sin(e*math.Pi/180)
	b.writef(`<path d="M%g,%g A%g,%g 0 %d,1 %g,%g" style="%s"/>`, sx, sy, rx, ry, largeArc, ex, ey, style)
}

func (b *builder) renderPolyline(pts []schema.Point, style string) {
	if len(pts) == 0 {
		return
	}
	var sb strings.Builder
	for _, p := range pts {
		x, y := lp(p)
		fmt.Fprintf(&sb, "%g,%g ", x, y)
	}
	b.writef(`<polyline points="%s" style="%s"/>`, strings.TrimSpace(sb.String()), style)
}

func (b *builder) renderPolygon(pts []schema.Point, style string) {
	if len(pts) == 0 {
		return
	}
	var sb strings.Builder
	for _, p := range pts {
		x, y := lp(p)
		fmt.Fprintf(&sb, "%g,%g ", x, y)
	}
	b.writef(`<polygon points="%s" style="%s"/>`, strings.TrimSpace(sb.String()), style)
}

// ---------- Bounding box ----------

type bbox struct{ minX, minY, maxX, maxY schema.Length }

func newBBox() bbox {
	big := schema.Length(math.MaxInt64 / 2)
	return bbox{minX: big, minY: big, maxX: -big, maxY: -big}
}

func (bb *bbox) add(x, y schema.Length) {
	if x < bb.minX {
		bb.minX = x
	}
	if x > bb.maxX {
		bb.maxX = x
	}
	if y < bb.minY {
		bb.minY = y
	}
	if y > bb.maxY {
		bb.maxY = y
	}
}

func (bb bbox) midX() schema.Length { return (bb.minX + bb.maxX) / 2 }
func (bb bbox) midY() schema.Length { return (bb.minY + bb.maxY) / 2 }
func (bb bbox) wPx() float64        { return nmToPx(bb.maxX - bb.minX) }
func (bb bbox) hPx() float64        { return nmToPx(bb.maxY - bb.minY) }

func symBBox(sym *schema.Symbol) bbox {
	bb := newBBox()
	bb.add(0, 0) // always include the origin

	for _, g := range sym.Graphics {
		expandBB(&bb, g)
	}
	for _, p := range sym.Pins {
		bb.add(p.Position.X, p.Position.Y)
		cx, cy := connEnd(p)
		bb.add(cx, cy)
	}
	return bb
}

func expandBB(bb *bbox, g schema.Graphic) {
	switch v := g.(type) {
	case schema.Line:
		bb.add(v.A.X, v.A.Y)
		bb.add(v.B.X, v.B.Y)
	case schema.Rect:
		bb.add(v.Box.Min.X, v.Box.Min.Y)
		bb.add(v.Box.Max.X, v.Box.Max.Y)
	case schema.Ellipse:
		bb.add(v.Center.X-v.RX, v.Center.Y-v.RY)
		bb.add(v.Center.X+v.RX, v.Center.Y+v.RY)
	case schema.Arc:
		bb.add(v.Center.X-v.Radius, v.Center.Y-v.Radius)
		bb.add(v.Center.X+v.Radius, v.Center.Y+v.Radius)
	case schema.EllArc:
		bb.add(v.Center.X-v.RX, v.Center.Y-v.RY)
		bb.add(v.Center.X+v.RX, v.Center.Y+v.RY)
	case schema.Polyline:
		for _, p := range v.Points {
			bb.add(p.X, p.Y)
		}
	case schema.Polygon:
		for _, p := range v.Points {
			bb.add(p.X, p.Y)
		}
	case schema.Bezier:
		for _, p := range v.Points {
			bb.add(p.X, p.Y)
		}
	}
}

func connEnd(p *schema.Pin) (schema.Length, schema.Length) {
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

// ---------- Sorting ----------

func sortedSymbols(m map[schema.SymbolID]*schema.Symbol) []*schema.Symbol {
	out := make([]*schema.Symbol, 0, len(m))
	for _, s := range m {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].LibRef < out[j].LibRef
	})
	return out
}

// ---------- Coordinate helpers ----------

// lp converts a schema point to local SVG coordinates (Y-flipped, unscaled).
// Scaling is applied by the enclosing SVG group transform.
func lp(p schema.Point) (float64, float64) {
	return nmToPx(p.X), -nmToPx(p.Y)
}

func nmToPx(nm schema.Length) float64 {
	return float64(nm) / float64(nmPerPx)
}

// ---------- Style helpers ----------

func strokeCSS(s schema.Stroke) string {
	w := nmToPx(s.Width)
	if w <= 0 {
		w = 0.5
	}
	return fmt.Sprintf("stroke:#%02x%02x%02x;stroke-width:%g;fill:none",
		s.Color.R, s.Color.G, s.Color.B, w)
}

func fillCSS(s schema.Stroke, fill *schema.Color) string {
	base := fmt.Sprintf("stroke:#%02x%02x%02x;stroke-width:%g",
		s.Color.R, s.Color.G, s.Color.B, nmToPx(s.Width))
	if fill != nil {
		return base + fmt.Sprintf(";fill:#%02x%02x%02x", fill.R, fill.G, fill.B)
	}
	return base + ";fill:none"
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

func xmlEsc(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}

// ---------- Builder ----------

type builder struct{ strings.Builder }

func (b *builder) writef(format string, args ...any) {
	fmt.Fprintf(&b.Builder, format+"\n", args...)
}
