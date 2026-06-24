package convert

import (
	"testing"

	"github.com/rveen/golib/formats/altium/schema"
)

func TestMilsToNm(t *testing.T) {
	tests := []struct {
		mils, frac int
		want       schema.Length
	}{
		{0, 0, 0},
		{1, 0, 25400},
		{0, 10000, 25400}, // 1 mil via fractional part
		{100, 0, 2540000},
		{-1, 0, -25400},
		{1, 5000, 38100}, // 1.5 mils = 38 100 nm
	}
	for _, tc := range tests {
		got := MilsToNm(tc.mils, tc.frac)
		if got != tc.want {
			t.Errorf("MilsToNm(%d,%d) = %d, want %d", tc.mils, tc.frac, got, tc.want)
		}
	}
}

func TestFlipY(t *testing.T) {
	if FlipY(100) != -100 {
		t.Error("FlipY(100) should be -100")
	}
	if FlipY(0) != 0 {
		t.Error("FlipY(0) should be 0")
	}
}

func TestBGRToColor(t *testing.T) {
	tests := []struct {
		in      uint32
		r, g, b uint8
	}{
		{0x000000FF, 0xFF, 0x00, 0x00}, // pure red in BGR
		{0x00FF0000, 0x00, 0x00, 0xFF}, // pure blue in BGR
		{0x0000FF00, 0x00, 0xFF, 0x00}, // pure green
		{0x00FFFFFF, 0xFF, 0xFF, 0xFF}, // white
		{0x00000000, 0x00, 0x00, 0x00}, // black
	}
	for _, tc := range tests {
		got := BGRToColor(tc.in)
		if got.R != tc.r || got.G != tc.g || got.B != tc.b || got.A != 255 {
			t.Errorf("BGRToColor(%06x) = {%d,%d,%d,%d}, want {%d,%d,%d,255}",
				tc.in, got.R, got.G, got.B, got.A, tc.r, tc.g, tc.b)
		}
	}
}

func TestOverbarAltiumToKicad(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"CHRG", "CHRG"},            // no overbar
		{"C\\H\\R\\G\\", "~{CHRG}"}, // full overbar
		{"N\\C", "~{N}C"},           // partial: first char overbarred
		{"AB\\C", "A~{B}C"},         // middle
		{"", ""},                    // empty
		{"A\\B\\C", "~{AB}C"},       // two consecutive overbar chars then plain
	}
	for _, tc := range tests {
		got := OverbarAltiumToKicad(tc.in)
		if got != tc.want {
			t.Errorf("OverbarAltiumToKicad(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestPinOrientation(t *testing.T) {
	tests := []struct {
		in   int
		want schema.Dir4
	}{
		{0, schema.DirRight},
		{1, schema.DirUp},
		{2, schema.DirLeft},
		{3, schema.DirDown},
		{-1, schema.DirRight}, // unknown → default
		{99, schema.DirRight},
	}
	for _, tc := range tests {
		got := PinOrientation(tc.in)
		if got != tc.want {
			t.Errorf("PinOrientation(%d) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestComponentOrientation(t *testing.T) {
	tests := []struct {
		in   int
		want schema.Angle
	}{
		{0, 0},
		{1, 90},
		{2, 180},
		{3, 270},
		{-1, 0},
		{4, 0},
	}
	for _, tc := range tests {
		got := ComponentOrientation(tc.in)
		if got != tc.want {
			t.Errorf("ComponentOrientation(%d) = %g, want %g", tc.in, got, tc.want)
		}
	}
}

func TestPinElectrical(t *testing.T) {
	tests := []struct {
		in   int
		want schema.PinType
	}{
		{0, schema.PinInput},
		{1, schema.PinBidi},
		{4, schema.PinPassive},
		{7, schema.PinPower},
		{-1, schema.PinPassive},
		{99, schema.PinPassive},
	}
	for _, tc := range tests {
		got := PinElectrical(tc.in)
		if got != tc.want {
			t.Errorf("PinElectrical(%d) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestSheetSize(t *testing.T) {
	// Standard size.
	p := SheetSize(0, false, 0, 0, 0, 0, false)
	if p.Std != schema.PaperA4 {
		t.Errorf("style 0: got %d want PaperA4", p.Std)
	}
	// Custom size: 1000 mils × 500 mils.
	p2 := SheetSize(0, true, 1000, 0, 500, 0, true)
	if p2.Std != schema.PaperCustom || p2.Custom == nil {
		t.Errorf("custom: expected PaperCustom with non-nil size")
	}
	wantW := MilsToNm(1000, 0)
	if p2.Custom.W != wantW {
		t.Errorf("custom W: got %d want %d", p2.Custom.W, wantW)
	}
	if !p2.Portrait {
		t.Error("expected portrait=true")
	}
	// Out-of-range style → A4 fallback.
	p3 := SheetSize(99, false, 0, 0, 0, 0, false)
	if p3.Std != schema.PaperA4 {
		t.Errorf("unknown style: got %d want PaperA4", p3.Std)
	}
}

func TestNormalizeAngle(t *testing.T) {
	tests := []struct{ in, want schema.Angle }{
		{0, 0},
		{90, 90},
		{360, 0},
		{-90, 270},
		{450, 90},
	}
	for _, tc := range tests {
		got := NormalizeAngle(tc.in)
		if got != tc.want {
			t.Errorf("NormalizeAngle(%g) = %g, want %g", tc.in, got, tc.want)
		}
	}
}

func TestTextPositioning(t *testing.T) {
	// h: -1 left, 0 center, +1 right.  v: -1 top, 0 center, +1 bottom.
	tests := []struct {
		name         string
		just         schema.Justify
		orient       schema.Angle
		wantA        int
		wantH, wantV int
	}{
		// Altium default (bottom-left, rightwards) → horizontal, left bottom.
		{"default", schema.JustifyBottomLeft, 0, 0, -1, 1},
		// Leftwards flips horizontal justification but stays horizontal.
		{"leftwards", schema.JustifyBottomLeft, 180, 0, 1, 1},
		// Upwards → vertical, justification unchanged.
		{"upwards", schema.JustifyBottomLeft, 90, 90, -1, 1},
		// Downwards → vertical with flipped horizontal justification.
		{"downwards", schema.JustifyBottomLeft, 270, 90, 1, 1},
		// Top-right anchor.
		{"top-right", schema.JustifyTopRight, 0, 0, 1, -1},
		// Center-center stays centered (no justify clause).
		{"center", schema.JustifyCenterCenter, 0, 0, 0, 0},
		// Never produces 180°/270° — leftwards rightward-flips, not rotates.
		{"right-leftwards", schema.JustifyBottomRight, 180, 0, -1, 1},
	}
	for _, tc := range tests {
		a, h, v := TextPositioning(tc.just, tc.orient)
		if a != tc.wantA || h != tc.wantH || v != tc.wantV {
			t.Errorf("%s: TextPositioning(%d, %g) = (%d, %d, %d), want (%d, %d, %d)",
				tc.name, tc.just, tc.orient, a, h, v, tc.wantA, tc.wantH, tc.wantV)
		}
		if a != 0 && a != 90 {
			t.Errorf("%s: angle %d is not 0 or 90 (text would render upside down)", tc.name, a)
		}
	}
}

func TestCompensateFieldForInstanceRotation(t *testing.T) {
	// A default designator is absolute horizontal, left bottom (angle 0, h -1, v +1).
	// After compensation it must STORE values that render back to left-bottom once
	// KiCad re-applies the instance rotation.
	tests := []struct {
		instRot             int
		wantA, wantH, wantV int
	}{
		{0, 0, -1, 1},    // identity
		{90, 90, 1, -1},  // R7: stored vertical right-top; KiCad folds 180° → horizontal left-bottom
		{180, 0, 1, -1},  // 180° fold negates both axes
		{270, 90, -1, 1}, // net 360° → no fold, no negate
	}
	for _, tc := range tests {
		a, h, v := CompensateFieldForInstanceRotation(0, -1, 1, tc.instRot)
		if a != tc.wantA || h != tc.wantH || v != tc.wantV {
			t.Errorf("instRot %d: got (%d,%d,%d), want (%d,%d,%d)",
				tc.instRot, a, h, v, tc.wantA, tc.wantH, tc.wantV)
		}
		if a != 0 && a != 90 {
			t.Errorf("instRot %d: stored angle %d is not 0 or 90", tc.instRot, a)
		}
	}
}

func TestCompensateFieldRoundTrip(t *testing.T) {
	// Applying KiCad's render transform to the compensated values must recover
	// the intended absolute appearance for every instance rotation.
	render := func(a, h, v, instRot int) (int, int, int) {
		net := ((a+instRot)%360 + 360) % 360
		switch net {
		case 180:
			return 0, -h, -v
		case 270:
			return 90, -h, -v
		default: // 0 or 90
			return net, h, v
		}
	}
	for _, absA := range []int{0, 90} {
		for _, absH := range []int{-1, 0, 1} {
			for _, absV := range []int{-1, 0, 1} {
				for _, rot := range []int{0, 90, 180, 270} {
					sa, sh, sv := CompensateFieldForInstanceRotation(absA, absH, absV, rot)
					ra, rh, rv := render(sa, sh, sv, rot)
					if ra != absA || rh != absH || rv != absV {
						t.Errorf("rot %d abs(%d,%d,%d): stored(%d,%d,%d) renders(%d,%d,%d)",
							rot, absA, absH, absV, sa, sh, sv, ra, rh, rv)
					}
				}
			}
		}
	}
}
