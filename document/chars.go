package document

// Character classes, as in ogdl v1's lexer (v2 doesn't export them).

// isEndChar reports whether c is a control character other than tab, newline
// or carriage return (0 marks the end of the input).
func isEndChar(c byte) bool {
	return c < 32 && c != '\t' && c != '\n' && c != '\r'
}

// isBreakChar reports whether c is a newline or carriage return.
func isBreakChar(c byte) bool {
	return c == 10 || c == 13
}

// isSpaceChar reports whether c is a space or tab.
func isSpaceChar(c byte) bool {
	return c == 32 || c == 9
}
