package document

import (
	"github.com/rveen/ogdl"
)

// Markdown entities:
//
// Block:
// - paragraph
// - block quote
// - header
// - code block
// - table
// - list (numbered or not)
// - definition list
// - input, form (->action->output)
// - extension
// - math
// - graph, chart, drawing (SVG)
//
// Inline:
// - image
// - link
// - style
//
// Commands:
// - \nh
//
// Extensions:
// - {ogdl

// Document parses a Markdown document (an array of Unicode characters), into a
// tree (a Graph object).
func Document(p *ogdl.Parser) (*ogdl.Graph, error) {

	var err error

	for {
		r, err := DocBlock(p)
		if !r || err != nil {
			break
		}
	}

	g := p.Graph()

	// Handle inlines
	for _, n := range g.Out {

		switch n.ThisString() {

		case "!p", "!q", "!h1", "!h2", "!h3", "!h4", "!h5", "!h6":
			DocInline(n)
		}
	}

	g.This = "!doc"
	return g, err
}

func DocInline(g *ogdl.Graph) {

	if g == nil || g.Len() == 0 {
		return
	}

	p := ogdl.NewBytesParser(g.Bytes())
	g.Clear()

	var tmp []byte

	for {
		c, _ := p.Byte()

		if ogdl.IsBreakChar(c) || ogdl.IsEndChar(c) {
			break
		}

		if c == '\\' {
			p.UnreadByte()
			b := DocEscape(p)
			if b {
				if len(tmp) > 0 {
					g.Add(string(tmp))
					tmp = nil
				}
				g.AddNodes(p.Graph())

			} else {
				p.Byte()
				tmp = append(tmp, c)
				g.Add(string(tmp))
				tmp = nil
			}
		} else {
			tmp = append(tmp, c)
		}
	}

	if len(tmp) > 0 {
		g.Add(string(tmp))
	}
}

func DocBlock(p *ogdl.Parser) (bool, error) {

	c, _ := p.Byte()

	// if not a special character, just text
	switch c {
	case 0:
		return false, nil
	case '!':
	case '#':
		DocHeader(p)
	case '.':
		DocCommand(p)
	case '>':
		DocQuote(p)
	case '-':
		DocList(p)
	default:
		p.UnreadByte()
		DocText(p, "!p", "")
	}
	return true, nil
}

// Read until the end of line
func DocHeader(p *ogdl.Parser) {

	// Read number of !
	n := 1
	var c rune
	for {
		c, _ = p.Rune()
		if c != '!' && c != '#' {
			break
		}
		n++
	}
	if c != ' ' {
		// This is not a header but text
		DocText(p, "!p", "!"+string(c))
		return
	}

	// The rest of the line is the header
	var b []byte
	for {
		c, _ = p.Rune()
		if c == '\n' || c == '\r' || ogdl.IsEndRune(c) {
			break
		}
		b = append(b, byte(c))
	}
	p.Handler().Add("!h" + string('0'+n))
	if len(b) > 0 {
		p.Handler().Inc()
		p.Handler().Add(string(b))
		p.Handler().Dec()
	}
}

func DocQuote(p *ogdl.Parser) {
	DocText(p, "!q", "")
}

func DocCommand(p *ogdl.Parser) {
}

func DocList(p *ogdl.Parser) {
}

// Read a paragraph (all characters until a newline followed by a special
// character, or an empty line).
func DocText(p *ogdl.Parser, head, pre string) {

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

	lv := p.Handler().Level()

	if len(head) != 0 {
		p.Handler().Add(head)
		p.Handler().Inc()
	}

	p.Handler().Add(pre + string(b))

	p.Handler().SetLevel(lv)

}

func isDocSpecial(c rune) bool {
	if c == '#' || c == '!' || c == '.' || c == '>' || c == '-' {
		return true
	}
	return false
}
