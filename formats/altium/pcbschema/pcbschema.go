// Package pcbschema defines the intermediate representation (IR) for PCB data.
// It is independent of any input or output format.
//
// Canonical units:
//   - Distance: nanometres (int64), via schema.Length.
//   - Angle: degrees, counter-clockwise positive. via schema.Angle.
//   - Y axis: up-positive (flip to Y-down in emitters only).
package pcbschema

import "github.com/rveen/golib/formats/altium/schema"

// Reuse primitive types from the schematic IR so converters work across both.
type (
	Length = schema.Length
	Angle  = schema.Angle
	Point  = schema.Point
	Size   = schema.Size
)

// Board is the root of the PCB IR.
type Board struct {
	Layers       []*Layer
	Nets         []*Net
	Components   []*Component
	Tracks       []*Track
	Vias         []*Via
	Pads         []*Pad
	Arcs         []*Arc
	Fills        []*Fill
	Texts        []*PcbText
	Zones        []*Zone
	Polys        []*Poly      // graphic polygons (non-pour regions)
	CustomPads   []*CustomPad // component-owned copper regions emitted as custom pads
	Keepouts     []*Keepout   // rule-area zones from keepout-layer tracks
	BoardOutline []*Track     // Edge.Cuts segments from BoardRegions
	Meta         Meta
}

// Keepout is a rule-area zone derived from an Altium keepout-layer track. The
// outline is the track's stroke expanded to a closed polygon (a rounded-end
// stadium), in absolute board coordinates. It spans all copper layers.
type Keepout struct {
	Outline []Point
	Prov    schema.Provenance
}

// Meta carries board-level metadata.
type Meta struct {
	SourceFile string
	Thickness  Length // board stackup thickness in nm
}

// Layer is one entry in the resolved KiCad layer table.
type Layer struct {
	AltiumID  int
	KiCadID   int
	KiCadName string
	Type      string // "signal", "user", "mixed", "power"
}

// Net is a named electrical net.
type Net struct {
	Index int // 0-based Altium index; KiCad uses Index+1
	Name  string
}

// Component is a footprint instance placed on the board.
type Component struct {
	Index      int
	Designator string
	Pattern    string // footprint reference (PATTERN key)
	Layer      uint8  // 1 = top, 32 = bottom
	Position   Point
	Rotation   Angle
	Prov       schema.Provenance
}

// Track is a routed copper segment (from Tracks6).
type Track struct {
	Layer     uint8
	Net       uint16 // 0xFFFF = unconnected
	Component uint16 // 0xFFFF = free (board-level)
	Start     Point
	End       Point
	Width     Length
	Prov      schema.Provenance
}

// Via is a drilled inter-layer connection (from Vias6).
type Via struct {
	Net        uint16
	Position   Point
	Diameter   Length
	HoleSize   Length
	StartLayer uint8
	EndLayer   uint8
	Prov       schema.Provenance
}

// PadShape enumerates pad copper shapes.
type PadShape uint8

const (
	PadShapeUnknown   PadShape = 0
	PadShapeCircle    PadShape = 1
	PadShapeRect      PadShape = 2
	PadShapeOctagonal PadShape = 3
	PadShapeRounded   PadShape = 9
)

// Pad is a component pad or through-hole (from Pads6).
type Pad struct {
	Designator string
	Layer      uint8
	Net        uint16
	Component  uint16
	Position   Point
	TopSize    Size
	MidSize    Size
	BotSize    Size
	HoleSize   Length
	TopShape   PadShape
	BotShape   PadShape
	Rotation   Angle
	Plated     bool
	// AltShape == PadShapeRounded promotes a circle pad to a KiCad roundrect with
	// corner ratio CornerRadius/200 (top layer).
	AltShape     PadShape
	CornerRadius uint8
	Prov         schema.Provenance
}

// Arc is a copper arc (from Arcs6).
type Arc struct {
	Layer      uint8
	Net        uint16
	Component  uint16
	Center     Point
	Radius     Length
	StartAngle Angle
	EndAngle   Angle
	Width      Length
	Prov       schema.Provenance
}

// Fill is a rectangular copper fill (from Fills6).
type Fill struct {
	Layer     uint8
	Net       uint16
	Component uint16
	Pos1      Point
	Pos2      Point
	Rotation  Angle
	Prov      schema.Provenance
}

// PcbText is a text object on the board (from Texts6).
type PcbText struct {
	Layer        uint8
	Component    uint16
	Position     Point
	Height       Length
	StrokeWidth  Length
	Rotation     Angle
	Mirrored     bool
	IsComment    bool
	IsDesignator bool
	Text         string
	Prov         schema.Provenance
}

// Poly is a graphic polygon on a non-pour layer (from Regions6/ShapeBasedRegions6
// that are not copper pours, board cutouts, or keepouts). Emitted as gr_poly
// (board-level) or fp_poly (component-level).
type Poly struct {
	Layer     uint8
	Component uint16 // 0xFFFF = board-level
	Vertices  []Point
	Width     Length
	Filled    bool
	Prov      schema.Provenance
}

// CustomPad is a component-owned copper region (from ShapeBasedRegions6) emitted
// as a KiCad custom pad: a tiny circle anchor plus a filled polygon primitive.
// The outline is in absolute board coordinates; arc entries carry the arc's
// start/mid/end points so the emitter can write a KiCad (arc ...) primitive.
type CustomPad struct {
	Component uint16
	Net       uint16
	Layer     uint8 // Altium copper layer: 1 = top, 32 = bottom
	Anchor    Point // absolute position of the first outline vertex
	Outline   []PadOutlineEntry
	Prov      schema.Provenance
}

// PadOutlineEntry is one entry of a custom-pad outline. A straight entry adds the
// point Pt; an arc entry draws a circular arc from Pt through Mid to End.
type PadOutlineEntry struct {
	IsArc bool
	Pt    Point // straight vertex, or arc start
	Mid   Point // arc midpoint (valid when IsArc)
	End   Point // arc endpoint (valid when IsArc)
}

// Zone is a copper pour region (from Polygons6 text records).
type Zone struct {
	Layer      string
	Net        int
	NetName    string
	Vertices   []Point
	Fills      []ZoneFill // pre-computed copper fill polygons from Regions6/ShapeBasedRegions6
	Priority   int        // from POURINDEX (higher = higher priority in KiCad)
	HatchStyle string     // "Solid", "45Degree", "90Degree", "Horizontal", "Vertical", "None", ""
	HatchGap   Length     // spacing between hatch lines (nm)
	TrackWidth Length     // hatch line width (nm)
	Prov       schema.Provenance
}

// ZoneFill is one filled-copper sub-polygon belonging to a Zone.
type ZoneFill struct {
	Vertices []Point
	Holes    [][]Point // cutout holes within this fill polygon (islands)
}
