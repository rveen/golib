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

	if len(s) < 3 {
		return ""
	}

	r, _, _ := transform.String(tr, s)

	var ru []rune

	start := false
	for _, c := range r {
		if !start && unicode.In(c, unicode.Punct, unicode.Space) {
			continue
		}
		start = true
		ru = append(ru, c)
	}

	n := 0
	for i := len(ru) - 1; i >= 0; i-- {
		if unicode.In(ru[i], unicode.Punct, unicode.Space) {
			n++
		} else {
			break
		}
	}
	ru = ru[0 : len(ru)-n]

	r = string(ru)

	if len(r) < 3 || len(r) > 32 {
		return ""
	}

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
