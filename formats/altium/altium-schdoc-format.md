# Altium SchDoc binary format specification

Reverse-engineered description of the Altium Designer schematic document
(`.SchDoc`) binary format. This is not an official Altium document. It is
reconstructed from independent implementations and is sufficient to read
existing files; it is incomplete and some fields are unconfirmed.

## 1. Scope and sources

This document describes the on-disk layout of `.SchDoc` files and, where they
share the same structure, schematic library files (`.SchLib`). PCB documents
(`.PcbDoc`) use a related but different record model and are out of scope.

Primary sources, both independently reverse-engineered:

- KiCad Altium schematic importer, `eeschema/sch_io/altium/altium_parser_sch.{h,cpp}`
  and `common/io/altium/altium_binary_parser.cpp`, `altium_props_utils.cpp`.
  This is the most actively maintained reader and is the authority for exact
  property keys, value decoding and enumerations used below.
- `vadmium/python-altium`, `format.md`. The canonical community reference for
  the container and record-header layout. KiCad's source cites it for sheet
  sizes.

Where the two disagree, the disagreement is noted. Property keys are quoted
exactly as they appear after canonicalization (see section 4).

## 2. Container: OLE / compound file binary

A `.SchDoc` file is an OLE Compound File (Microsoft CFB, also called a
"compound document" or "structured storage"). The CFB layer provides a
FAT-like directory of named streams and storages inside a single file. Any
CFB library can enumerate it; the schematic semantics live entirely in the
stream contents, not in the CFB metadata.

The root storage of a `.SchDoc` exposes up to three streams:

| Stream       | Required | Contents                                              |
|--------------|----------|-------------------------------------------------------|
| `FileHeader` | yes      | The schematic itself: a sequence of object records.   |
| `Storage`    | no       | Embedded binary files (e.g. images), zlib-compressed. |
| `Additional` | no       | Extra records that logically append to `FileHeader`.  |

The `Additional` stream, when present, has the same record format as
`FileHeader`. Its records continue the same index sequence and belong after
the `FileHeader` records. Records seen only there include harness objects and
record types 215–218.

Schematic library files (`.SchLib`) differ at the container level: each symbol
is a child storage under the root, and each such storage holds a `Data` stream
(the record sequence for that symbol) plus optional `PinFrac`, `PinWideText`
and `PinTextData` streams carrying high-precision pin coordinates and wide-text
pin data. The record format inside `Data` is identical to `FileHeader`.

Integrated-library member streams may be individually compressed. The first
byte of such a stream is a tag: `0x00` means the remaining bytes are the raw
stream; `0x02` means the remaining bytes are a zlib stream (standard zlib
header) to be inflated before parsing.

## 3. Record framing

Each stream is a flat sequence of records. A record begins with a 4-byte
header, little-endian:

```
offset 0  uint32  header word
              bits 0..23  (mask 0x00FFFFFF)  payload length in bytes
              bits 24..31 (mask 0xFF000000)  flags / payload type; 0x00 = text
offset 4  ...      payload (length bytes)
```

The community reference describes the same four bytes as: 2-byte little-endian
length, one `0x00` byte, one record-type byte. The two descriptions are
equivalent for the common case because payloads are well under 16 MiB, so the
top byte of the length word is otherwise zero and doubles as the type/flags
byte.

Payload interpretation by the top byte:

- `0x00` — text payload: a property list (section 4), conventionally
  terminated by a single `\0` byte that is included in the declared length.
  A trailing null is expected but not guaranteed; at least one known file
  omits the final byte. Readers should tolerate its absence.
- nonzero — binary payload. Used in the `Storage` stream for embedded files
  (section 11) and for the per-symbol pin auxiliary streams in `.SchLib`.

The first record of every stream is the header record (section 7.1). It is a
property list with no `RECORD` key.

## 4. Property list grammar

A text payload is a flat, unordered list of key/value pairs:

```
|KEY1=value1|KEY2=value2|...|KEYn=valuen
```

Rules, as implemented by the reference readers:

- Pairs are separated by `|` (U+007C). Each pair is `KEY=VALUE`. The value
  runs to the next `|` or to end of payload.
- The leading `|` is conventional but may be missing in older files; readers
  locate the first key by comparing the position of the first `|` and the
  first `=`.
- Keys are case-insensitive. Readers canonicalize by trimming surrounding
  whitespace and upper-casing. Altium writes keys either fully upper-case or
  in CamelCase; after canonicalization both forms collide intentionally.
- Values are not trimmed of internal content but are right/left trimmed of
  surrounding spaces by the reference reader.
- A key may be repeated; behavior on duplicates is reader-defined (KiCad keeps
  the first inserted).
- Values are positional only through ordered families (see "lists" in
  section 5); there is otherwise no ordering guarantee.

### 4.1 Character encoding

Default value encoding is a single-byte Windows code page, effectively
Latin-1 / ISO-8859-1 (CP-1252 in practice for punctuation). Two mechanisms
carry full Unicode:

- `%UTF8%`-prefixed keys. For a logical property `NAME`, a parallel key
  `%UTF8%NAME` may be present whose value is UTF-8 encoded. After
  canonicalization the prefix is preserved as `%UTF8%NAME`. A reader that
  wants Unicode looks up `%UTF8%KEY` first and falls back to `KEY`.
- A `UNICODE` flag plus `UNICODE__KEY` entries. When a `UNICODE` property
  contains the token `EXISTS`, a value `UNICODE__KEY` holds a comma-separated
  list of decimal UTF-16 code units, each reassembled into a character.

A reader processing Latin-1 values should treat the byte `0xFF` (rendered `ÿ`)
as a space for most keys; Altium renders it as whitespace. The exceptions in
the reference reader are `PATTERN` and `SOURCEFOOTPRINTLIBRARY`.

## 5. Data types and encodings

All values are ASCII text inside the property list; the "type" is a decoding
convention keyed by property name.

### 5.1 Integers

Decimal, optionally signed: `RECORD=31`, `OWNERPARTID=-1`. A missing integer
property defaults to 0 (some records use −1 as the documented default for
owner indices and part IDs).

### 5.2 Bitfields

Integers whose bits are flags. Example `PINCONGLOMERATE` (section 9.5).

### 5.3 Colours

24-bit RGB packed into a decimal integer, Delphi `TColor` order
(blue is the high byte):

```
bits 0..7   (0x0000FF)  red
bits 8..15  (0x00FF00)  green
bits 16..23 (0xFF0000)  blue
```

Example: `COLOR=8388608` = 0x800000 = pure blue channel = RGB #000080.

### 5.4 Real numbers

Decimal with fractional part, locale-independent (`.` decimal separator),
typically three places: `ENDANGLE=360.000`. Used for angles. Omitted when zero.

### 5.5 Booleans

`T` or `TRUE` is true; anything else (or absence) is false. False is usually
written by omitting the property rather than `F`.

### 5.6 Coordinates, sizes and the `_FRAC` mechanism

The coordinate origin is the bottom-left of the sheet; y increases upward.
(KiCad negates y on import because its origin is top-left.)

The base unit of a coordinate or size is 1/100 inch = 10 mil = 0.254 mm.
A value of `LOCATION.X=100` therefore means 1000 mil = 1 inch.

Sub-unit precision is carried by an optional companion property with a `_FRAC`
suffix, measured in units of 1/100000 of the base unit:

```
value_in_base_units = INT(KEY) + INT(KEY_FRAC) / 100000
physical = value_in_base_units * 10 mil
         = value_in_base_units * 0.254 mm
```

Example: `LOCATION.X=200|LOCATION.X_FRAC=50000` = 200.5 base units
= 2005 mil = 2.005 inch.

A second, rarer variant uses a `_FRAC1` suffix. There the base integer is
itself scaled by 10 before combining (i.e. the base integer is in mil, not in
10-mil units). It is used for `DISTANCEFROMTOP` on sheet entries and harness
entries. Treat `_FRAC1` families as a distinct decoding path.

Points are written as a `.X`/`.Y` pair: `LOCATION.X`, `LOCATION.Y`,
`CORNER.X`, `CORNER.Y`.

### 5.7 Coordinate lists and value families

Variable-length data is encoded as a count property plus indexed members,
indices starting at 1:

- Point list: `LOCATIONCOUNT=2|X1=100|Y1=100|X2=200|Y2=100`.
- Font table: `FONTIDCOUNT=2|SIZE1=10|FONTNAME1=...|SIZE2=10|FONTNAME2=...`.

Each member coordinate (`Xn`, `Yn`) may carry its own `_FRAC` companion.

### 5.8 Line width

`LINEWIDTH` is encoded with the standard coordinate/`_FRAC` mechanism. A
missing or zero width denotes a hairline. Older files use a small enumerated
form (omitted = 4 mil, 1 = 10 mil, 2 = 20 mil, 3 = 40 mil); the reference
reader maps a decoded width of 0 to a 1-mil minimum.

## 6. Object model and ownership

The `FileHeader` stream is an ordered list of objects:

1. The header object (no `RECORD` key).
2. The sheet object (`RECORD=31`) at index 0, carrying document-wide settings.
3. All other objects, indexed from 0 in stream order, in depth-first order of
   their ownership tree.

Objects form a tree through ownership properties. The common ownership
interface (present on most non-trivial records) is:

| Property               | Type    | Meaning                                                                 |
|------------------------|---------|-------------------------------------------------------------------------|
| `OWNERINDEX`           | integer | Stream index of the owning object; −1 (or absent) if top-level.         |
| `OWNERPARTID`          | integer | Which part of a multi-part component this object belongs to; −1 = all.  |
| `OWNERPARTDISPLAYMODE` | integer | Which alternate display mode this object belongs to.                    |
| `INDEXINSHEET`         | integer | Object's own index hint within the sheet; −1 / 0 common defaults.       |
| `ISNOTACCESIBLE`       | boolean | Object is part of a symbol body (not independently selectable).         |

Note the misspelling `ISNOTACCESIBLE` is in the file format itself.

Part and display-mode filtering: a component (`RECORD=1`) declares
`PARTCOUNT`, `DISPLAYMODECOUNT`, `CURRENTPARTID` and `DISPLAYMODE`. A child
object is shown only when its `OWNERPARTID` equals the component's
`CURRENTPARTID` (or is −1, meaning "all parts") and its `OWNERPARTDISPLAYMODE`
matches the active display mode. Ignoring this duplicates every part of a
multi-part component (e.g. a quad op-amp drawn four times).

Some harness records do not reliably carry `OWNERINDEX`; readers reconstruct
the parent from stream position instead.

## 7. Record type registry

`RECORD` integer to object type. Gaps are unassigned or unobserved.

| RECORD | Object                | RECORD | Object                       |
|--------|-----------------------|--------|------------------------------|
| (none) | Header                | 28     | Text frame                   |
| 0      | Header (implied)      | 29     | Junction                     |
| 1      | Component             | 30     | Image                        |
| 2      | Pin                   | 31     | Sheet (document settings)    |
| 3      | IEEE symbol           | 32     | Sheet name                   |
| 4      | Label                 | 33     | File name                    |
| 5      | Bezier                | 34     | Designator                   |
| 6      | Polyline              | 37     | Bus entry                    |
| 7      | Polygon               | 39     | Template                     |
| 8      | Ellipse               | 41     | Parameter                    |
| 9      | Piechart              | 43     | Parameter set / warning sign |
| 10     | Round rectangle       | 44     | Implementation list          |
| 11     | Elliptical arc        | 45     | Implementation               |
| 12     | Arc                   | 46     | Map definer list             |
| 13     | Line                  | 47     | Map definer                  |
| 14     | Rectangle             | 48     | Implementation parameters    |
| 15     | Sheet symbol          | 209    | Note                         |
| 16     | Sheet entry           | 211    | Compile mask                 |
| 17     | Power port            | 215    | Harness connector            |
| 18     | Port                  | 216    | Harness entry                |
| 22     | No-ERC marker         | 217    | Harness type                 |
| 25     | Net label             | 218    | Signal harness               |
| 26     | Bus                   | 225    | Blanket                      |
| 27     | Wire                  | 226    | Hyperlink                    |

## 8. Shared field interfaces

Several decoding mixins recur. They are documented once here and referenced
from the per-record tables.

Owner interface — `OWNERINDEX`, `OWNERPARTID`, `OWNERPARTDISPLAYMODE`,
`INDEXINSHEET`, `ISNOTACCESIBLE` (section 6).

Fill interface:

| Property      | Type    | Meaning                          |
|---------------|---------|----------------------------------|
| `AREACOLOR`   | colour  | Fill colour.                     |
| `ISSOLID`     | boolean | Filled if true.                  |
| `TRANSPARENT` | boolean | Fill is transparent.             |

Border interface:

| Property    | Type   | Meaning                                              |
|-------------|--------|------------------------------------------------------|
| `LINEWIDTH` | length | Border width; 0 = hairline (treated as 1-mil min).   |
| `COLOR`     | colour | Border / line colour.                                |

## 9. Per-record field reference

Only the more load-bearing fields are listed. Records inherit the interfaces
named in their heading. All coordinates follow section 5.6.

### 9.1 Header (`RECORD` absent)

| Property       | Type    | Meaning                                                        |
|----------------|---------|----------------------------------------------------------------|
| `HEADER`       | string  | Fixed signature, e.g. `Protel for Windows - Schematic Capture Binary File Version 5.0`. |
| `WEIGHT`       | integer | Number of remaining objects in the stream.                     |
| `MINORVERSION` | integer | Optional.                                                      |
| `UNIQUEID`     | string  | Optional document id.                                          |

The `Storage` stream's header record uses `HEADER=Icon storage`.

### 9.2 Sheet (`RECORD=31`) — document settings, index 0

| Property                | Type    | Meaning                                                   |
|-------------------------|---------|-----------------------------------------------------------|
| `FONTIDCOUNT`           | integer | Size of the font table; members `SIZEn`, `FONTNAMEn`, `ITALICn`, `BOLDn`, `UNDERLINEn`, `ROTATIONn`, `AREACOLORn`. `FONTID` on other records indexes this table (1-based). |
| `SHEETSTYLE`            | enum    | Predefined paper size (Appendix A.6). Default A4.         |
| `WORKSPACEORIENTATION`  | enum    | 0 landscape, 1 portrait.                                  |
| `USECUSTOMSHEET`        | boolean | If true, use `CUSTOMX`/`CUSTOMY` instead of `SHEETSTYLE`. |
| `CUSTOMX`, `CUSTOMY`    | length  | Custom drawing-area size.                                 |
| `SYSTEMFONT`            | integer | Default font index (normally 1).                          |
| `AREACOLOR`             | colour  | Drawing-area background.                                  |
| `BORDERON`, `TITLEBLOCKON`, `SNAPGRIDON`, `VISIBLEGRIDON` | boolean | Display toggles. |
| `SNAPGRIDSIZE`, `VISIBLEGRIDSIZE` | length | Grid spacings.                              |
| `DISPLAY_UNIT`          | integer | Display unit (1 mm, 4 = 10-mil units, etc.).              |
| `TEMPLATEFILENAME`      | string  | Optional sheet template path.                             |

A font-table member (per index n) decodes to: `FONTNAMEn` (string),
`SIZEn` (length, line spacing — em size is roughly 0.875× this),
`ROTATIONn` (0 or 90), `ITALICn`/`BOLDn`/`UNDERLINEn` (boolean),
`AREACOLORn` (colour).

### 9.3 Component (`RECORD=1`) — owner interface

| Property               | Type    | Meaning                                                       |
|------------------------|---------|---------------------------------------------------------------|
| `LIBREFERENCE`         | string  | Symbol name in the source library.                            |
| `SOURCELIBRARYNAME`    | string  | Source library.                                               |
| `COMPONENTDESCRIPTION` | string  | Description (often paired with `%UTF8%COMPONENTDESCRIPTION`). |
| `UNIQUEID`             | string  | Component instance id.                                        |
| `CURRENTPARTID`        | integer | Active part; child objects with a different non-(−1) `OWNERPARTID` are not drawn. |
| `PARTCOUNT`            | integer | Part count + 1 (a normal single-part component has 2).        |
| `DISPLAYMODECOUNT`     | integer | Number of alternate symbols.                                  |
| `DISPLAYMODE`          | integer | Active alternate symbol (may be a string in some files).      |
| `LOCATION.X/.Y`        | point   | Placement.                                                    |
| `ORIENTATION`          | enum    | Rotation (Appendix A.1).                                      |
| `ISMIRRORED`           | boolean | Mirrored placement.                                           |
| `AREACOLOR`, `COLOR`   | colour  | Body fill and outline defaults.                               |

### 9.4 Pin (`RECORD=2`) — owner interface

| Property            | Type    | Meaning                                                       |
|---------------------|---------|---------------------------------------------------------------|
| `NAME`              | string  | Pin function name (shown inside the body).                    |
| `DESIGNATOR`        | string  | Pin number (shown outside the body).                          |
| `TEXT`              | string  | Auxiliary text.                                               |
| `ELECTRICAL`        | enum    | Electrical type (Appendix A.3).                               |
| `PINLENGTH`         | length  | Pin line length.                                              |
| `LOCATION.X/.Y`     | point   | Point where the pin attaches to the body.                     |
| `PINCONGLOMERATE`   | bitfield| Orientation + visibility flags (section 9.5).                 |
| `SYMBOL_OUTER`      | enum    | Outer symbol (Appendix A.4).                                  |
| `SYMBOL_INNER`      | enum    | Inner symbol.                                                 |
| `SYMBOL_OUTEREDGE`  | enum    | Outer-edge symbol (e.g. negation bubble).                     |
| `SYMBOL_INNEREDGE`  | enum    | Inner-edge symbol (e.g. clock arrow).                         |

High-precision pin coordinates may be supplied out-of-band by the `.SchLib`
`PinFrac` stream; the in-record `LOCATION` is then a rounded value.

### 9.5 `PINCONGLOMERATE` bitfield

| Bit | Mask | Meaning                                       |
|-----|------|-----------------------------------------------|
| 0-1 | 0x03 | Orientation (Appendix A.1).                   |
| 2   | 0x04 | Pin hidden.                                   |
| 3   | 0x08 | Pin name shown.                               |
| 4   | 0x10 | Designator shown.                             |
| 5   | 0x20 | Unknown.                                      |
| 6   | 0x40 | Locked.                                       |

### 9.6 Designator (`RECORD=34`) and Parameter (`RECORD=41`) — owner interface

Designator labels the owning component's reference. Parameter is a generic
named label (value, tolerance, etc.).

| Property        | Type    | Meaning                                                            |
|-----------------|---------|--------------------------------------------------------------------|
| `NAME`          | string  | Parameter name (Designator's is `Designator`).                     |
| `TEXT`          | string  | Displayed value; UTF-8 variant in `%UTF8%TEXT`. A leading `=` makes it a reference to another parameter's value by `NAME`. |
| `LOCATION.X/.Y` | point   | Position (with `_FRAC` companions).                                |
| `ORIENTATION`   | enum    | 0/1/2/3 with alignment-then-rotation semantics (Appendix A.1).     |
| `JUSTIFICATION` | enum    | Anchor corner (Appendix A.2).                                      |
| `COLOR`         | colour  | Text colour.                                                       |
| `FONTID`        | integer | Index into the sheet font table.                                   |
| `ISHIDDEN`      | boolean | Hidden.                                                            |
| `ISMIRRORED`    | boolean | Mirrored.                                                          |
| `SHOWNAME`      | boolean | Show the parameter name as well as its value (Parameter only).     |
| `UNIQUEID`      | string  | Optional, project-unique; eight letters A–Y.                       |

`=CurrentDate`, `=CurrentTime` and `=DocumentFullPathAndName` are generated at
display time, not stored in a referenced parameter.

### 9.7 Label (`RECORD=4`) — owner interface

Free text. `TEXT`, `LOCATION.X/.Y`, `COLOR`, `FONTID`, `ISMIRRORED`,
`ORIENTATION` (Appendix A.1), `JUSTIFICATION` (Appendix A.2). Label text is not
passed through Altium-to-display string substitution the way other records' is.

### 9.8 Implementation list (`RECORD=44`) and Implementation (`RECORD=45`)

A component owns one implementation list (`RECORD=44`); the list owns
implementation records (`RECORD=45`) describing footprints and models.

| Property         | Type    | Meaning                                                        |
|------------------|---------|----------------------------------------------------------------|
| `MODELNAME`      | string  | Model/footprint name.                                          |
| `MODELTYPE`      | string  | `PCBLIB`, `SI`, `SIM`, `PCB3DLib`, …                           |
| `MODELDATAFILE0` | string  | Backing library/data file (first entry).                       |
| `DESCRIPTION`    | string  | Optional.                                                      |
| `ISCURRENT`      | boolean | This is the active implementation.                             |

Records 46, 47 and 48 are children of an implementation (datalinks, mapping
definers, model parameters); their internal fields are largely unconfirmed.

### 9.9 Graphic primitives

All inherit the owner interface; filled shapes add the fill interface; outlined
shapes add the border interface.

| Record                  | RECORD | Geometry properties                                                        |
|-------------------------|--------|----------------------------------------------------------------------------|
| Line                    | 13     | `LOCATION.X/.Y` → `CORNER.X/.Y`; `LINESTYLE`/`LINESTYLEEXT` (Appendix A.7). |
| Rectangle               | 14     | `LOCATION` = bottom-left, `CORNER` = top-right; fill + border.             |
| Round rectangle         | 10     | As rectangle plus `CORNERXRADIUS`, `CORNERYRADIUS`.                        |
| Polyline                | 6      | `LOCATIONCOUNT` + `Xn`/`Yn`; `LINESTYLE` (Appendix A.7); border.          |
| Polygon                 | 7      | `LOCATIONCOUNT` + `Xn`/`Yn`; fill + border.                                |
| Bezier                  | 5      | `LOCATIONCOUNT` + `Xn`/`Yn` control points; border.                       |
| Arc                     | 12     | `LOCATION` = centre, `RADIUS`, `STARTANGLE`, `ENDANGLE` (degrees).         |
| Elliptical arc          | 11     | As arc plus `SECONDARYRADIUS`.                                             |
| Ellipse                 | 8      | `LOCATION` = centre, `RADIUS`, `SECONDARYRADIUS`; fill + border.          |
| Piechart                | 9      | Same fields as arc.                                                        |
| Image                   | 30     | `LOCATION` and `CORNER` bounding box; `FILENAME`; `EMBEDIMAGE`, `KEEPASPECT` (boolean). Embedded bytes live in the `Storage` stream. |

### 9.10 Connectivity

| Record       | RECORD | Key properties                                                                    |
|--------------|--------|-----------------------------------------------------------------------------------|
| Wire         | 27     | `LOCATIONCOUNT` + `Xn`/`Yn` polyline; `LINEWIDTH`.                                 |
| Bus          | 26     | `LOCATIONCOUNT` + `Xn`/`Yn` polyline; `LINEWIDTH`.                                 |
| Bus entry    | 37     | `LOCATION` → `CORNER` segment; `COLOR`, `LINEWIDTH`.                               |
| Junction     | 29     | `LOCATION`.                                                                       |
| Net label    | 25     | `TEXT`, `LOCATION`, `ORIENTATION` (A.1), `JUSTIFICATION` (A.2).                    |
| Power port   | 17     | `TEXT` (net name), `LOCATION`, `ORIENTATION` (A.1), `STYLE` (A.5), `SHOWNETNAME`. |
| Port         | 18     | `NAME`, `LOCATION`, `WIDTH`, `HEIGHT`, `IOTYPE` (A.8), `STYLE` (A.9), `ALIGNMENT`, colours, `HARNESSTYPE`. |
| No-ERC       | 22     | `LOCATION`, `ISACTIVE`, `SUPPRESSALL`.                                             |

### 9.11 Hierarchy (multi-sheet design)

| Record        | RECORD | Key properties                                                                  |
|---------------|--------|---------------------------------------------------------------------------------|
| Sheet symbol  | 15     | `LOCATION`, `XSIZE`/`YSIZE`, `COLOR`, `AREACOLOR`, `ISSOLID`.                    |
| Sheet entry   | 16     | `NAME`, `DISTANCEFROMTOP` (`_FRAC1`), `SIDE` (A.10), `IOTYPE` (A.8), `STYLE` (A.9), `HARNESSTYPE`. |
| Sheet name    | 32     | `TEXT`, `LOCATION`, `ORIENTATION` (A.1), `ISHIDDEN`.                             |
| File name     | 33     | `TEXT`, `LOCATION`, `ORIENTATION` (A.1), `ISHIDDEN`.                             |

Sheet name (32) and file name (33) are owned by a sheet symbol and label it
with the child sheet's display name and file path respectively.

### 9.12 Harness objects (`RECORD` 215–218)

Signal-harness feature; typically in the `Additional` stream and children of
the sheet. Owner reconstruction often relies on stream position rather than
`OWNERINDEX`, and `OWNERINDEXADDITIONALLIST=T` marks objects whose owner index
refers to the `Additional` stream sequence.

| Record            | RECORD | Key properties                                                              |
|-------------------|--------|-----------------------------------------------------------------------------|
| Harness connector | 215    | `LOCATION`, `XSIZE`/`YSIZE`, `COLOR`, `AREACOLOR`, `PRIMARYCONNECTIONPOSITION`, `SIDE` (A.10). |
| Harness entry     | 216    | `NAME`, `DISTANCEFROMTOP` (`_FRAC1`), `SIDE` (A.10), `COLOR`, `AREACOLOR`, `TEXTCOLOR`, `TEXTFONTID`. |
| Harness type      | 217    | `TEXT`, `LOCATION`, `COLOR`, `TEXTFONTID`, `ISHIDDEN`.                       |
| Signal harness    | 218    | `LOCATIONCOUNT` + `Xn`/`Yn` polyline, `COLOR`, `LINEWIDTH`.                  |

### 9.13 Annotation and misc

| Record      | RECORD | Key properties                                                                       |
|-------------|--------|--------------------------------------------------------------------------------------|
| Text frame  | 28     | `LOCATION`/`CORNER` box, `TEXT` (`~1` encodes a newline), `FONTID`, `WORDWRAP`, `SHOWBORDER`, `TEXTMARGIN`, `ALIGNMENT` (A.11), `AREACOLOR`, `TEXTCOLOR`, `COLOR`, `LINEWIDTH`, `ISSOLID`. |
| Note        | 209    | Text frame plus `AUTHOR`. (Decodes as a text frame.)                                 |
| Hyperlink   | 226    | Label fields plus `URL`. (Decodes as a label.)                                       |
| Template    | 39     | `FILENAME`; owns title-block graphics.                                               |
| IEEE symbol | 3      | `LOCATION`, `SYMBOL`, `SCALEFACTOR`, `COLOR`.                                         |

## 10. Enumerations (Appendix A)

### A.1 Orientation (`TRotateBy90`)

| Value | Direction       |
|-------|-----------------|
| 0     | Rightwards (0°) |
| 1     | Upwards (90°)   |
| 2     | Leftwards (180°)|
| 3     | Downwards (270°)|

### A.2 Label justification (anchor)

| Value | Anchor        | Value | Anchor        | Value | Anchor       |
|-------|---------------|-------|---------------|-------|--------------|
| 0     | Bottom-left   | 3     | Center-left   | 6     | Top-left     |
| 1     | Bottom-center | 4     | Center-center | 7     | Top-center   |
| 2     | Bottom-right  | 5     | Center-right  | 8     | Top-right    |

### A.3 Pin electrical type (`ELECTRICAL`)

| Value | Type            | Value | Type           |
|-------|-----------------|-------|----------------|
| 0     | Input           | 4     | Passive        |
| 1     | Bidirectional   | 5     | Tri-state / Hi-Z |
| 2     | Output          | 6     | Open emitter   |
| 3     | Open collector  | 7     | Power          |

### A.4 Pin symbols (`SYMBOL_*`)

| Value | Symbol         | Value | Symbol                  |
|-------|----------------|-------|-------------------------|
| 0     | None           | 13    | Schmitt                 |
| 1     | Negated (dot)  | 17    | Low output              |
| 2     | Right-left     | 22    | Open collector pull-up  |
| 3     | Clock          | 23    | Open emitter            |
| 4     | Low input      | 24    | Open emitter pull-up    |
| 5     | Analog in      | 25    | Digital in              |
| 6     | No-logic-connect | 30  | Shift left              |
| 8     | Postpone output| 32    | Open output             |
| 9     | Open collector | 33    | Left-right              |
| 10    | Hi-Z           | 34    | Bidirectional           |
| 11    | High current   |       |                         |
| 12    | Pulse          |       |                         |

(Values 7, 14–16, 18–21, 26–29, 31 are unassigned.)

### A.5 Power port style (`STYLE`)

| Value | Style          | Value | Style              |
|-------|----------------|-------|--------------------|
| 0     | Circle         | 6     | Earth              |
| 1     | Arrow          | 7     | GOST arrow         |
| 2     | Bar            | 8     | GOST power ground  |
| 3     | Wave           | 9     | GOST earth         |
| 4     | Power ground   | 10    | GOST bar           |
| 5     | Signal ground  |       |                    |

### A.6 Sheet size (`SHEETSTYLE`) — value: name, drawing area in 10-mil units

| Value | Name   | Area (W×H) | Value | Name    | Area (W×H) |
|-------|--------|------------|-------|---------|------------|
| 0     | A4     | 1150×760   | 9     | E       | 4200×3200  |
| 1     | A3     | 1550×1110  | 10    | Letter  | 1100×850   |
| 2     | A2     | 2230×1570  | 11    | Legal   | 1400×850   |
| 3     | A1     | 3150×2230  | 12    | Tabloid | 1700×1100  |
| 4     | A0     | 4460×3150  | 13    | OrCAD A | 990×790    |
| 5     | A      | 950×750    | 14    | OrCAD B | 1540×990   |
| 6     | B      | 1500×950   | 15    | OrCAD C | 2060×1560  |
| 7     | C      | 2000×1500  | 16    | OrCAD D | 3260×2060  |
| 8     | D      | 3200×2000  | 17    | OrCAD E | 4280×3280  |

### A.7 Line style (`LINESTYLE` / `LINESTYLEEXT`)

| Value | Style       |
|-------|-------------|
| 0     | Solid       |
| 1     | Dashed      |
| 2     | Dotted      |
| 3     | Dash-dotted |

`LINESTYLE` overrides `LINESTYLEEXT` when both are present.

### A.8 Port I/O type (`IOTYPE`)

| Value | Type        |
|-------|-------------|
| 0     | Unspecified |
| 1     | Output      |
| 2     | Input       |
| 3     | Bidirectional |

### A.9 Port style (`STYLE`)

| Value | Style            | Value | Style       |
|-------|------------------|-------|-------------|
| 0     | None, horizontal | 4     | None, vertical |
| 1     | Left             | 5     | Top         |
| 2     | Right            | 6     | Bottom      |
| 3     | Left-right       | 7     | Top-bottom  |

### A.10 Sheet/harness side (`SIDE`)

| Value | Side   |
|-------|--------|
| 0     | Left   |
| 1     | Right  |
| 2     | Top    |
| 3     | Bottom |

### A.11 Text-frame alignment (`ALIGNMENT`)

| Value | Alignment |
|-------|-----------|
| 1     | Left      |
| 2     | Center    |
| 3     | Right     |

## 11. Storage stream (embedded files)

After its header record, the `Storage` stream holds one binary record per
embedded file (record type 1, i.e. the header top byte is nonzero). The binary
payload layout:

```
0xD0                          (1 byte, tag)
filename length               (1 byte)
filename                      (filename-length bytes)
compressed size               (uint32, little-endian)
zlib-compressed data          (compressed-size bytes, standard zlib stream)
```

The reference reader skips the first 5 bytes of the record body, reads a
length-prefixed file name, then a `uint32` data length, then that many bytes.
Image records (`RECORD=30`) reference these by `FILENAME`.

There is also an alternative, in-property encoding seen in some files: a
storage entry expressed as property keys `NAME`, `DATA_LEN`, and `DATA`, where
`DATA` is the file contents as a hex string of length `2 * DATA_LEN`.

## 12. Parsing strategy and known quirks

Recommended read path:

1. Open the file with a CFB reader; locate `FileHeader` (and `Storage`,
   `Additional` if present).
2. Walk `FileHeader` record by record: read the 4-byte header, split into
   length (low 24 bits) and type (high 8 bits), slice the payload.
3. For text payloads, split on `|`, build a key→value map, canonicalize keys
   (trim, upper-case), resolve `%UTF8%`/`UNICODE` variants for any value you
   need as Unicode.
4. Dispatch on `RECORD`. Decode coordinates via the `_FRAC` rule; treat
   absent integers as 0, absent booleans as false.
5. Build the ownership tree from `OWNERINDEX` (stream index of the owner),
   then apply `OWNERPARTID` / `OWNERPARTDISPLAYMODE` filtering against each
   component's `CURRENTPARTID` and active display mode.
6. Append `Additional` records to the same index space; reconstruct harness
   ownership from position where `OWNERINDEX` is unreliable.

Quirks to defend against:

- The trailing null of a text payload may be missing; do not assume it.
- The leading `|` of a property list may be missing (older files); locate the
  first key by `min(first '|', first '=')`.
- A property list ending mid-value has been observed (a dropped final byte);
  decode defensively.
- `DISPLAYMODE` may be a string rather than an integer; tolerate both.
- The byte `0xFF` inside Latin-1 values usually means a space.
- Property keys arrive in both upper-case and CamelCase; only the
  canonicalized form is reliable.
- Misspelled keys are normative: `ISNOTACCESIBLE` (one S).
- `DISTANCEFROMTOP` uses the `_FRAC1` (mil-scaled) variant, not `_FRAC`.
- The format carries no explicit version negotiation beyond the `HEADER`
  signature and `MINORVERSION`; field presence varies by Altium release, so
  treat every property as optional with a default.

For a Go reader, the CFB layer is available via `github.com/richardlehane/mscfb`
(stream enumeration) and inflation via the standard library `compress/zlib`.
The record and property layers above the CFB are simple enough to implement
directly: a `uint32` header split, a `bytes.Split` on `|`, and a
`map[string]string` per record, with typed accessors mirroring section 5.

## 13. Status and limitations

This specification is reconstructed from third-party readers and is adequate
for importing existing schematics, not for byte-exact round-tripping. The
fields of records 46/47/48, the compile mask (211), the blanket (225), and
several harness sub-fields are partially or wholly unconfirmed. Property sets
differ across Altium versions; absence of a property is normal and must map to
its documented default. Verify against real files before relying on any single
field.
