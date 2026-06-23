package kicadpcb

import (
	"sort"

	"github.com/rveen/golib/formats/altium/schema"
)

// fracture.go implements polygon hole elimination ("fracturing").
//
// Altium stores each filled copper region as an outer outline plus a set of
// hole contours (anti-pads, thermal clearances). KiCad's filled_polygon format
// expects a single contour per island, with holes woven into the outline via a
// zero-width "slit" (a pair of coincident bridge edges). KiCad reconstructs the
// holes on load by detecting these coincident edges.
//
// fractureFill merges all holes of a fill into one such slit-bridged contour.
// It returns (contour, true) on success, or (nil, false) if a hole cannot be
// bridged without self-intersection — in which case the caller should not emit
// a cached fill (KiCad regenerates the fill from the zone outline instead).

// signedArea returns twice the signed area of a polygon (shoelace). Positive and
// negative encode opposite winding directions.
func signedArea(p []schema.Point) float64 {
	var a float64
	n := len(p)
	for i := 0; i < n; i++ {
		j := (i + 1) % n
		a += float64(p[i].X)*float64(p[j].Y) - float64(p[j].X)*float64(p[i].Y)
	}
	return a
}

func reversePts(p []schema.Point) {
	for i, j := 0, len(p)-1; i < j; i, j = i+1, j-1 {
		p[i], p[j] = p[j], p[i]
	}
}

// segmentsProperlyIntersect reports whether segments p1p2 and p3p4 cross at an
// interior point. Shared endpoints and collinear touching do NOT count, so a
// bridge that merely meets an edge at a shared vertex is allowed.
func segmentsProperlyIntersect(p1, p2, p3, p4 schema.Point) bool {
	d1 := cross3(p3, p4, p1)
	d2 := cross3(p3, p4, p2)
	d3 := cross3(p1, p2, p3)
	d4 := cross3(p1, p2, p4)
	if ((d1 > 0 && d2 < 0) || (d1 < 0 && d2 > 0)) &&
		((d3 > 0 && d4 < 0) || (d3 < 0 && d4 > 0)) {
		return true
	}
	return false
}

// cross3 returns the 2D cross product (a-o) × (b-o).
func cross3(o, a, b schema.Point) float64 {
	return float64(a.X-o.X)*float64(b.Y-o.Y) - float64(a.Y-o.Y)*float64(b.X-o.X)
}

// contourSelfIntersects reports whether a closed contour has any two
// non-adjacent edges that properly cross. Altium region outlines are not always
// simple polygons, and bridging can occasionally produce crossings; KiCad can
// crash on self-intersecting fills, so such contours must not be emitted.
func contourSelfIntersects(p []schema.Point) bool {
	n := len(p)
	for i := 0; i < n; i++ {
		a, b := p[i], p[(i+1)%n]
		for j := i + 1; j < n; j++ {
			// Skip edges adjacent to edge i (they legitimately share a vertex).
			if (j+1)%n == i || (i+1)%n == j {
				continue
			}
			c, d := p[j], p[(j+1)%n]
			if segmentsProperlyIntersect(a, b, c, d) {
				return true
			}
		}
	}
	return false
}

// bridgeCrossesAny reports whether the bridge segment from a to b properly
// crosses any edge of the given closed contour.
func bridgeCrossesAny(a, b schema.Point, contour []schema.Point) bool {
	n := len(contour)
	for i := 0; i < n; i++ {
		if segmentsProperlyIntersect(a, b, contour[i], contour[(i+1)%n]) {
			return true
		}
	}
	return false
}

// bridgeHole merges one hole into the current contour by finding a collision-free
// bridge between a hole vertex and a contour vertex (preferring the shortest).
// Returns the merged contour and true on success.
func bridgeHole(cur, hole []schema.Point) ([]schema.Point, bool) {
	type cand struct {
		oi, hi int
		dist   float64
	}
	cands := make([]cand, 0, len(cur)*len(hole))
	for hi := range hole {
		for oi := range cur {
			dx := float64(cur[oi].X - hole[hi].X)
			dy := float64(cur[oi].Y - hole[hi].Y)
			cands = append(cands, cand{oi, hi, dx*dx + dy*dy})
		}
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i].dist < cands[j].dist })

	for _, c := range cands {
		o := cur[c.oi]
		h := hole[c.hi]
		// The bridge must not properly cross the contour or the hole boundary.
		if bridgeCrossesAny(o, h, cur) || bridgeCrossesAny(o, h, hole) {
			continue
		}
		// Build the slit-bridged contour:
		//   cur[0..oi] -> hole[hi..] -> hole[..hi] -> hole[hi] -> cur[oi] -> cur[oi+1..]
		merged := make([]schema.Point, 0, len(cur)+len(hole)+2)
		merged = append(merged, cur[:c.oi+1]...)
		for k := 0; k < len(hole); k++ {
			merged = append(merged, hole[(c.hi+k)%len(hole)])
		}
		merged = append(merged, hole[c.hi]) // close the hole loop
		merged = append(merged, cur[c.oi:]...)
		return merged, true
	}
	return nil, false
}

// fractureFill merges an outline and its holes into a single slit-bridged contour.
// Returns (contour, true) on success or (nil, false) if any hole cannot be bridged.
func fractureFill(outer []schema.Point, holes [][]schema.Point) ([]schema.Point, bool) {
	if len(outer) < 3 {
		return nil, false
	}
	if len(holes) == 0 {
		// Even with no holes, reject a non-simple source outline.
		if contourSelfIntersects(outer) {
			return nil, false
		}
		return outer, true
	}

	cur := append([]schema.Point(nil), outer...)
	outerCCW := signedArea(outer) > 0

	// Prepare holes: normalize each to the opposite winding of the outline (so the
	// slit subtracts area), and process them right-to-left for stable bridging.
	type prepped struct {
		pts  []schema.Point
		maxX schema.Length
	}
	hs := make([]prepped, 0, len(holes))
	for _, h := range holes {
		if len(h) < 3 {
			continue
		}
		hh := append([]schema.Point(nil), h...)
		if (signedArea(hh) > 0) == outerCCW {
			reversePts(hh)
		}
		mx := hh[0].X
		for _, v := range hh {
			if v.X > mx {
				mx = v.X
			}
		}
		hs = append(hs, prepped{hh, mx})
	}
	sort.Slice(hs, func(i, j int) bool { return hs[i].maxX > hs[j].maxX })

	for _, hd := range hs {
		merged, ok := bridgeHole(cur, hd.pts)
		if !ok {
			return nil, false
		}
		cur = merged
	}
	// Final safety check: never emit a self-intersecting contour to KiCad.
	if contourSelfIntersects(cur) {
		return nil, false
	}
	return cur, true
}
