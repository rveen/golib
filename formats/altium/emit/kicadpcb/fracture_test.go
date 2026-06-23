package kicadpcb

import (
	"testing"

	"github.com/rveen/golib/formats/altium/altium/pcbmapper"
	"github.com/rveen/golib/formats/altium/altium/pcbreader"
	"github.com/rveen/golib/formats/altium/schema"
)

func pt(x, y int) schema.Point { return schema.Point{X: schema.Length(x), Y: schema.Length(y)} }

func TestFractureNoHoles(t *testing.T) {
	outer := []schema.Point{pt(0, 0), pt(100, 0), pt(100, 100), pt(0, 100)}
	got, ok := fractureFill(outer, nil)
	if !ok {
		t.Fatal("expected success with no holes")
	}
	if len(got) != 4 {
		t.Fatalf("expected 4 points, got %d", len(got))
	}
}

func TestFractureSingleHole(t *testing.T) {
	outer := []schema.Point{pt(0, 0), pt(100, 0), pt(100, 100), pt(0, 100)}
	hole := []schema.Point{pt(40, 40), pt(60, 40), pt(60, 60), pt(40, 60)}
	got, ok := fractureFill(outer, [][]schema.Point{hole})
	if !ok {
		t.Fatal("expected fracture success")
	}
	// 4 outer + 4 hole + 2 bridge-duplicated vertices = 10.
	if len(got) != 10 {
		t.Fatalf("expected 10 points, got %d", len(got))
	}
	if contourSelfIntersects(got) {
		t.Error("fractured contour self-intersects")
	}
}

func TestFractureMultipleHoles(t *testing.T) {
	outer := []schema.Point{pt(0, 0), pt(200, 0), pt(200, 200), pt(0, 200)}
	holes := [][]schema.Point{
		{pt(20, 20), pt(40, 20), pt(40, 40), pt(20, 40)},
		{pt(160, 160), pt(180, 160), pt(180, 180), pt(160, 180)},
		{pt(90, 90), pt(110, 90), pt(110, 110), pt(90, 110)},
	}
	got, ok := fractureFill(outer, holes)
	if !ok {
		t.Fatal("expected fracture success")
	}
	if contourSelfIntersects(got) {
		t.Error("fractured contour with multiple holes self-intersects")
	}
}

// TestFractureRealData fractures every actual zone fill from the sample board and
// verifies the result is either a clean (non-self-intersecting) contour or a
// reported failure (which the emitter handles by skipping the cached fill).
func TestFractureRealData(t *testing.T) {
	rb, err := pcbreader.ReadFile("../../testdata/test.PcbDoc")
	if err != nil {
		t.Skipf("sample board unavailable: %v", err)
	}
	board, _, err := pcbmapper.Map(rb, "test.PcbDoc")
	if err != nil {
		t.Fatal(err)
	}
	var total, ok, failed, withHoles int
	for _, z := range board.Zones {
		for _, fill := range z.Fills {
			if len(fill.Vertices) < 3 {
				continue
			}
			total++
			if len(fill.Holes) > 0 {
				withHoles++
			}
			contour, good := fractureFill(fill.Vertices, fill.Holes)
			if !good {
				failed++
				continue
			}
			ok++
			if contourSelfIntersects(contour) {
				t.Errorf("real zone fill produced self-intersecting contour (%d outline pts, %d holes)",
					len(fill.Vertices), len(fill.Holes))
			}
		}
	}
	t.Logf("zone fills: %d total, %d with holes, %d fractured OK, %d fell back", total, withHoles, ok, failed)
}

func TestFractureWindingNormalized(t *testing.T) {
	// Outer CCW, hole also given CCW (same winding) — must still produce a clean slit.
	outer := []schema.Point{pt(0, 0), pt(100, 0), pt(100, 100), pt(0, 100)}
	holeSameWinding := []schema.Point{pt(40, 40), pt(60, 40), pt(60, 60), pt(40, 60)}
	got, ok := fractureFill(outer, [][]schema.Point{holeSameWinding})
	if !ok {
		t.Fatal("expected fracture success")
	}
	if contourSelfIntersects(got) {
		t.Error("fractured contour self-intersects after winding normalization")
	}
}
