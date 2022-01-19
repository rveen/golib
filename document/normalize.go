package document

import (
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

var tr = transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)

func Normalize(s string) string {

	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}

	r, _, _ := transform.String(tr, s)

	return toLowerCamel(r)
}

func CleanToLower(s string) string {

	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}

	tr := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	r, _, _ := transform.String(tr, s)

	return strings.ToLower(r)
}

func valid(r rune) rune {
	switch r {
	case '\t':
		fallthrough
	case ' ':
		return '_'
	}
	return r
}

// Converts a string to CamelCase
// Source: https://github.com/iancoleman/strcase/blob/master/camel.go
func toCamelInitCase(s string, initCase bool) string {

	n := strings.Builder{}
	n.Grow(len(s))
	capNext := initCase
	for i, v := range []byte(s) {
		vIsCap := v >= 'A' && v <= 'Z'
		vIsLow := v >= 'a' && v <= 'z'
		if capNext {
			if vIsLow {
				v += 'A'
				v -= 'a'
			}
		} else if i == 0 {
			if vIsCap {
				v += 'a'
				v -= 'A'
			}
		}
		if vIsCap || vIsLow {
			n.WriteByte(v)
			capNext = false
		} else if vIsNum := v >= '0' && v <= '9'; vIsNum {
			n.WriteByte(v)
			capNext = true
		} else {
			capNext = v == '_' || v == '\t' || v == ' ' || v == '-' || v == '.'
		}
	}
	return n.String()
}

// ToLowerCamel converts a string to lowerCamelCase
func toLowerCamel(s string) string {
	return toCamelInitCase(s, false)
}
