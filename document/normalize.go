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

	//return toLowerCamel(r)
	return toKebab(r)
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
// Source: https://github.com/iancoleman/strcase/
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

// toLowerCamel converts a string to lowerCamelCase
func toLowerCamel(s string) string {
	return toCamelInitCase(s, false)
}

// ToKebab converts a string to kebab-case
func toKebab(s string) string {
	return toDelimited(s, '_')
}

// ToDelimited converts a string to delimited.snake.case
// (in this case `delimiter = '.'`)
func toDelimited(s string, delimiter uint8) string {
	return ToScreamingDelimited(s, delimiter, "", false)
}

// ToScreamingDelimited converts a string to SCREAMING.DELIMITED.SNAKE.CASE
// (in this case `delimiter = '.'; screaming = true`)
// or delimited.snake.case
// (in this case `delimiter = '.'; screaming = false`)
func ToScreamingDelimited(s string, delimiter uint8, ignore string, screaming bool) string {
	s = strings.TrimSpace(s)
	n := strings.Builder{}
	n.Grow(len(s) + 2) // nominal 2 bytes of extra space for inserted delimiters
	for i, v := range []byte(s) {
		vIsCap := v >= 'A' && v <= 'Z'
		vIsLow := v >= 'a' && v <= 'z'
		if vIsLow && screaming {
			v += 'A'
			v -= 'a'
		} else if vIsCap && !screaming {
			v += 'a'
			v -= 'A'
		}

		// treat acronyms as words, eg for JSONData -> JSON is a whole word
		if i+1 < len(s) {
			next := s[i+1]
			vIsNum := v >= '0' && v <= '9'
			nextIsCap := next >= 'A' && next <= 'Z'
			nextIsLow := next >= 'a' && next <= 'z'
			nextIsNum := next >= '0' && next <= '9'
			// add underscore if next letter case type is changed
			if (vIsCap && (nextIsLow || nextIsNum)) || (vIsLow && (nextIsCap || nextIsNum)) || (vIsNum && (nextIsCap || nextIsLow)) {
				prevIgnore := ignore != "" && i > 0 && strings.ContainsAny(string(s[i-1]), ignore)
				if !prevIgnore {
					if vIsCap && nextIsLow {
						if prevIsCap := i > 0 && s[i-1] >= 'A' && s[i-1] <= 'Z'; prevIsCap {
							n.WriteByte(delimiter)
						}
					}
					n.WriteByte(v)
					if vIsLow || vIsNum || nextIsNum {
						n.WriteByte(delimiter)
					}
					continue
				}
			}
		}

		if (v == ' ' || v == '_' || v == '-' || v == '.') && !strings.ContainsAny(string(v), ignore) {
			// replace space/underscore/hyphen/dot with delimiter
			n.WriteByte(delimiter)
		} else {
			n.WriteByte(v)
		}
	}

	return n.String()
}
