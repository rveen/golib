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
func docEscape(p *ogdl.Parser) bool {
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

	b := ogdlFlow(p)
	p.Handler().SetLevel(lv)
	return b
}
