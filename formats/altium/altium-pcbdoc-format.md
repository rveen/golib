# Altium PcbDoc binary format specification

This document describes the on-disk structure of Altium Designer `.PcbDoc` files
(and the related `.CSPcbDoc`, `.CMPcbDoc`, `.PcbLib`, `.IntLib` variants). It is a
reverse-engineered description, not an official specification. The format is
proprietary and undocumented by Altium.

The same PCB binary format is produced by Altium Designer, Altium Circuit Studio,
Altium Circuit Maker, and Solidworks PCB.

## Status and sources

This description is compiled from independent reverse-engineering work. The primary
source is the KiCad Altium importer, including its formal Kaitai Struct definition.
See the [references](#references) section for the full list.

Field offsets, lengths, and enumerations given here are those that importers rely on
in practice. Bytes marked *reserved* are skipped by known importers and their meaning
is unknown. Several values are version-dependent; this is noted where known.

---

## 1. Container

A `.PcbDoc` file is a **Microsoft Compound File Binary Format** container
(MS-CFB, also called OLE Structured Storage or OLE2). This is the same container
used by legacy Microsoft Office files (`.doc`, `.xls`).

The file begins with the standard CFB signature:

```
D0 CF 11 E0 A1 B1 1A E1
```

A reader detects a valid file by testing this magic number. Parsing the container
itself (FAT, directory entries, mini-stream, sector chains) is fully specified by
MS-CFB and is out of scope here; use an existing CFB library.

The CFB container exposes a tree of named **storages** (directories) and **streams**
(byte files). Everything below concerns the contents of those streams.

---

## 2. Top-level layout

The root storage contains one storage per object class, plus a few special streams.
Each object-class storage normally contains exactly two streams:

| Stream     | Contents                                                       |
|------------|----------------------------------------------------------------|
| `Header`   | A single `uint32` record count (little-endian).                |
| `Data`     | The records themselves, concatenated.                          |

Some storages add auxiliary streams (for example `Parameters`, or an
`ExtendedPrimitiveInformation/Data` pair).

The record count in `Header` is advisory. Robust importers read `Data` until the
stream is exhausted rather than trusting the count.

### 2.1 Directory tree

```
Board.PcbDoc                  (CFB container)
├── FileHeader                (version / magic string)
├── Board6/      Header, Data   board setup, layer stackup  (text records)
├── Components6/ Header, Data   component instances          (text records)
├── Nets6/       Header, Data   net list                     (text records)
├── Classes6/    Header, Data   net/component/layer classes  (text records)
├── Rules6/      Header, Data   design rules                 (text records)
├── Polygons6/   Header, Data   copper pours                 (text records)
├── Dimensions6/ Header, Data   dimensions                   (text records)
├── Arcs6/       Header, Data   arcs                         (binary records)
├── Pads6/       Header, Data   pads                         (binary records)
├── Vias6/       Header, Data   vias                         (binary records)
├── Tracks6/     Header, Data   line segments                (binary records)
├── Texts6/      Header, Data   strings                      (binary records)
├── Fills6/      Header, Data   rectangular fills            (binary records)
├── Regions6/    Header, Data   polygonal regions            (binary records)
├── ShapeBasedRegions6/         shape-based regions          (binary records)
├── ComponentBodies6/           3D body links                (binary records)
├── BoardRegions/               board outline regions        (binary records)
├── WideStrings6/ Header, Data  UTF-16 string table          (table, §4.5)
├── Models/      Header, Data   3D model references + STEP data
├── SmartUnions/ Header, Data   primitive unions
├── ExtendedPrimitiveInformation/ Header, Data  per-primitive overrides
└── ... (further optional storages, see §2.2)
```

The trailing `6` in most names is an Altium internal version tag, not a count.

### 2.2 Known storages

The following storages have been observed. Not all are present in every file;
presence depends on the design and on the writing application version.

```
AdvancedPlacerOptions6        FromTos6                  Polygons6
Arcs6                         Models                    Regions6
Board6                        ModelsNoEmbed             Rules6
BoardRegions                  Nets6                     ShapeBasedComponentBodies6
Classes6                      Pads6                     ShapeBasedRegions6
ComponentBodies6              PadViaLibrary             SignalClasses
Components6                   PadViaLibraryCache        SmartUnions
Connections6                  PadViaLibraryLinks        Texts / Texts6
Coordinates6                  PinSwapOptions6           Textures
DesignRuleCheckerOptions6     PinPairsSection           Tracks6
DifferentialPairs6            UnionNames                UniqueIDPrimitiveInformation
Dimensions6                   UniqueIdPrimitiveInformation
EmbeddedBoards6               Vias6
EmbeddedFonts6                WideStrings6
Embeddeds6
ExtendedPrimitiveInformation
FileVersionInfo
Fills6
```

---

## 3. Common conventions

### 3.1 Byte order and integer types

All multi-byte integers and floats are **little-endian**.

| Notation | Meaning                          |
|----------|----------------------------------|
| `u1`     | unsigned 8-bit                   |
| `u2`     | unsigned 16-bit                  |
| `u4`     | unsigned 32-bit                  |
| `s4`     | signed 32-bit                    |
| `f8`     | IEEE-754 double (8 bytes)        |

### 3.2 Coordinates and units

The native length unit is **0.1 microinch** (0.1 µin = 2.54 nm). Equivalently:

```
1 mil  = 10 000 native units
1 inch = 100 000 000 native units
1 native unit = 2.54 nm
```

Coordinates in binary records are stored as `s4` pairs `(x, y)`.

The Y axis points **up** (increasing Y is toward the top of the board). Tools whose
Y axis points down must negate Y on import.

Angles are stored as `f8` in **degrees**, counter-clockwise.

### 3.3 Strings

Three string encodings appear:

**Short Pascal string** — one `u1` length byte, then that many bytes of payload.
Used for designators, font names, and the like.

**Long Pascal string** — one `u4` length, then that many payload bytes. Used for
embedded blobs.

**Code page.** Legacy byte strings are interpreted as **ISO 8859-1** (Latin-1) by
default. Newer files carry Unicode separately (see §3.4 and §4.5). The exact code
page is not stored in the file; ISO 8859-1 is a pragmatic default.

### 3.4 Property lists (the text record body)

Text-format records store their fields as a single delimited string:

```
|KEY1=value1|KEY2=value2|...|KEYn=valuen
```

Structure of one property block:

1. `u4` length. If any bit of the **top byte** is set (`length & 0xFF000000`), the
   block is a **binary blob**, not a property list; the low 24 bits give its length
   (`length & 0x00FFFFFF`) and the bytes are handled by a record-specific decoder.
2. Otherwise the low 24 bits give the byte length of an ASCII/Latin-1 string.
3. The string is normally NUL-terminated; the terminator is not part of the data.
   (Some files written by older versions omit it.)

Parsing rules used by importers:

- Tokens are separated by `|`. Each token is `KEY=VALUE`. A leading `|` may be
  missing on the first token in old files.
- Keys are **case-folded to upper case** and trimmed. Altium writes keys in either
  all-caps or CamelCase; folding unifies them.
- Booleans are the strings `T`/`TRUE` (true) versus anything else (false).
- A key prefixed `%UTF8%` carries a UTF-8 value; the prefix is stripped and the value
  decoded as UTF-8 instead of Latin-1. The plain and `%UTF8%` forms may coexist.
- A `UNICODE` key set to a value containing `EXISTS` signals that `UNICODE__<KEY>`
  entries hold comma-separated UTF-16 code units for the corresponding fields.

Common keys seen across record types include
`RECORD`, `LAYER`, `NET`, `COMPONENT`, `LOCKED`, `KEEPOUTRESTRIC`, `ID`,
`NAME`, `CHECKSUM`, `KIND`, `ROTATION`, and many record-specific keys.

---

## 4. Record formats

There are two record encodings. Which one a storage uses is fixed per storage type
(see the tree in §2.1).

### 4.1 Text records

The `Data` stream is a sequence of property blocks (§3.4). Each block is one record.
The first key is usually `RECORD=<n>` identifying the object type.

Example — a net entry in `Nets6/Data` carries at least:

```
|RECORD=...|NAME=GND|...
```

Storages using this encoding: `Board6`, `Components6`, `Nets6`, `Classes6`,
`Rules6`, `Polygons6`, `Dimensions6`.

`Board6` is the central setup record. It holds the layer stackup (one numbered
group of keys per layer, e.g. copper thickness, dielectric constant, material name),
track-width and grid defaults, and global board parameters.

### 4.2 Binary records — framing

The `Data` stream is a sequence of binary records. Each record begins with:

```
u1  record_type      (see enum, §5.1)
```

followed by one or more **subrecords**. Each subrecord is:

```
u4  subrecord_length
... subrecord_length bytes of payload
```

A reader sets a cursor at `payload_start + subrecord_length` and skips to it after
parsing, so unknown trailing bytes inside a subrecord are tolerated. The number of
subrecords is fixed per record type.

The layouts below give byte offsets **within the first subrecord payload** unless
stated otherwise. Flag bytes are described bit by bit where known; `is_locked` is
inverted (`locked` when the bit is *clear*).

### 4.3 Arc record (`record_type = 1`)

One subrecord.

| Off | Type | Field            | Notes                                   |
|-----|------|------------------|-----------------------------------------|
| 0   | u1   | layer            | v6 layer id (§5.2)                       |
| 1   | u1   | flags1           | bit2 → not locked; bit1 → polygon outline|
| 2   | u1   | flags2 / keepout | value 2 ⇒ keepout                        |
| 3   | u2   | net              | index into Nets6, `0xFFFF` = none        |
| 5   | u2   | polygon          | owning polygon index                     |
| 7   | u2   | component        | owning component index, `0xFFFF` = none  |
| 9   | —    | reserved (4)     |                                          |
| 13  | xy   | center           | s4, s4                                    |
| 21  | s4   | radius           |                                          |
| 25  | f8   | start_angle      | degrees                                  |
| 33  | f8   | end_angle        | degrees                                  |
| 41  | s4   | width            |                                          |
| 45  | u2   | subpolyindex     |                                          |

If the subrecord extends further, later versions append: a reserved byte, a `u4`
union index, a `u4` v7 layer id, and a `u1` keepout-restrictions bitmask.

### 4.4 Track record (`record_type = 4`)

One subrecord. Identical header to the arc through `component`, then:

| Off | Type | Field        |
|-----|------|--------------|
| 13  | xy   | start        |
| 21  | xy   | end          |
| 29  | s4   | width        |
| 33  | —    | reserved (12)|

A keepout-restrictions byte follows when `subrecord_length >= 46`.

### 4.5 Via record (`record_type = 3`)

One subrecord.

| Off | Type | Field        | Notes                                   |
|-----|------|--------------|-----------------------------------------|
| 0   | —    | reserved (1) |                                         |
| 1   | u1   | flags1       | tenting / test-fab / lock bits          |
| 2   | u1   | flags2       | test-fab bottom in bit0                 |
| 3   | u2   | net          |                                         |
| 5   | —    | reserved (2) |                                         |
| 7   | u2   | component    |                                         |
| 9   | —    | reserved (4) |                                         |
| 13  | xy   | position     |                                         |
| 21  | s4   | diameter     | pad diameter                            |
| 25  | s4   | hole_size    |                                         |
| 29  | u1   | start_layer  | v6 layer id                             |
| 30  | u1   | end_layer    | v6 layer id                             |
| 31  | —    | reserved (43)|                                         |
| 74  | u1   | via_mode     | pad-mode enum (§5.3)                     |
| 75  | s4×32| diameter_alt | per-layer diameters                     |

### 4.6 Pad record (`record_type = 2`)

The most complex record. **Six subrecords**, each `u4`-length-prefixed:

| # | Contents                                                              |
|---|-----------------------------------------------------------------------|
| 1 | Designator — a short Pascal string.                                   |
| 2 | Reserved / unused.                                                    |
| 3 | Reserved / unused.                                                    |
| 4 | Reserved / unused.                                                    |
| 5 | Geometry block, **≥ 110 bytes** (layout below).                      |
| 6 | Per-layer size/shape stack (present only when its length > 0).       |

Subrecord 5 layout (offsets from its payload start):

| Off | Type | Field                       | Notes                              |
|-----|------|-----------------------------|------------------------------------|
| 0   | u1   | layer                       | v6 layer id                        |
| 1   | u1   | flags1                      | tenting / test-fab top / lock      |
| 2   | u1   | flags2                      | test-fab bottom                    |
| 3   | u2   | net                         |                                    |
| 5   | —    | reserved (2)                |                                    |
| 7   | u2   | component                   |                                    |
| 9   | —    | reserved (4)                |                                    |
| 13  | xy   | position                    |                                    |
| 21  | xy   | top_size                    |                                    |
| 29  | xy   | mid_size                    |                                    |
| 37  | xy   | bottom_size                 |                                    |
| 45  | s4   | hole_size                   |                                    |
| 49  | u1   | top_shape                   | pad-shape enum (§5.4)              |
| 50  | u1   | mid_shape                   |                                    |
| 51  | u1   | bottom_shape                |                                    |
| 52  | f8   | direction                   | rotation, degrees                  |
| 60  | u1   | plated                      | boolean                            |
| 61  | —    | reserved (1)                |                                    |
| 62  | u1   | pad_mode                    | pad-mode enum (§5.3)               |
| 63  | —    | reserved (23)               |                                    |
| 86  | s4   | paste_mask_expansion_manual |                                    |
| 90  | s4   | solder_mask_expansion_manual|                                    |
| 94  | —    | reserved (7)                |                                    |
| 101 | u1   | paste_mask_expansion_mode   | mode enum (§5.5)                   |
| 102 | u1   | solder_mask_expansion_mode  |                                    |
| 103 | —    | reserved (3)                |                                    |

When subrecord 5 is larger, further fields follow, including hole rotation,
testpoint-assembly flags, and pad-to-die length.

Subrecord 6 (optional, full pad stack) contains, in order: 29 `s4` X half-sizes,
29 `s4` Y half-sizes, 29 `u1` shapes, a reserved byte, a `u1` hole type
(0 round, 1 square, 2 slot), an `s4` slot length, an `f8` slot rotation, 32 `s4`
hole-offset X, 32 `s4` hole-offset Y, a reserved byte, 32 `u1` alternate shapes,
32 `u1` corner radii, then 32 reserved bytes.

### 4.7 Text record (`record_type = 5`)

Two subrecords: a fixed properties block, then the string.

Subrecord 1 (selected fields):

| Off | Type    | Field             | Notes                              |
|-----|---------|-------------------|------------------------------------|
| 0   | u1      | layer             |                                    |
| 1   | u1      | flags             | lock bit                           |
| 3   | u2      | net               |                                    |
| 7   | u2      | component         |                                    |
| 13  | xy      | position          |                                    |
| 21  | u4      | height            |                                    |
| 25  | u1      | font_name_id      |                                    |
| 27  | f8      | rotation          | degrees                            |
| 35  | u1      | mirrored          | boolean                            |
| 36  | u4      | stroke_width      |                                    |
| 40  | u1      | is_comment        | boolean                            |
| 41  | u1      | is_designator     | boolean                            |
| 44  | u1      | bold              | boolean                            |
| 45  | u1      | italic            | boolean                            |
| 46  | str(64) | font_name         | UTF-16, NUL-padded                 |
| ... | u1      | inverted          | boolean                            |
| ... | s4      | margin            |                                    |
| ... | u1      | font_type         | text-type enum (§5.6)              |
| ... | str(64) | barcode_name      | UTF-16; barcode fields if used     |

Subrecord 2 is a short Pascal string: the displayed text. When the text record
references the wide-string table (§4.9), the visible string is taken from there.

### 4.8 Fill, region, and component-body records

**Fill (`6`)** — one subrecord: header as in §4.3 through `component`, then
`xy pos1`, `xy pos2` (opposite corners), `f8 rotation`, reserved bytes, optional
keepout byte.

**Region (`11` = 0x0B)** — one subrecord: standard header, `u2 subpolyindex`,
`u2 component`, reserved, `u2 hole_count`, reserved, then a `u4`-length property
string, then the outline as a vertex list, then `hole_count` hole vertex lists.
Each vertex list is `u4 count` followed by `count` vertices. Two vertex encodings
exist: `(f8 x, f8 y)` point pairs, and a richer arc-aware vertex
`(u1 is_round, xy position, xy center, u4 radius, f8 angle1, f8 angle2)`.
`ShapeBasedRegions6` and `BoardRegions` reuse this record.

**Component body (`12` = 0x0C)** — one subrecord: reserved (7), `u2 component`,
reserved (9), then a `u4`-length property string carrying the 3D model id, offsets,
and rotation. Model geometry itself lives in `Models`.

### 4.9 Wide-string table (`WideStrings6/Data`)

A flat table mapping integer indices to UTF-16 strings, used so that Unicode text
need not be embedded in each text record. Repeated entries until end of stream:

```
u4  index
u4  byte_length          (length ≤ 2 means an empty string; no NUL bytes stored)
... byte_length bytes    (UTF-16LE, includes a trailing NUL counted in byte_length)
```

Text records reference rows by index.

---

## 5. Enumerations

### 5.1 Binary record type (first byte of each binary record)

| Value | Record              |
|-------|---------------------|
| 1     | Arc                 |
| 2     | Pad                 |
| 3     | Via                 |
| 4     | Track               |
| 5     | Text                |
| 6     | Fill                |
| 11 (0x0B) | Region          |
| 12 (0x0C) | Model / component body |

### 5.2 Layer ids (v6 scheme, `u1`)

| Range  | Meaning                                  |
|--------|------------------------------------------|
| 1      | Top copper                               |
| 2–31   | Mid layers 1–30                          |
| 32     | Bottom copper                            |
| 33 / 34| Top / bottom overlay (silkscreen)        |
| 35 / 36| Top / bottom paste                       |
| 37 / 38| Top / bottom solder mask                 |
| 39–54  | Internal planes 1–16                     |
| 55     | Drill guide                              |
| 56     | Keepout                                  |
| 57–72  | Mechanical 1–16                          |
| 73     | Drill drawing                            |
| 74     | Multi-layer                              |
| 75–82  | Connections, background, markers, grids, pad/via holes |

Two later layer schemes exist for designs with more than 30 copper or 16 mechanical
layers. They use 32-bit ids in separate ranges (a v7 copper base at `0x01000000`,
a v7 mechanical base at `0x01020000`, and v8 "other" layers based at `0x01030000`).
When present, the 32-bit id supersedes the `u1` layer field.

### 5.3 Pad / via mode

| Value | Meaning                              |
|-------|--------------------------------------|
| 0     | Simple (one size on all layers)      |
| 1     | Top / middle / bottom                |
| 2     | Full layer stack                     |

### 5.4 Pad shape

| Value | Shape                                       |
|-------|---------------------------------------------|
| 0     | Unknown                                     |
| 1     | Circle (round)                              |
| 2     | Rectangle                                   |
| 3     | Octagonal                                   |
| 9     | Rounded rectangle (alternate-shape table only) |

Pad hole shape: 0 round, 1 square, 2 slot.

### 5.5 Mask-expansion mode

| Value | Meaning |
|-------|---------|
| 0     | Unknown / none |
| 1     | From rule |
| 2     | Manual    |

### 5.6 Text type and barcode

Text type: 0 stroke, 1 TrueType, 2 barcode.
Barcode type: 0 Code 39, 1 Code 128.
Text justification (1–9): left/center/right × top/center/bottom; 0 means manual.

### 5.7 Other enumerations (text records)

**Class kind** (`Classes6`): 0 net, 1 source-schematic, 2 from-to, 3 pad, 4 layer,
6 differential-pair, 7 polygon.

**Rule kind** (`Rules6`): 1 clearance, 2 diff-pair routing, 3 height, 4 hole size,
5 hole-to-hole clearance, 6 width, 7 paste-mask expansion, 8 solder-mask expansion,
9 plane clearance, 10 polygon connect, 11 routing vias.

**Region kind**: 0 copper, 1 polygon cutout, 2 dashed outline, 4 cavity definition,
5 board cutout.

**Dimension kind** (`Dimensions6`): 1 linear, 2 angular, 3 radial, 4 leader,
5 datum, 6 baseline, 7 center, 9 linear-diameter, 10 radial-diameter.

**Polygon hatch style** (`Polygons6`): 1 solid, 2 45°, 3 90°, 4 horizontal,
5 vertical, 6 none.

**Connect style**: 1 direct, 2 thermal relief, 3 none.

---

## 6. Special streams

### 6.1 FileHeader

Holds a version/identification string, written as a length-prefixed subrecord
followed by a short Pascal string such as `PCB 5.0 Binary File`. It identifies the
writing application generation. It carries no geometry.

### 6.2 Models and embedded 3D data

`Models/Data` lists 3D model references (as property records) including model id,
rotation, and Z offset. The model geometry is STEP data, stored compressed.
Compressed blobs use a `0x02` lead byte for zlib-compressed and `0x00` for
uncompressed (this convention is shared with the IntLib stream decoder).

### 6.3 ExtendedPrimitiveInformation

An `ExtendedPrimitiveInformation/Data` stream holds per-primitive overrides keyed by
record type and primitive index — for example, per-object solder/paste mask
expansion that is not expressible in the base record.

---

## 7. Libraries and embedded containers

A `.PcbLib` footprint library is itself a CFB container. Each footprint is a named
storage; its `Data` stream is a sequence of the same binary primitive records
described in §4, optionally with `Parameters` and `ExtendedPrimitiveInformation`
sub-streams.

A `.IntLib` integrated library nests further: each entry under its `PCBLib` storage
is a compressed stream that, once decompressed (lead byte `0x02` = zlib,
`0x00` = raw), is a complete CFB container of its own.

---

## 8. Caveats

- The format is undocumented and version-dependent. Subrecord lengths grow across
  versions; always honour `subrecord_length` and tolerate unknown trailing bytes.
- The text-stream code page is not recorded; Latin-1 is a heuristic default, with
  Unicode carried via `%UTF8%` keys, `UNICODE__` keys, or the `WideStrings6` table.
- The `Header` record count is unreliable; read until end of `Data`.
- Reserved byte runs almost certainly carry meaning that is simply not yet decoded.
- Field offsets here reflect what importers depend on, not an exhaustive decode of
  every byte.

---

## References

Primary:

- KiCad Altium importer source —
  `pcbnew/pcb_io/altium` and `common/io/altium` in the KiCad source tree.
- KiCad Kaitai Struct definition — `pcbnew/pcb_io/altium/altium_parser.ksy`
  (a formal, machine-readable description of the binary records).
- KiCad developer documentation, Altium import —
  https://dev-docs.kicad.org/en/import-formats/altium/index.html

Reverse-engineering work and other implementations:

- T. Pointhuber, "Reverse-Engineering of (binary) File-Formats" (FOSDEM 2021) —
  the talk by the KiCad importer author describing the methodology.
- thesourcerer8/altium2kicad — Perl converter; the `convertpcb.pl` offsets are an
  early public description of the PCB records.
- matthiasbock/python-altium and vadmium/python-altium — record-level documentation
  (schematic-focused but shares the container and property conventions).
- pcjc2/openaltium — C++ implementation.
- issus/AltiumSharp — C# implementation.
- "Protel 99 SE PCB ASCII File Format Reference" — the documented ASCII counterpart
  of the binary format.

Container format:

- Microsoft Compound File Binary Format (MS-CFB) specification.
- microsoft/compoundfilereader — the CFB reader used by the KiCad importer.
