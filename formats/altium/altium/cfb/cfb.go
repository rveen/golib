// Package cfb reads and writes Microsoft Compound File Binary containers
// (OLE structured storage), the container format of .SchDoc and .PcbDoc files.
//
// Read loads the whole storage tree into memory; Bytes serialises it again as
// a version 3 file (512-byte sectors). Entry names, CLSIDs, state bits and
// timestamps survive a round trip; the sector layout does not.
package cfb

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sort"
	"unicode"
	"unicode/utf16"
)

// Entry types as stored in the directory.
const (
	TypeStorage = 1
	TypeStream  = 2
	TypeRoot    = 5
)

const (
	difSect    = 0xFFFFFFFC
	fatSect    = 0xFFFFFFFD
	endOfChain = 0xFFFFFFFE
	freeSect   = 0xFFFFFFFF
	noStream   = 0xFFFFFFFF

	sectorSize = 512
	miniSize   = 64
	miniCutoff = 4096
	dirSize    = 128
	headerFAT  = 109 // FAT sector locations held in the header
)

var magic = []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}

var le = binary.LittleEndian

// Entry is a storage, stream or the root storage.
type Entry struct {
	Name      string
	Type      uint8
	CLSID     [16]byte
	StateBits uint32
	Created   uint64 // Windows FILETIME
	Modified  uint64 // Windows FILETIME
	Data      []byte   // stream contents (streams only)
	Children  []*Entry // storages and root only
}

// Child returns the direct child with the given name (case-insensitive, as in
// CFB itself), or nil.
func (e *Entry) Child(name string) *Entry {
	for _, c := range e.Children {
		if compareNames(c.Name, name) == 0 {
			return c
		}
	}
	return nil
}

// Walk calls fn for e and every entry below it, depth first, with the path of
// names leading to the entry (empty for e itself).
func (e *Entry) Walk(fn func(path []string, e *Entry)) {
	var walk func(path []string, e *Entry)
	walk = func(path []string, e *Entry) {
		fn(path, e)
		for _, c := range e.Children {
			walk(append(path[:len(path):len(path)], c.Name), c)
		}
	}
	walk(nil, e)
}

// IsCFB reports whether b starts with the CFB signature.
func IsCFB(b []byte) bool {
	return len(b) >= 8 && bytes.Equal(b[:8], magic)
}

// Read parses a complete compound file and returns its root entry.
func Read(b []byte) (*Entry, error) {
	if len(b) < 512 || !IsCFB(b) {
		return nil, fmt.Errorf("not a compound file")
	}
	shift := le.Uint16(b[30:])
	if shift != 9 && shift != 12 {
		return nil, fmt.Errorf("unsupported sector shift %d", shift)
	}
	ss := 1 << shift
	ms := 1 << le.Uint16(b[32:])
	major := le.Uint16(b[26:])
	nFAT := int(le.Uint32(b[44:]))
	firstDir := le.Uint32(b[48:])
	cutoff := le.Uint32(b[56:])
	firstMiniFAT := le.Uint32(b[60:])
	nextDIFAT := le.Uint32(b[68:])

	sector := func(n uint32) ([]byte, error) {
		off := (int(n) + 1) * ss
		if n > 0x7FFFFFFF || off >= len(b) {
			return nil, fmt.Errorf("sector %d out of range", n)
		}
		return b[off:min(off+ss, len(b))], nil // the last sector may be short
	}

	// FAT sector locations: 109 in the header, the rest in the DIFAT chain.
	var fatSectors []uint32
	for i := 0; i < headerFAT && len(fatSectors) < nFAT; i++ {
		fatSectors = append(fatSectors, le.Uint32(b[76+4*i:]))
	}
	for seen := 0; len(fatSectors) < nFAT && nextDIFAT < endOfChain-1; seen++ {
		if seen > len(b)/ss {
			return nil, fmt.Errorf("DIFAT chain loops")
		}
		s, err := sector(nextDIFAT)
		if err != nil {
			return nil, err
		}
		n := ss/4 - 1
		for i := 0; i < n && len(fatSectors) < nFAT; i++ {
			fatSectors = append(fatSectors, le.Uint32(s[4*i:]))
		}
		nextDIFAT = le.Uint32(s[4*n:])
	}

	var fat []uint32
	for _, n := range fatSectors {
		s, err := sector(n)
		if err != nil {
			return nil, fmt.Errorf("FAT: %w", err)
		}
		for i := 0; i+4 <= len(s); i += 4 {
			fat = append(fat, le.Uint32(s[i:]))
		}
	}

	readChain := func(start uint32) ([]byte, error) {
		var out []byte
		for n, steps := start, 0; n != endOfChain; steps++ {
			if int(n) >= len(fat) || steps > len(fat) {
				return nil, fmt.Errorf("broken sector chain at %d", n)
			}
			s, err := sector(n)
			if err != nil {
				return nil, err
			}
			out = append(out, s...)
			n = fat[n]
		}
		return out, nil
	}

	dir, err := readChain(firstDir)
	if err != nil {
		return nil, fmt.Errorf("directory: %w", err)
	}
	var miniFAT []uint32
	if firstMiniFAT != endOfChain && firstMiniFAT != freeSect {
		mf, err := readChain(firstMiniFAT)
		if err != nil {
			return nil, fmt.Errorf("mini FAT: %w", err)
		}
		for i := 0; i+4 <= len(mf); i += 4 {
			miniFAT = append(miniFAT, le.Uint32(mf[i:]))
		}
	}

	type raw struct {
		e                  *Entry
		left, right, child uint32
		start              uint32
		size               uint64
	}
	var raws []raw
	for off := 0; off+dirSize <= len(dir); off += dirSize {
		d := dir[off : off+dirSize]
		nameLen := int(le.Uint16(d[64:]))
		var units []uint16
		for i := 0; i+2 <= min(nameLen, 64) && i < 62; i += 2 {
			units = append(units, le.Uint16(d[i:]))
		}
		if len(units) > 0 && units[len(units)-1] == 0 {
			units = units[:len(units)-1]
		}
		e := &Entry{
			Name:      string(utf16.Decode(units)),
			Type:      d[66],
			StateBits: le.Uint32(d[96:]),
			Created:   le.Uint64(d[100:]),
			Modified:  le.Uint64(d[108:]),
		}
		copy(e.CLSID[:], d[80:96])
		size := le.Uint64(d[120:])
		if major == 3 {
			size &= 0xFFFFFFFF
		}
		raws = append(raws, raw{e, le.Uint32(d[68:]), le.Uint32(d[72:]), le.Uint32(d[76:]), le.Uint32(d[116:]), size})
	}
	if len(raws) == 0 || raws[0].e.Type != TypeRoot {
		return nil, fmt.Errorf("missing root entry")
	}

	ministream, err := readChain(raws[0].start)
	if err != nil {
		return nil, fmt.Errorf("mini stream: %w", err)
	}
	if uint64(len(ministream)) > raws[0].size {
		ministream = ministream[:raws[0].size]
	}

	streamData := func(r raw) ([]byte, error) {
		if r.size == 0 {
			return []byte{}, nil
		}
		var out []byte
		if r.size < uint64(cutoff) {
			for n, steps := r.start, 0; n != endOfChain; steps++ {
				off := int(n) * ms
				if int(n) >= len(miniFAT) || steps > len(miniFAT) || off+ms > len(ministream) {
					return nil, fmt.Errorf("broken mini chain at %d", n)
				}
				out = append(out, ministream[off:off+ms]...)
				n = miniFAT[n]
			}
		} else {
			if out, err = readChain(r.start); err != nil {
				return nil, err
			}
		}
		if uint64(len(out)) < r.size {
			return nil, fmt.Errorf("stream shorter than its size")
		}
		return out[:r.size], nil
	}

	visited := make([]bool, len(raws))
	var siblings func(id uint32) ([]*Entry, error)
	siblings = func(id uint32) ([]*Entry, error) {
		if id == noStream {
			return nil, nil
		}
		if int(id) >= len(raws) || visited[id] {
			return nil, fmt.Errorf("bad directory tree at entry %d", id)
		}
		visited[id] = true
		r := raws[id]
		left, err := siblings(r.left)
		if err != nil {
			return nil, err
		}
		switch r.e.Type {
		case TypeStream:
			if r.e.Data, err = streamData(r); err != nil {
				return nil, fmt.Errorf("stream %q: %w", r.e.Name, err)
			}
		case TypeStorage:
			if r.e.Children, err = siblings(r.child); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("entry %d has type %d", id, r.e.Type)
		}
		right, err := siblings(r.right)
		if err != nil {
			return nil, err
		}
		return append(append(left, r.e), right...), nil
	}

	root := raws[0].e
	visited[0] = true
	if root.Children, err = siblings(raws[0].child); err != nil {
		return nil, err
	}
	return root, nil
}

// Bytes serialises the tree below root as a version 3 compound file.
func (root *Entry) Bytes() ([]byte, error) {
	if root.Type != TypeRoot {
		return nil, fmt.Errorf("Bytes must be called on the root entry")
	}

	type node struct {
		e                  *Entry
		left, right, child uint32
		red                bool
		start              uint32
		size               uint64
	}
	var nodes []*node

	// Siblings form a red-black tree. A tree built by splitting the sorted
	// list at the middle has all its null links at depth d or d+1, with
	// d = floor(log2(n+1)); colouring the nodes at depth d red gives every
	// path the same black height.
	var tree func(ids []uint32, depth, redDepth int) uint32
	tree = func(ids []uint32, depth, redDepth int) uint32 {
		if len(ids) == 0 {
			return noStream
		}
		mid := len(ids) / 2
		n := nodes[ids[mid]]
		n.red = depth >= redDepth
		n.left = tree(ids[:mid], depth+1, redDepth)
		n.right = tree(ids[mid+1:], depth+1, redDepth)
		return ids[mid]
	}

	var place func(e *Entry) (uint32, error)
	place = func(e *Entry) (uint32, error) {
		if len(utf16.Encode([]rune(e.Name))) > 31 {
			return 0, fmt.Errorf("entry name %q longer than 31 characters", e.Name)
		}
		id := uint32(len(nodes))
		nodes = append(nodes, &node{e: e, left: noStream, right: noStream, child: noStream})
		if e.Type == TypeStream {
			return id, nil
		}
		kids := append([]*Entry(nil), e.Children...)
		sort.SliceStable(kids, func(i, j int) bool { return compareNames(kids[i].Name, kids[j].Name) < 0 })
		ids := make([]uint32, len(kids))
		for i, k := range kids {
			if i > 0 && compareNames(kids[i-1].Name, k.Name) == 0 {
				return 0, fmt.Errorf("duplicate entry name %q", k.Name)
			}
			if k.Type != TypeStream && k.Type != TypeStorage {
				return 0, fmt.Errorf("entry %q has type %d", k.Name, k.Type)
			}
			var err error
			if ids[i], err = place(k); err != nil {
				return 0, err
			}
		}
		redDepth := 0
		for (1 << (redDepth + 1)) <= len(ids)+1 {
			redDepth++
		}
		nodes[id].child = tree(ids, 0, redDepth)
		return id, nil
	}
	if _, err := place(root); err != nil {
		return nil, err
	}
	nodes[0].red = false

	// Sectors after the header, and the FAT describing them.
	var body []byte
	var fat []uint32
	alloc := func(data []byte) (start uint32, count int) {
		if len(data) == 0 {
			return endOfChain, 0
		}
		start = uint32(len(fat))
		count = (len(data) + sectorSize - 1) / sectorSize
		body = append(body, data...)
		body = append(body, make([]byte, count*sectorSize-len(data))...)
		for i := 1; i < count; i++ {
			fat = append(fat, start+uint32(i))
		}
		fat = append(fat, endOfChain)
		return start, count
	}

	var mini []byte
	var miniFAT []uint32
	for _, n := range nodes {
		if n.e.Type != TypeStream {
			continue
		}
		n.size = uint64(len(n.e.Data))
		switch {
		case len(n.e.Data) == 0:
			n.start = endOfChain
		case len(n.e.Data) < miniCutoff:
			n.start = uint32(len(miniFAT))
			count := (len(n.e.Data) + miniSize - 1) / miniSize
			mini = append(mini, n.e.Data...)
			mini = append(mini, make([]byte, count*miniSize-len(n.e.Data))...)
			for i := 1; i < count; i++ {
				miniFAT = append(miniFAT, n.start+uint32(i))
			}
			miniFAT = append(miniFAT, endOfChain)
		default:
			n.start, _ = alloc(n.e.Data)
		}
	}
	nodes[0].start, _ = alloc(mini)
	nodes[0].size = uint64(len(mini))

	var mf []byte
	for _, v := range miniFAT {
		mf = le.AppendUint32(mf, v)
	}
	for len(mf)%sectorSize != 0 {
		mf = le.AppendUint32(mf, freeSect)
	}
	firstMiniFAT, nMiniFAT := alloc(mf)

	var dir []byte
	for _, n := range nodes {
		d := make([]byte, dirSize)
		units := utf16.Encode([]rune(n.e.Name))
		for i, u := range units {
			le.PutUint16(d[2*i:], u)
		}
		le.PutUint16(d[64:], uint16(2*(len(units)+1)))
		d[66] = n.e.Type
		if !n.red {
			d[67] = 1
		}
		le.PutUint32(d[68:], n.left)
		le.PutUint32(d[72:], n.right)
		le.PutUint32(d[76:], n.child)
		copy(d[80:96], n.e.CLSID[:])
		le.PutUint32(d[96:], n.e.StateBits)
		le.PutUint64(d[100:], n.e.Created)
		le.PutUint64(d[108:], n.e.Modified)
		if n.e.Type != TypeStorage {
			le.PutUint32(d[116:], n.start)
			le.PutUint64(d[120:], n.size)
		}
		dir = append(dir, d...)
	}
	for len(dir)%sectorSize != 0 {
		d := make([]byte, dirSize)
		le.PutUint32(d[68:], noStream)
		le.PutUint32(d[72:], noStream)
		le.PutUint32(d[76:], noStream)
		dir = append(dir, d...)
	}
	firstDir, _ := alloc(dir)

	// The FAT must also cover its own sectors and the DIFAT sectors.
	perFAT := sectorSize / 4
	perDIFAT := perFAT - 1
	nFAT, nDIFAT := 0, 0
	for {
		total := len(fat) + nFAT + nDIFAT
		needFAT := (total + perFAT - 1) / perFAT
		needDIFAT := 0
		if needFAT > headerFAT {
			needDIFAT = (needFAT - headerFAT + perDIFAT - 1) / perDIFAT
		}
		if needFAT == nFAT && needDIFAT == nDIFAT {
			break
		}
		nFAT, nDIFAT = needFAT, needDIFAT
	}
	firstFATSector := uint32(len(fat))
	for i := 0; i < nFAT; i++ {
		fat = append(fat, fatSect)
	}
	firstDIFATSector := uint32(len(fat))
	for i := 0; i < nDIFAT; i++ {
		fat = append(fat, difSect)
	}
	for len(fat) < nFAT*perFAT {
		fat = append(fat, freeSect)
	}
	for _, v := range fat {
		body = le.AppendUint32(body, v)
	}

	fatLoc := func(i int) uint32 {
		if i < nFAT {
			return firstFATSector + uint32(i)
		}
		return freeSect
	}
	for d := 0; d < nDIFAT; d++ {
		for i := 0; i < perDIFAT; i++ {
			body = le.AppendUint32(body, fatLoc(headerFAT+d*perDIFAT+i))
		}
		next := uint32(endOfChain)
		if d+1 < nDIFAT {
			next = firstDIFATSector + uint32(d+1)
		}
		body = le.AppendUint32(body, next)
	}

	h := make([]byte, sectorSize)
	copy(h, magic)
	le.PutUint16(h[24:], 0x003E)
	le.PutUint16(h[26:], 3)
	le.PutUint16(h[28:], 0xFFFE)
	le.PutUint16(h[30:], 9)
	le.PutUint16(h[32:], 6)
	le.PutUint32(h[44:], uint32(nFAT))
	le.PutUint32(h[48:], firstDir)
	le.PutUint32(h[56:], miniCutoff)
	le.PutUint32(h[60:], firstMiniFAT)
	le.PutUint32(h[64:], uint32(nMiniFAT))
	if nDIFAT > 0 {
		le.PutUint32(h[68:], firstDIFATSector)
	} else {
		le.PutUint32(h[68:], endOfChain)
	}
	le.PutUint32(h[72:], uint32(nDIFAT))
	for i := 0; i < headerFAT; i++ {
		le.PutUint32(h[76+4*i:], fatLoc(i))
	}
	return append(h, body...), nil
}

// compareNames orders directory names as CFB requires: shorter names first,
// then by upper-cased UTF-16 code units.
func compareNames(a, b string) int {
	ua, ub := utf16.Encode([]rune(a)), utf16.Encode([]rune(b))
	if len(ua) != len(ub) {
		return len(ua) - len(ub)
	}
	for i := range ua {
		ca, cb := unicode.ToUpper(rune(ua[i])), unicode.ToUpper(rune(ub[i]))
		if ca != cb {
			return int(ca) - int(cb)
		}
	}
	return 0
}
