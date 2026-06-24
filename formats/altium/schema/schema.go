// Package schema defines the generic intermediate representation (IR) for
// schematic data. It is independent of any input or output format — nothing
// here imports altium or emit packages.
//
// Canonical units:
//   - Distance: nanometres (int64).
//   - Angle: degrees, counter-clockwise positive, in [0, 360).
//   - Y axis: up-positive (flip to down-positive in the KiCad emitter only).
//   - Colour: RGBA (from Altium BGR via convert.BGRToColor).
package schema

// Length is a distance in nanometres.
type Length = int64

// Angle is degrees, counter-clockwise positive, normalised to [0, 360).
type Angle = float64

// Point is a 2-D coordinate in nanometres.
type Point struct{ X, Y Length }

// Size is a width×height in nanometres.
type Size struct{ W, H Length }

// RectBox is an axis-aligned bounding box.
type RectBox struct{ Min, Max Point }

// Provenance records where an element originated, for warnings and debugging.
type Provenance struct {
	Sheet  string
	Record int // INDEXINSHEET, or stream offset for binary records
	Kind   string
}

// SymbolID is a stable hash of the canonical symbol definition. Equal content
// across different components in the same schematic shares one Symbol.
type SymbolID string

// ---------- Top-level schematic ----------

// Schematic is the root of the IR.
type Schematic struct {
	Sheets  []*Sheet
	Symbols map[SymbolID]*Symbol // deduplicated definitions
	Meta    Meta
}

// Meta carries schematic-level metadata.
type Meta struct {
	SourceFile string
	Tool       string // e.g. "Altium Designer"
	Raw        map[string]string
}

// ---------- Sheet ----------

// Sheet is one schematic page.
type Sheet struct {
	Name       string
	FileName   string
	Paper      Paper
	Fonts      []Font // sheet font table; a FontRef of n indexes Fonts[n-1]
	Components []*Component
	Wires      []*Wire
	Buses      []*Bus
	Junctions  []Point
	NetLabels  []*NetLabel
	PowerPorts []*PowerPort
	Ports      []*Port
	SubSheets  []*SheetSymbol
	Graphics   []Graphic // free graphics not owned by a component
	Texts      []*Text
	Params     []Field
	Prov       Provenance
}

// Font is one entry of the sheet font table (section 9.2). Height is the em
// height in nanometres — the Altium line-spacing SIZEn scaled by 0.875.
type Font struct {
	Name      string
	Height    Length
	Bold      bool
	Italic    bool
	Underline bool
	Rotation  Angle
}

// DefaultFontHeight is used when a text element has no resolvable font.
const DefaultFontHeight Length = 1_270_000 // 1.27 mm (50 mil)

// FontHeight returns the em height for a font reference, falling back to
// DefaultFontHeight when the reference is unset or out of range.
func (s *Sheet) FontHeight(ref FontRef) Length {
	if int(ref) >= 1 && int(ref) <= len(s.Fonts) {
		if h := s.Fonts[ref-1].Height; h > 0 {
			return h
		}
	}
	return DefaultFontHeight
}

// ---------- Component and Symbol ----------

// Component is an instance of a symbol placed on a sheet.
type Component struct {
	Symbol         SymbolID
	Designator     string
	DesignatorFont FontRef
	DesignatorPos  Point   // label position in component-local frame (de-rotated)
	DesignatorRot  Angle   // absolute text orientation in degrees (0/90/180/270)
	DesignatorJust Justify // designator text anchor justification
	Position       Point
	Rotation       Angle
	Mirrored       bool
	Unit           int // 1-based; multi-part symbols
	BodyStyle      int // 1 = normal, 2 = De Morgan
	Fields         []Field
	Prov           Provenance
}

// ValueField returns a pointer to the component's value/comment field (the
// "Value" or "Comment" parameter), or nil if none exists. The returned pointer
// is into the Fields slice and is valid for the lifetime of the component.
func (c *Component) ValueField() *Field {
	for i := range c.Fields {
		if equalFold(c.Fields[i].Name, "value") || equalFold(c.Fields[i].Name, "comment") {
			return &c.Fields[i]
		}
	}
	return nil
}

// Value returns the component's value/comment text, its font reference, and
// whether such a field exists. Callers that also need the label position should
// use ValueField instead.
func (c *Component) Value() (text string, font FontRef, ok bool) {
	if f := c.ValueField(); f != nil {
		return f.Value, f.Font, true
	}
	return "", 0, false
}

// equalFold reports whether a and b are equal under simple ASCII case folding.
// Kept local so the schema package stays dependency-free.
func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if 'A' <= ca && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if 'A' <= cb && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}

// Symbol is a deduplicated definition. Geometry is in the symbol's local
// frame with the origin at the component anchor point.
type Symbol struct {
	ID         SymbolID
	LibRef     string
	UnitCount  int
	BodyStyles int
	Pins       []*Pin
	Graphics   []Graphic
	Prov       Provenance
}

// Pin is a connection point of a symbol.
type Pin struct {
	Name          string
	Number        string
	Position      Point // relative to symbol origin
	PinLength     Length
	Orientation   Dir4
	Electrical    PinType
	Shape         PinShape
	NameVisible   bool
	NumberVisible bool
	Hidden        bool
	Unit          int
	Prov          Provenance
}

// Field is any named property: designator, value, footprint link, custom param.
type Field struct {
	Name    string
	Value   string
	Visible bool
	Pos     Point
	Rot     Angle   // absolute text orientation in degrees (0/90/180/270)
	Just    Justify // text anchor justification
	Font    FontRef
}

// ---------- Connectivity geometry ----------

// Wire is a net segment.
type Wire struct {
	Points []Point
	Prov   Provenance
}

// Bus is a bus segment.
type Bus struct {
	Points []Point
	Prov   Provenance
}

// NetLabel labels a wire with a net name.
type NetLabel struct {
	Text string
	Pos  Point
	Rot  Angle   // absolute text orientation in degrees (0/90/180/270)
	Just Justify // text anchor justification
	Font FontRef
	Prov Provenance
}

// PowerPort is a power/ground symbol that labels a net.
type PowerPort struct {
	NetName     string
	Style       PowerStyle
	ShowNetName bool
	Pos         Point
	Rot         Angle
	Font        FontRef
	Prov        Provenance
}

// Port is a hierarchical inter-sheet connector.
type Port struct {
	Name      string
	Direction PortDir
	Pos       Point
	Prov      Provenance
}

// SheetSymbol is a box on the parent sheet that references a child sheet.
type SheetSymbol struct {
	FileName string
	Name     string
	Box      RectBox
	Entries  []SheetEntry
	Prov     Provenance
}

// SheetEntry is a port on a SheetSymbol.
type SheetEntry struct {
	Name      string
	Direction PortDir
	Pos       Point
}

// Text is a free-standing text annotation on a sheet.
type Text struct {
	Pos     Point
	Content string
	Font    FontRef
	Just    Justify
	Rot     Angle
	Prov    Provenance
}

// ---------- Graphics ----------

// Graphic is implemented by all drawable primitives.
type Graphic interface{ graphic() }

// Color is an RGBA colour value.
type Color struct{ R, G, B, A uint8 }

// Stroke describes a line style.
type Stroke struct {
	Width Length
	Color Color
}

// Concrete graphic types. Each implements Graphic.

type Line struct {
	A, B  Point
	Style Stroke
}

type Rect struct {
	Box   RectBox
	Style Stroke
	Fill  *Color
}

type RoundRect struct {
	Box    RectBox
	Radius Length
	Style  Stroke
	Fill   *Color
}

type Arc struct {
	Center       Point
	Radius       Length
	Start, End   Angle
	Style        Stroke
}

type EllArc struct {
	Center     Point
	RX, RY     Length
	Start, End Angle
	Style      Stroke
}

type Ellipse struct {
	Center Point
	RX, RY Length
	Style  Stroke
	Fill   *Color
}

type Polyline struct {
	Points []Point
	Style  Stroke
}

type Polygon struct {
	Points []Point
	Style  Stroke
	Fill   *Color
}

type Bezier struct {
	Points []Point
	Style  Stroke
}

type Image struct {
	Box RectBox
	Ref string // key into embedded storage
}

func (Line) graphic()      {}
func (Rect) graphic()      {}
func (RoundRect) graphic() {}
func (Arc) graphic()       {}
func (EllArc) graphic()    {}
func (Ellipse) graphic()   {}
func (Polyline) graphic()  {}
func (Polygon) graphic()   {}
func (Bezier) graphic()    {}
func (Image) graphic()     {}

// ---------- Enumerations ----------

// Dir4 is a cardinal direction, used for pin orientation.
type Dir4 int

const (
	DirRight Dir4 = iota // 0 — pin points right (stub extends left)
	DirUp                // 1
	DirLeft              // 2
	DirDown              // 3
)

// PinType is the electrical function of a pin.
type PinType int

const (
	PinInput        PinType = iota // 0
	PinBidi                        // 1
	PinOutput                      // 2
	PinOpenCollector               // 3
	PinPassive                     // 4
	PinHiZ                         // 5
	PinOpenEmitter                 // 6
	PinPower                       // 7
)

// PinShape is the graphical symbol drawn at the pin's connection end.
type PinShape int

const (
	PinShapeNone           PinShape = iota
	PinShapeInverted                // bubble
	PinShapeClk                     // clock
	PinShapeInvertedClk             // inverted clock
	PinShapeInputLow                // active-low input line
	PinShapeOutputLow               // active-low output line
	PinShapeAnalog                  // no special marker
)

// PowerStyle is the graphical variant of a power port.
type PowerStyle int

const (
	PowerStyleBar     PowerStyle = iota
	PowerStyleGND
	PowerStyleEarth
	PowerStyleArrow
	PowerStyleTee
	PowerStyleWaveLine
	PowerStyleRailPosNeg
)

// PortDir is the data-flow direction of a hierarchical port.
type PortDir int

const (
	PortUnspecified PortDir = iota
	PortInput
	PortOutput
	PortBidi
)

// Justify is text alignment, encoded to match Altium's JUSTIFICATION values
// (0–8): a vertical band (bottom/center/top) crossed with a horizontal band
// (left/center/right). Altium's default is BottomLeft (0).
type Justify int

const (
	JustifyBottomLeft   Justify = iota // 0
	JustifyBottomCenter                // 1
	JustifyBottomRight                 // 2
	JustifyCenterLeft                  // 3
	JustifyCenterCenter                // 4
	JustifyCenterRight                 // 5
	JustifyTopLeft                     // 6
	JustifyTopCenter                   // 7
	JustifyTopRight                    // 8
)

// Paper describes the sheet paper size.
type Paper struct {
	Std      PaperStd
	Custom   *Size // non-nil when Std == PaperCustom
	Portrait bool
}

// PaperStd is a standard paper size identifier.
type PaperStd int

const (
	PaperA4 PaperStd = iota
	PaperA3
	PaperA2
	PaperA1
	PaperA0
	PaperA
	PaperB
	PaperC
	PaperD
	PaperE
	PaperLetter
	PaperLegal
	PaperTabloid
	PaperCustom
)

// FontRef identifies a font by index (1-based, matching the sheet font table).
type FontRef int
