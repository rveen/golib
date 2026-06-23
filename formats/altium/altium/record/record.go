// Package record defines the raw Altium schematic record type produced by the reader.
// Each Record is a property map decoded from one entry in a FileHeader stream.
package record

import "strconv"

// Record is a single decoded Altium schematic object.
type Record struct {
	Type  int               // value of the RECORD property
	Props map[string]string // all key/value pairs, keys uppercased
	Index int               // INDEXINSHEET (-1 if absent)
}

// Str returns the property value for key, or "" if absent.
func (r Record) Str(key string) string {
	return r.Props[key]
}

// Int returns the property value parsed as int. ok is false if the key is
// absent or the value is not a valid integer.
func (r Record) Int(key string) (int, bool) {
	s, ok := r.Props[key]
	if !ok {
		return 0, false
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0, false
	}
	return v, true
}

// IntDef returns the property parsed as int, or def if absent or unparseable.
func (r Record) IntDef(key string, def int) int {
	v, ok := r.Int(key)
	if !ok {
		return def
	}
	return v
}

// Bool returns true if the property equals "T" or "TRUE" (Altium boolean true).
// False is usually written by omitting the key, not by writing "F".
func (r Record) Bool(key string) bool {
	v := r.Props[key]
	return v == "T" || v == "TRUE"
}

// UTF8Str returns the value for key, preferring the %UTF8%KEY variant when
// present (section 4.1 of the format spec).
func (r Record) UTF8Str(key string) string {
	if v, ok := r.Props["%UTF8%"+key]; ok {
		return v
	}
	return r.Props[key]
}

// Record type constants, table-driven from the Altium schematic format specification.
// Source: KiCad developer documentation enumeration tables (documentation, not code).
const (
	TypeHeader           = 0
	TypeComponent        = 1
	TypePin              = 2
	TypeIEEESymbol       = 3  // skip
	TypeLabel            = 4
	TypeBezier           = 5
	TypePolyline         = 6
	TypePolygon          = 7
	TypeEllipse          = 8
	TypePieChart         = 9
	TypeRoundRectangle   = 10
	TypeEllipticalArc    = 11
	TypeArc              = 12
	TypeLine             = 13
	TypeRectangle        = 14
	TypeSheetSymbol      = 15
	TypeSheetEntry       = 16
	TypePowerPort        = 17
	TypePort             = 18
	TypeNoERC            = 22
	TypeNetLabel         = 25
	TypeBus              = 26
	TypeWire             = 27
	TypeTextFrame        = 28
	TypeJunction         = 29
	TypeImage            = 30
	TypeSheet            = 31
	TypeSheetName        = 32
	TypeFileName         = 33
	TypeDesignator       = 34
	TypeBusEntry         = 37
	TypeTemplate         = 39
	TypeParameter        = 41
	TypeParameterSet     = 43 // skip
	TypeImplementList    = 44
	TypeImplementation   = 45 // footprint link
	TypeMapDefinerList   = 46 // skip
	TypeMapDefiner       = 47 // skip
	TypeImplParams       = 48 // skip
	TypeNote             = 209
	TypeCompileMask      = 211 // skip
	TypeHarnessConnector = 215
	TypeHarnessEntry     = 216
	TypeHarnessType      = 217
	TypeSignalHarness    = 218
	TypeBlanket          = 225 // skip
	TypeHyperlink        = 226
)

// TypeName maps a record type ID to a human-readable name.
// IDs not in this map are undocumented or unassigned.
var TypeName = map[int]string{
	TypeHeader:           "HEADER",
	TypeComponent:        "COMPONENT",
	TypePin:              "PIN",
	TypeIEEESymbol:       "IEEE_SYMBOL",
	TypeLabel:            "LABEL",
	TypeBezier:           "BEZIER",
	TypePolyline:         "POLYLINE",
	TypePolygon:          "POLYGON",
	TypeEllipse:          "ELLIPSE",
	TypePieChart:         "PIE_CHART",
	TypeRoundRectangle:   "ROUND_RECTANGLE",
	TypeEllipticalArc:    "ELLIPTICAL_ARC",
	TypeArc:              "ARC",
	TypeLine:             "LINE",
	TypeRectangle:        "RECTANGLE",
	TypeSheetSymbol:      "SHEET_SYMBOL",
	TypeSheetEntry:       "SHEET_ENTRY",
	TypePowerPort:        "POWER_PORT",
	TypePort:             "PORT",
	TypeNoERC:            "NO_ERC",
	TypeNetLabel:         "NET_LABEL",
	TypeBus:              "BUS",
	TypeWire:             "WIRE",
	TypeTextFrame:        "TEXT_FRAME",
	TypeJunction:         "JUNCTION",
	TypeImage:            "IMAGE",
	TypeSheet:            "SHEET",
	TypeSheetName:        "SHEET_NAME",
	TypeFileName:         "FILE_NAME",
	TypeDesignator:       "DESIGNATOR",
	TypeBusEntry:         "BUS_ENTRY",
	TypeTemplate:         "TEMPLATE",
	TypeParameter:        "PARAMETER",
	TypeParameterSet:     "PARAMETER_SET",
	TypeImplementList:    "IMPLEMENTATION_LIST",
	TypeImplementation:   "IMPLEMENTATION",
	TypeMapDefinerList:   "MAP_DEFINER_LIST",
	TypeMapDefiner:       "MAP_DEFINER",
	TypeImplParams:       "IMPL_PARAMS",
	TypeNote:             "NOTE",
	TypeCompileMask:      "COMPILE_MASK",
	TypeHarnessConnector: "HARNESS_CONNECTOR",
	TypeHarnessEntry:     "HARNESS_ENTRY",
	TypeHarnessType:      "HARNESS_TYPE",
	TypeSignalHarness:    "SIGNAL_HARNESS",
	TypeBlanket:          "BLANKET",
	TypeHyperlink:        "HYPERLINK",
}

// SkipTypes are record types that are explicitly not imported; they still get
// a Report warning rather than a silent drop.
var SkipTypes = map[int]bool{
	TypeIEEESymbol:   true,
	TypeParameterSet: true,
	TypeMapDefinerList: true,
	TypeMapDefiner:   true,
	TypeImplParams:   true,
	TypeCompileMask:  true,
	TypeBlanket:      true,
}
