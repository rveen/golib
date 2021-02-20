package document

import (
	"regexp"
	"strings"

	"github.com/rveen/golib/parser"
	"github.com/rveen/ogdl"
)

var (
	anchor = regexp.MustCompile(`{#\w+}`)
	link   = regexp.MustCompile(`\[(.+)\]\((.+)\)`)

	// Not complete: * should not be followed by space
	bold   = regexp.MustCompile(`\*\*(.+)\*\*`)
	italic = regexp.MustCompile(`\*(.+)\*`)
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

func Block(p *parser.Parser) bool {

	c := p.PeekByte()

	switch c {
	case 0:
		return false
	case '#':
		Header(p)
	case '.':
		Command(p)
	case '>':
		Quote(p)
	case '-':
		List(p)
	case '|':
		Table(p)
	case '`':
		Code(p)
	case '{':
		Data(p)
	default:
		Text(p, "!p", "")
	}

	return true
}

func InLine(s string) string {
	s = link.ReplaceAllString(s, "<a href=\"$2\">$1</a>")
	s = bold.ReplaceAllString(s, "<b>$1</b>")
	s = italic.ReplaceAllString(s, "<em>$1</em>")
	return s
}

// Header processes lines containing a header
//
// Output format:
//
//   !h
//     1
//     "Header text"
//     headerText
//
// The second subnode is the normalized string to be used in the data representation
//
func Header(p *parser.Parser) {

	// Read number of !
	n := 0
	var c byte
	var ok bool

	for {
		c, ok = p.Byte()
		if !ok {
			return
		}
		if c != '#' {
			break
		}
		n++
	}
	if c != ' ' {
		// This is not a header but text
		Text(p, "!p", "!"+string(c))
		return
	}

	// The rest of the line is the header
	b := []byte{'0'}
	b[0] += byte(n)
	p.Emit("!h")
	p.Inc()
	p.Emit(string(b))
	p.Dec()

	p.Space()
	s := p.Line()

	// Check for anchor
	var key string
	key, s = getKey(s)

	if s != "" {
		p.Inc()
		p.Emit(s)
		p.Emit(key)
		p.Dec()
	}
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

func Quote(p *parser.Parser) {
	Text(p, "!q", "")
}

func Command(p *parser.Parser) {
	p.Line()
}

// List processes (eventually) nested lists.
// One call to this function processes all consecutive lines starting with '-',
// even if not in the first position. Indentation levels need to be taken into
// account.
//
// Output format:
//
// !ul
//   !l1
//     "item 1"
//     item1
//
// TODO recursive parsing of lists (nested lists can be ul or ol)
// TODO detect HR (---)
// TODO include definition lists (- text :: text) <-- not std markdown
func List(p *parser.Parser) {

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

// Table
//
// - If ||, first column is key
// - If separation line is present, first row is key
//
func Table(p *parser.Parser) {

	p.Emit("!tb")
	p.Inc()

	tableV := false
	tableH := false

	var header []string
	n := 0

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

		// Clean the last | because of strings.Split behavior
		if s[len(s)-1] == '|' {
			s = s[:len(s)-1]
		}

		ff := strings.Split(s, "|")
		for i, f := range ff {
			ff[i] = strings.TrimSpace(f)
		}

		switch n {
		case 0:
			header = ff
		case 1:
			p.Emit("!tr")
			p.Inc()
			for i, f := range header {
				if i == 0 && tableV {
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
		}

		n++
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
func Code(p *parser.Parser) {

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
func Text(p *parser.Parser, head, pre string) {

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
func Data(p *parser.Parser) {

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
