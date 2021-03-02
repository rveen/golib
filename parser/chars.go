package parser

import (
	"bytes"
	"unicode"
	"unicode/utf8"
)

// PeekByte returns the next byte witohut consuming it
func (p *Parser) PeekByte() byte {
	if p.Ix >= len(p.Buf) || p.Ix < 0 {
		return 0
	}
	return p.Buf[p.Ix]
}

// PeekRune returns the next rune witohut consuming it
func (p *Parser) PeekRune() (rune, bool) {
	r, ok := p.Rune()

	if !ok {
		return 0, false
	}

	return r, p.UnreadRune(r)
}

// Byte reads and returns a single byte.
// If no byte is available, returns 0 and an error.
func (p *Parser) Byte() (byte, bool) {

	if p.Ix >= len(p.Buf) {
		return 0, false
	}

	p.Ix++
	return p.Buf[p.Ix-1], true
}

// UnreadByte unreads the last byte. It can unread all buffered bytes.
func (p *Parser) UnreadByte() bool {
	if p.Ix == 0 {
		return false
	}
	p.Ix--
	return true
}

// Rune reads a single UTF-8 encoded Unicode character and returns the
// rune. If the encoded rune is invalid, it consumes one byte
// and returns unicode.ReplacementChar (U+FFFD) with a size of 1.
func (p *Parser) Rune() (rune, bool) {

	if p.Ix >= len(p.Buf) {
		return 0, false
	}

	r, size := rune(p.Buf[p.Ix]), 1
	if r >= utf8.RuneSelf {
		r, size = utf8.DecodeRune(p.Buf[p.Ix:])
	}
	p.Ix += size
	return r, true
}

// UnreadRune unreads the last rune (should be supplied)
func (p *Parser) UnreadRune(r rune) bool {

	s := string(r)
	n := len(s)

	if p.Ix-n < -1 {
		return false
	}
	p.Ix -= n

	return true
}

// End returns true if the end of stream has been reached.
func (p *Parser) End() bool {
	return p.Ix >= len(p.Buf)
}

// IsTextChar returns true for all integers > 32 and
// are not OGDL separators (parenthesis and comma)
func isTextChar(c byte) bool {
	return c > 32
}

// IsEndChar returns true for all integers < 32 that are not newline,
// carriage return or tab.
func isEndChar(c byte) bool {
	return c < 32 && c != '\t' && c != '\n' && c != '\r'
}

// IsEndRune returns true for all integers < 32 that are not newline,
// carriage return or tab.
func isEndRune(c rune) bool {
	return c < 32 && c != '\t' && c != '\n' && c != '\r'
}

// IsBreakChar returns true for 10 and 13 (newline and carriage return)
func isBreakChar(c byte) bool {
	return c == 10 || c == 13
}

// IsSpaceChar returns true for space and tab
func isSpaceChar(c byte) bool {
	return c == 32 || c == 9
}

// IsTemplateTextChar returns true for all not END chars and not $
func isTemplateTextChar(c byte) bool {
	return !isEndChar(c) && c != '$'
}

// IsOperatorChar returns true for all operator characters used in OGDL
// expressions (those parsed by NewExpression).
func isOperatorChar(c byte) bool {
	if c < 0 {
		return false
	}
	return bytes.IndexByte([]byte("+-*/%&|!<>=~^"), c) != -1
}

// ---- Following functions are the only ones that depend on Unicode --------

// IsLetter returns true if the given character is a letter, as per Unicode.
func isLetter(c rune) bool {
	return unicode.IsLetter(c) || c == '_'
}

// IsDigit returns true if the given character a numeric digit, as per Unicode.
func isDigit(c rune) bool {
	return unicode.IsDigit(c)
}

// IsTokenChar returns true for letters, digits and _ (as per Unicode).
func isUnicodeTokenChar(c rune) bool {
	return unicode.IsLetter(c) || unicode.IsDigit(c) || c == '_'
}
