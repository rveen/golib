package parser

// String is a concatenation of characters that are > 0x20
func (p *Parser) String() string {

	var buf []byte

	for {
		if p.Ix >= len(p.Buf) {
			break
		}

		c := p.Buf[p.Ix]
		if !isTextChar(c) {
			break
		}

		p.Ix++
		buf = append(buf, c)
	}

	return string(buf)
}

// Break (= newline) is NL, CR or CR+NL or EoS
func (p *Parser) Break() bool {

	c, ok := p.Byte()

	if !ok {
		return true
	}

	ret := false

	if c == '\r' {
		c, ok = p.Byte()
		if !ok {
			return true
		}
		ret = true
	}

	if c == '\n' {
		ret = true
	} else {
		p.UnreadByte()
	}

	return ret
}

// WhiteSpace is equivalent to Space | Break. It consumes all white space,
// whether spaces, tabs or newlines
func (p *Parser) WhiteSpace() bool {

	any := false

	for {
		if p.Ix >= len(p.Buf) {
			break
		}
		c := p.Buf[p.Ix]
		if c != 13 && c != 10 && c != 9 && c != 32 {
			break
		}
		any = true
		p.Ix++
	}

	return any
}

// Token reads from the Parser input stream and returns
// a token, if any.
func (p *Parser) Token(isTokenChar func(rune) bool) string {

	var buf []rune

	for {
		c, ok := p.Rune()
		if !ok {
			break
		}
		if !isTokenChar(c) {
			p.UnreadRune(c)
			break
		}
		buf = append(buf, c)
	}

	return string(buf)
}

func (p *Parser) TokenList(isTokenChar func(rune) bool) []string {

	var ss []string
	for {
		s := p.Token(isTokenChar)
		if s == "" {
			break
		}

		ss = append(ss, s)

		p.Space()
		c, _ := p.Byte()
		if c != ',' {
			p.UnreadByte()
			break
		}
		p.Space()
	}
	return ss
}

func (p *Parser) TokenListEv(isTokenChar func(rune) bool) bool {

	ok := false

	for {
		s := p.Token(isTokenChar)
		if s == "" {
			break
		}

		p.Emit(s)
		ok = true

		p.Space()
		c, _ := p.Byte()
		if c != ',' {
			p.UnreadByte()
			break
		}
		p.Space()
	}

	return ok
}

// Space is (0x20|0x09)+. It return the number of spaces found (whether
// tabs or spaces), and a byte than can have the values 0, ' ' and '\t'
// indicating mixed, all spaces or all tabs
func (p *Parser) Space() (int, byte) {

	spaces := 0
	tabs := 0

	for {
		c, ok := p.Byte()
		if !ok {
			break
		}
		if c != '\t' && c != ' ' {
			p.UnreadByte()
			break
		}

		if c == ' ' {
			spaces++
		} else {
			tabs++
		}
	}

	var r byte
	if tabs == 0 {
		r = ' '
	} else if spaces == 0 {
		r = '\t'
	}

	return spaces + tabs, r
}

// Number returns true if it finds a number at the current
// parser position. It returns also the number found.
// TODO recognize exp notation ?
func (p *Parser) Number() (string, bool) {

	var buf []byte
	var sign byte
	point := false

	c := p.PeekByte()
	if c == '-' || c == '+' {
		sign = c
		p.Byte()
	}

	for {
		c, _ := p.Byte()
		if !isDigit(rune(c)) {
			if !point && c == '.' {
				point = true
				buf = append(buf, c)
				continue
			}
			break
		}
		buf = append(buf, c)
	}

	p.UnreadByte()
	if sign == '-' {
		return "-" + string(buf), len(buf) > 0
	}
	return string(buf), len(buf) > 0
}

// Operator returns the operator at the current parser position, if any;
// an empty string if not.
func (p *Parser) Operator() string {

	var buf []byte

	for {
		c, ok := p.Byte()
		if !ok {
			break
		}
		if !isOperatorChar(c) {
			p.UnreadByte()
			break
		}
		buf = append(buf, c)
	}

	return string(buf)
}

// Quoted string. Can have newlines in it. It returns the string and a possible error.
func (p *Parser) Quoted(ind int) string {

	ix := p.Ix

	c1, ok := p.Byte()
	if !ok || (c1 != '"' && c1 != '\'' && c1 != '`') {
		p.Ix = ix
		return ""
	}

	var buf []byte

	for {
		c, ok := p.Byte()
		if !ok {
			p.Ix = ix
			return ""
		}

		if c == c1 {
			break
		}

		buf = append(buf, c)

		// in indentation is specified, remove that amount, iff uniform space is
		// found.
		if c == 10 && ind != 0 {
			n, u := p.Space()
			if u != 0 {
				for ; n-ind > 0; n-- {
					buf = append(buf, u)
				}
			}
		}
	}

	return string(buf)
}

func (p *Parser) QToken(isTokenChar func(rune) bool) string {

	s := p.Quoted(0)
	if s == "" {
		s = p.Token(isTokenChar)
	}

	return s
}

func (p *Parser) Line() string {

	var buf []byte

	for {
		if p.Ix >= len(p.Buf) {
			break
		}

		c := p.Buf[p.Ix]
		p.Ix++
		if c == 10 {
			break
		}
		buf = append(buf, c)
	}

	return string(buf)
}
