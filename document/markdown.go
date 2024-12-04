package document

import (
	"regexp"
	"strings"

	"github.com/rveen/golib/csv"
	"github.com/rveen/golib/parser"
	"github.com/rveen/ogdl"
)

var (
	anchor = regexp.MustCompile(`{#\w+}`)
	typ    = regexp.MustCompile(`{!\w+}`)
	link   = regexp.MustCompile(`\[([^\]]+)\]\(([^\)]+)\)`)
	link2  = regexp.MustCompile(`\[\]\(([^\)]+)\)`)
	img    = regexp.MustCompile(`!\[([^\]]+)\]\( *([^ ]+) *([^\)]*)\)`)
	img2   = regexp.MustCompile(`!\[\]\( *([^ ]+) *(.*)\)`)

	// Not complete: * should not be followed by space
	bold   = regexp.MustCompile(`\*\*([^\*]+)\*\*`)
	italic = regexp.MustCompile(`\*([^\*]+)\*`)
)

// Markdown entities:
//
// Block:
// - !p, text: paragraph / block quote
// - !pre_xxx: code
// - !h, header
// - !tb, table
// - !ul, !ol, list (numbered or not)
// - !dl, definition list
// - input, form (->action->output)
// - !g, data
// - !x command
//
// Inline:
// - image
// - link
// - style
//

func block(p *parser.Parser) bool {

	c := p.PeekByte()

	switch c {
	case 0:
		return false
	case '#':
		header(p)
	case '.':
		command(p)
	case '>':
		quote(p)
	case '-':
		list(p)
	case '+':
		nlist(p)
	case '|':
		table(p)
	case '`':
		code(p)
	case '{':
		data(p)
	default:
		paragraph(p, "!p", "")
	}

	return true
}

func inLine(s string) string {
	s = img.ReplaceAllString(s, "<img style=\"$3\" src=\"$2\">")
	s = img2.ReplaceAllString(s, "<img style=\"$2\" src=\"$1\">")
	s = link.ReplaceAllString(s, "<a href=\"$2\">$1</a>")
	s = link2.ReplaceAllString(s, "<a href=\"$1\">$1</a>")
	s = bold.ReplaceAllString(s, "<b>$1</b>")
	s = italic.ReplaceAllString(s, "<em>$1</em>")

	s = strings.ReplaceAll(s, "___?", "<input class='form-control' type='text'/>")
	s = strings.ReplaceAll(s, "_ok_?", "<input class='btn btn-primary' type='submit' value='Submit'>")

	s = strings.ReplaceAll(s, ">-implements</a>", "><button class='btn btn-sm btn-info'>Implements</button></a>")

	return s
}

// Header processes lines containing a header
//
// TODO # text {#anchor !type}
//
// Output format:
//
//	!h
//	  1
//	  "Header text"
//	  headerText
//
// The second subnode is the normalized string to be used in the data representation
func header(p *parser.Parser) {

	// Read number of !
	n := 0
	var c byte
	var ok bool
	var title bool

	for {
		c, ok = p.Byte()
		if !ok {
			return
		}
		if c == '!' {
			title = true
			c, ok = p.Byte()
			break
		}
		if c != '#' {
			break
		}

		n++
	}
	if c != ' ' {
		// This is not a header but text
		paragraph(p, "!p", "!"+string(c))
		return
	}

	// The rest of the line is the header
	b := []byte{'0'}
	if !title {
		b[0] += byte(n)
	}
	p.Emit("!h")
	p.Inc()
	// h0 = title, h1...h6 = header
	p.Emit(string(b))

	p.Space()
	s := p.Line()

	// Check for anchor and type

	var key, typ string
	typ, s = getType(s)
	key, s = getKey(s)

	p.Emit(s) // The header text
	p.Emit("#" + key)
	p.Emit("!" + typ)

	p.Dec()
}

func getKey(s string) (string, string) {
	ii := anchor.FindStringIndex(s)
	key := ""

	if ii != nil {
		key = s[ii[0]+2 : ii[1]-1]
		s = s[:ii[0]]
	} else {
		key = Normalize(s)
	}
	return key, strings.TrimSpace(s)
}

func getType(s string) (string, string) {
	ii := typ.FindStringIndex(s)
	key := ""

	if ii == nil {
		return "", s
	}

	key = s[ii[0]+2 : ii[1]-1]
	s = s[:ii[0]]

	return key, strings.TrimSpace(s)
}

func quote(p *parser.Parser) {
	paragraph(p, "!q", "")
}

func command(p *parser.Parser) {
	s := p.Line()

	if s == ".csv" {
		tableCsv(p, "")
		return
	}

	if strings.HasPrefix(s, ".csv ") {
		ss := strings.Fields(s)
		if len(ss) > 1 {
			tableCsv(p, ss[1])
		} else {
			tableCsv(p, "")
		}
		return
	}

	p.Emit(s)
}

// List processes (eventually) nested lists.
// One call to this function processes all consecutive lines starting with '-',
// even if not in the first position. Indentation levels need to be taken into
// account.
//
// Output format:
//
// !ul
//
//	!l1
//	  "item 1"
//	  item1
//
// TODO recursive parsing of lists (nested lists can be ul or ol)
// TODO detect HR (---)
// TODO include definition lists (- text :: text) <-- not std markdown
func list(p *parser.Parser) {

	level := 1
	prevIndent := 0

	p.Emit("!ul")
	p.Inc()

	for {
		ix := p.Ix
		indent, _ := p.Space()
		if c, _ := p.Byte(); c != '-' {
			// end of list
			p.Ix = ix
			break
		}

		if indent > prevIndent {
			level++
		} else if indent < prevIndent {
			level--
		}

		// Read the text of the item, with possibly a key
		s := p.Line()
		k, s := getKey(s)

		if s != "" {

			p.Emit("!li")
			p.Inc()
			b := []byte{'0'}
			b[0] += byte(level)
			p.Emit(string(b))
			p.Emit(s)
			p.Emit(k)
			p.Dec()
		}
		prevIndent = indent
	}

	p.Dec()
}

// List processes (eventually) nested lists.
// One call to this function processes all consecutive lines starting with '-',
// even if not in the first position. Indentation levels need to be taken into
// account.
//
// Output format:
//
// !ul
//
//	!l1
//	  "item 1"
//	  item1
//
// TODO recursive parsing of lists (nested lists can be ul or ol)
// TODO detect HR (---)
// TODO include definition lists (- text :: text) <-- not std markdown
func nlist(p *parser.Parser) {

	level := 1
	prevIndent := 0

	p.Emit("!ol")
	p.Inc()

	for {
		ix := p.Ix
		indent, _ := p.Space()
		if c, _ := p.Byte(); c != '+' {
			// end of list
			p.Ix = ix
			break
		}

		if indent > prevIndent {
			level++
		} else if indent < prevIndent {
			level--
		}

		// Read the text of the item, with possibly a key
		s := p.Line()
		k, s := getKey(s)

		if s != "" {

			p.Emit("!li")
			p.Inc()
			b := []byte{'0'}
			b[0] += byte(level)
			p.Emit(string(b))
			p.Emit(s)
			p.Emit(k)
			p.Dec()
		}
		prevIndent = indent
	}

	p.Dec()
}

// CSV Table
//
// - If ||, first column is key
// - If separation line is present, first row is key
func tableCsv(p *parser.Parser, hmode string) {

	header := true

	p.Emit("!tb")
	p.Inc()

	for {
		line := strings.TrimSpace(p.Line())

		if line == "" {
			break
		}

		if line[0] == '#' {
			continue
		}

		ss := csv.Split(line)

		// Header

		if header {

			p.Emit("!tr")
			p.Inc()
			for _, s := range ss {
				k, v := getKey(s)
				p.Emit(v)
				p.Inc()
				p.Emit(k)
				p.Dec()
				// p.Emit(strings.TrimSpace(s))
			}
			p.Dec()
			header = false
			continue
		}

		// Data line

		p.Emit("!tr")
		p.Inc()
		for _, s := range ss {
			p.Emit(strings.TrimSpace(s))
		}
		p.Dec()
	}

	switch hmode {
	case "hrow", "", "h":
		p.Emit("!hrow")
	case "hcol", "v":
		p.Emit("!hcol")
	case "hboth", "hv":
		p.Emit("!hrow")
		p.Emit("!hcol")
	}

	p.Dec()
}

// Table
//
// - If ||, first column is key
// - If separation line is present, first row is key
func table(p *parser.Parser) {

	p.Emit("!tb")
	p.Inc()

	tableV := false
	tableH := false
	doNotNormalize := false

	var header []string

	row := 0

	for {
		if p.PeekByte() != '|' {
			break
		}
		p.Byte()
		s := p.Line()
		if s == "" {
			continue
		}

		if s[0] == '|' {
			tableV = true
			s = s[1:]
		}
		if strings.HasPrefix(s, "---") {
			tableH = true
			continue
		}

		s = strings.TrimSpace(s)

		// Clean the last | because of strings.Split behavior
		if len(s) > 0 && s[len(s)-1] == '|' {
			s = s[:len(s)-1]
		}

		ff := strings.Split(s, "|")
		for i, f := range ff {
			ff[i] = strings.TrimSpace(f)
		}

		switch row {
		case 0:
			header = ff
		case 1:
			p.Emit("!tr")
			p.Inc()
			for _, f := range header {
				if tableV || tableH {
					k, s := getKey(f)
					p.Emit(s)
					p.Inc()
					p.Emit(k)
					p.Dec()
				} else {
					p.Emit(f)
				}
			}
			p.Dec()

			fallthrough
		default:
			p.Emit("!tr")
			p.Inc()
			for i, f := range ff {
				if i == 0 && tableV {
					if f == "" {
						f = "_"
					} else if f[0] == '_' {
						doNotNormalize = true
						f = f[1:]
					}

					// TODO do not emit key if == text: adapt data.go
					if doNotNormalize {
						p.Emit(f)
						p.Inc()
						p.Emit(f)
						p.Dec()
					} else {
						k, s := getKey(f)
						p.Emit(s)
						p.Inc()
						p.Emit(k)
						p.Dec()
					}
				} else {
					p.Emit(f)
				}
			}
			p.Dec()
		}

		row++
	}
	if tableV {
		p.Emit("!hcol")
	}
	if tableH {
		p.Emit("!hrow")
	}
	p.Dec()
}

// Code processes ```[lang] entries
// Read all lines until a line that starts with '`'
func code(p *parser.Parser) {

	s := p.Line()
	// Any syntax specified?
	for {
		if s != "" && s[0] == '`' {
			s = s[1:]
		} else {
			break
		}
	}
	s = strings.TrimSpace(s)
	if s == "" {
		s = "code"
	}

	p.Emit("!pre")
	p.Inc()
	p.Emit(s)

	for {

		s = p.Line()
		if s == "" || s[0] == '`' {
			break
		}

		p.Emit(s)
	}
	p.Dec()
}

// Read a paragraph (all characters until a newline followed by a special
// character, or an empty line).
func paragraph(p *parser.Parser, head, pre string) {

	var b []byte
	start := true

	for {
		c, _ := p.Byte()
		if ogdl.IsSpaceChar(c) && start {

		}
		if ogdl.IsEndChar(c) {
			break
		}

		if ogdl.IsBreakChar(c) {
			c, _ = p.Byte()
			if isDocSpecial(rune(c)) {
				p.UnreadByte()
				break
			}
			if ogdl.IsBreakChar(c) {
				break
			}
			b = append(b, '\n')
		}
		b = append(b, byte(c))
	}

	if len(b) == 0 {
		return
	}

	lv := p.Level()

	if len(head) != 0 {
		p.Emit(head)
		p.Inc()
	}

	p.Emit(pre + string(b))

	p.SetLevel(lv)
}

func isDocSpecial(c rune) bool {
	if c == '#' || c == '!' || c == '.' || c == '>' || c == '-' || c == '`' || c == '|' {
		return true
	}
	return false
}

// Data reads all lines as OGDL until } is found
func data(p *parser.Parser) {

	// discard first line
	p.Line()

	p.Emit("!g")
	p.Inc()

	var sb strings.Builder

	for {
		s := p.Line()
		sb.WriteString(s)

		if p.PeekByte() == '}' || p.End() {
			p.Line()
			break
		}
	}

	p.Emit(sb.String())
	p.Dec()
}

// TODO: put this into the ogdl package (change that package to parser.Parser)
func ogdlFlow(p *ogdl.Parser) bool {

	anything := false

	lv := p.Handler().Level()

	for {
		if p.End() {
			break
		}

		p.WhiteSpace()

		c, _ := p.Byte()
		if c == ')' {
			break
		}
		if c == ',' {
			p.Handler().SetLevel(lv)
			continue
		}
		p.UnreadByte()

		b, ok, _ := p.Quoted(0)
		if !ok {
			b, ok = p.StringStop([]byte("),"))
		}

		if ok {
			p.Handler().Add(b)
			anything = true
			p.Handler().Inc()
			continue
		}
	}

	p.Handler().SetLevel(lv)
	return anything
}
