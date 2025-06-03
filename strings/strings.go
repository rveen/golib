package strings

import (
	"strings"
)

type Strings struct{}

func New() interface{} {
	return &Strings{}
}

func (str Strings) HasSuffix(a, b string) bool {
	println("Strings.HasSuffix", a, b)
	return strings.HasSuffix(a, b)
}

func (str Strings) HasPrefix(a, b string) bool {
	return strings.HasPrefix(a, b)
}

func (str Strings) Substring(s string, a, b int64) string {
	println("Strings.Substring", s, a, b)
	if b < 0 {
		b = int64(len(s)) + b
	}
	println("Strings.Substring", s, a, b, len(s))
	return s[a:b]
}
