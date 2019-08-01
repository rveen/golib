package document

import (
	"github.com/rveen/ogdl"
)

// DocEscape parsers inline escape sequences to OGDL
//
// Formats:
//    \token
//    \token()
//    \( )
func DocEscape(p *ogdl.Parser) bool {
	c, _ := p.Rune()

	if c != '\\' {
		p.UnreadRune()
		return false
	}

	c, _ = p.Rune()
	if !ogdl.IsLetter(c) && c != '(' {

		p.UnreadRune()
		p.UnreadRune()
		return false
	}

	lv := p.Handler().Level()

	p.Handler().Add("!esc")
	p.Handler().Inc()

	if ogdl.IsLetter(c) {
		p.UnreadByte()
		s, b := p.Token8()

		if !b {
			return false
		}

		p.Handler().Add(s)
		p.Handler().Inc()

		c, _ = p.Rune()
		if c != '(' {
			p.Handler().SetLevel(lv)
			return true
		}
	}

	b := OgdlFlow(p)
	p.Handler().SetLevel(lv)
	return b
}

func OgdlFlow(p *ogdl.Parser) bool {

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
