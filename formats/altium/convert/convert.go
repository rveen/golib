// Package convert centralises every source-to-canonical conversion used when
// mapping Altium records to the schema IR.  All conversions are pure functions
// so they can be tested in isolation.
//
// Y-axis convention:  Altium Y increases upward; the schema IR preserves that
// convention (Y-up positive).  The KiCad emitter is responsible for flipping Y
// exactly once when writing output.  FlipY is provided for emitters that need it.
package convert

import (
	"strings"

	"github.com/rveen/golib/formats/altium/schema"
)

// MilsToNm converts an Altium coordinate pair (integer mils + fractional
// 1/10000-mil part) to nanometres.
//
//	1 mil = 25 400 nm
//	1/10000 mil = 2.54 nm  →  multiply by 254, divide by 100
func MilsToNm(mils, frac int) schema.Length {
	return schema.Length((int64(mils)*10000+int64(frac))*254) / 100
}

// FlipY negates a Y coordinate. Emitters that convert schema Y-up to a
// Y-down target generally flip about the page height rather than about 0
// (see PaperDims); this plain negation is kept as a primitive helper.
func FlipY(y schema.Length) schema.Length { return -y }

// BGRToColor converts an Altium 24-bit BGR integer to schema.Color.
// Alpha is always 255 (fully opaque).
func BGRToColor(c uint32) schema.Color {
	return schema.Color{
		R: uint8(c & 0xFF),
		G: uint8((c >> 8) & 0xFF),
		B: uint8((c >> 16) & 0xFF),
		A: 255,
	}
}

// OverbarAltiumToKicad converts Altium's overbar notation to KiCad's.
// Altium places a backslash after each overlined character: "C\H\R\G\" → "~{CHRG}".
// Characters not followed by a backslash are passed through unchanged.
func OverbarAltiumToKicad(s string) string {
	if !strings.ContainsRune(s, '\\') {
		return s
	}
	var b strings.Builder
	var run strings.Builder
	inRun := false
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if i+1 < len(s) && s[i+1] == '\\' {
			// This character is overlined.
			if !inRun {
				inRun = true
			}
			run.WriteByte(ch)
			i++ // skip the backslash
		} else {
			if inRun {
				b.WriteString("~{")
				b.WriteString(run.String())
				b.WriteByte('}')
				run.Reset()
				inRun = false
			}
			b.WriteByte(ch)
		}
	}
	if inRun {
		b.WriteString("~{")
		b.WriteString(run.String())
		b.WriteByte('}')
	}
	return b.String()
}

// PinOrientation maps Altium pin ORIENTATION values to canonical Dir4.
// Altium: 0=Right, 1=Up, 2=Left, 3=Down.
var pinOrientationTable = [4]schema.Dir4{
	schema.DirRight,
	schema.DirUp,
	schema.DirLeft,
	schema.DirDown,
}

// PinOrientation returns the canonical direction for an Altium pin orientation
// value (0–3). Unknown values default to DirRight.
func PinOrientation(v int) schema.Dir4 {
	if v >= 0 && v < len(pinOrientationTable) {
		return pinOrientationTable[v]
	}
	return schema.DirRight
}

// ComponentOrientation maps Altium component ORIENTATION (0–3) to degrees CCW.
var componentOrientTable = [4]schema.Angle{0, 90, 180, 270}

// ComponentOrientation returns the rotation angle in degrees CCW for an Altium
// component orientation value (0–3). Unknown values default to 0°.
func ComponentOrientation(v int) schema.Angle {
	if v >= 0 && v < len(componentOrientTable) {
		return componentOrientTable[v]
	}
	return 0
}

// AltiumPowerStyle maps the Altium STYLE integer for a power port to the
// canonical PowerStyle enum. Mapping derived from observed schematics:
//
//	2 → Arrow (VCC/VDD/+5V rail symbols with a single bar)
//	4 → Earth (multi-bar earth ground, e.g. GND_ISO)
//	5 → GND   (standard three-bar power ground)
//	7 → Tee
//
// All other values map to Bar (simple horizontal bar).
func AltiumPowerStyle(v int) schema.PowerStyle {
	switch v {
	case 2, 3:
		return schema.PowerStyleArrow
	case 4:
		return schema.PowerStyleEarth
	case 5:
		return schema.PowerStyleGND
	case 7:
		return schema.PowerStyleTee
	default:
		return schema.PowerStyleBar
	}
}

// PinElectrical maps Altium ELECTRICAL values to schema.PinType.
// 0=Input, 1=Bidi, 2=Output, 3=OpenCollector, 4=Passive, 5=HiZ, 6=OpenEmitter, 7=Power.
var pinElectricalTable = [8]schema.PinType{
	schema.PinInput,
	schema.PinBidi,
	schema.PinOutput,
	schema.PinOpenCollector,
	schema.PinPassive,
	schema.PinHiZ,
	schema.PinOpenEmitter,
	schema.PinPower,
}

// PinElectrical returns the canonical PinType for an Altium ELECTRICAL value
// (0–7). Unknown values default to PinPassive.
func PinElectrical(v int) schema.PinType {
	if v >= 0 && v < len(pinElectricalTable) {
		return pinElectricalTable[v]
	}
	return schema.PinPassive
}

// sheetSizeTable maps Altium SHEETSTYLE IDs to (width, height) in mils.
// Source: KiCad developer documentation enumeration tables (documentation, not code).
var sheetSizeTable = []struct {
	std schema.PaperStd
	w   int // mils
	h   int
}{
	{schema.PaperA4, 1150, 760},
	{schema.PaperA3, 1550, 1110},
	{schema.PaperA2, 2230, 1570},
	{schema.PaperA1, 3150, 2230},
	{schema.PaperA0, 4460, 3150},
	{schema.PaperA, 950, 750},
	{schema.PaperB, 1500, 950},
	{schema.PaperC, 2000, 1500},
	{schema.PaperD, 3200, 2000},
	{schema.PaperE, 4200, 3200},
	{schema.PaperLetter, 1100, 850},
	{schema.PaperLegal, 1400, 850},
	{schema.PaperTabloid, 1700, 1100},
	// IDs 13–17 are OrCAD A–E; approximate the standard ANSI sizes.
	{schema.PaperA, 950, 750},
	{schema.PaperB, 1500, 950},
	{schema.PaperC, 2000, 1500},
	{schema.PaperD, 3200, 2000},
	{schema.PaperE, 4200, 3200},
}

// SheetSize returns the Paper for the given Altium SHEETSTYLE value.
// When useCustom is true the provided customX/customY (in mils, with
// fractional parts) override the standard size.
func SheetSize(style int, useCustom bool, customXMils, customXFrac, customYMils, customYFrac int, portrait bool) schema.Paper {
	if useCustom {
		w := MilsToNm(customXMils, customXFrac)
		h := MilsToNm(customYMils, customYFrac)
		return schema.Paper{Std: schema.PaperCustom, Custom: &schema.Size{W: w, H: h}, Portrait: portrait}
	}
	if style >= 0 && style < len(sheetSizeTable) {
		e := sheetSizeTable[style]
		return schema.Paper{Std: e.std, Portrait: portrait}
	}
	// Fallback: A4.
	return schema.Paper{Std: schema.PaperA4, Portrait: portrait}
}

// stdPaperMils maps schema.PaperStd to (width, height) in mils for the
// standard landscape orientation. These are the Altium drawing-area sizes.
var stdPaperMils = map[schema.PaperStd][2]int{
	schema.PaperA4:      {11500, 7600},
	schema.PaperA3:      {15500, 11100},
	schema.PaperA2:      {22300, 15700},
	schema.PaperA1:      {31500, 22300},
	schema.PaperA0:      {44600, 31500},
	schema.PaperA:       {9500, 7500},
	schema.PaperB:       {15000, 9500},
	schema.PaperC:       {20000, 15000},
	schema.PaperD:       {32000, 20000},
	schema.PaperE:       {42000, 32000},
	schema.PaperLetter:  {11000, 8500},
	schema.PaperLegal:   {14000, 8500},
	schema.PaperTabloid: {17000, 11000},
}

// PaperDims returns the sheet width and height in nanometres. Custom sizes are
// returned verbatim; unknown standard sizes fall back to A4.
func PaperDims(p schema.Paper) schema.Size {
	if p.Custom != nil {
		return schema.Size{W: p.Custom.W, H: p.Custom.H}
	}
	if d, ok := stdPaperMils[p.Std]; ok {
		return schema.Size{W: MilsToNm(d[0], 0), H: MilsToNm(d[1], 0)}
	}
	return schema.Size{W: MilsToNm(11500, 0), H: MilsToNm(7600, 0)}
}

// AltiumJustification maps an Altium JUSTIFICATION value (0–8) to schema.Justify.
// Out-of-range values default to BottomLeft (Altium's own default).
func AltiumJustification(v int) schema.Justify {
	if v >= 0 && v <= 8 {
		return schema.Justify(v)
	}
	return schema.JustifyBottomLeft
}

// TextPositioning mirrors KiCad's SetTextPositioning (sch_io_altium.cpp): it maps
// an Altium justification + absolute text orientation to a KiCad text angle and
// alignment. The returned angle is always 0 or 90 (KiCad keeps text upright and
// uses justification, never 180°/270°). The alignment values follow KiCad's
// GR_TEXT_*_ALIGN sign convention:
//
//	h: -1 = left,   0 = center, +1 = right
//	v: -1 = top,    0 = center, +1 = bottom
//
// orientDeg is the absolute text orientation in degrees (0/90/180/270), i.e.
// RIGHTWARDS/UPWARDS/LEFTWARDS/DOWNWARDS.
func TextPositioning(just schema.Justify, orientDeg schema.Angle) (angle, h, v int) {
	switch just {
	case schema.JustifyBottomLeft, schema.JustifyCenterLeft, schema.JustifyTopLeft:
		h = -1
	case schema.JustifyBottomRight, schema.JustifyCenterRight, schema.JustifyTopRight:
		h = +1
	default: // center column
		h = 0
	}

	switch just {
	case schema.JustifyTopLeft, schema.JustifyTopCenter, schema.JustifyTopRight:
		v = -1
	case schema.JustifyCenterLeft, schema.JustifyCenterCenter, schema.JustifyCenterRight:
		v = 0
	default: // bottom row (Altium default)
		v = +1
	}

	switch int(NormalizeAngle(orientDeg) + 0.5) {
	case 90: // UPWARDS
		angle = 90
	case 180: // LEFTWARDS
		h = -h
		angle = 0
	case 270: // DOWNWARDS
		h = -h
		angle = 90
	default: // RIGHTWARDS
		angle = 0
	}
	return angle, h, v
}

// CompensateFieldForInstanceRotation adjusts a field's absolute KiCad text angle
// and justification into the values to STORE on a rotated symbol instance.
//
// KiCad treats a symbol field's stored angle as relative to the instance's
// placement rotation: at render time it adds the instance rotation, keeps the
// text upright (folding 180°/270° back to 0°/90°) and flips the horizontal
// justification when it does. The stored position, by contrast, is absolute.
// So to make a field render at the intended absolute (angle, h, v) on an
// instance placed at instRotDeg, we pre-apply the inverse of that transform.
//
// angle is 0 or 90; h/v use KiCad's signed alignment (h: -1 left/+1 right,
// v: -1 top/+1 bottom). This mirrors AdjustFieldForSymbolOrientation in the
// reference importer (sch_io_altium.cpp), keyed to this emitter's placement
// convention (instance angle == component orientation × 90°).
func CompensateFieldForInstanceRotation(angle, h, v, instRotDeg int) (int, int, int) {
	instRot := (((instRotDeg/90)%4 + 4) % 4) * 90 // normalise to 0/90/180/270

	// Stored angle so that (stored + instRot) is congruent to the desired
	// rendered angle modulo 180 (text stays upright: 0 or 90).
	stored := (((angle-instRot)%180 + 180) % 180)

	// If reaching the desired rendered angle requires a net 180° turn, KiCad's
	// upright fold (point reflection) negates BOTH justification axes, so
	// pre-negate them here to cancel it.
	net := (stored + instRot) % 360
	if (net-angle+360)%360 == 180 {
		h = -h
		v = -v
	}
	return stored, h, v
}

// NormalizeAngle returns angle modulo 360 in [0, 360).
func NormalizeAngle(a schema.Angle) schema.Angle {
	a = a - 360*float64(int(a/360))
	if a < 0 {
		a += 360
	}
	return a
}
