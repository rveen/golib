package document

import (
	"fmt"
	"testing"

	"github.com/rveen/ogdl"
)

func Test1(t *testing.T) {
	g, _ := New("hola\ncaracola\n>holas\ncara\n\npa\\b(ra)rafo\n# cap1")

	fmt.Println(g.Show())

	fmt.Println(toHtml(g))
}

func Test2(t *testing.T) {
	s := "\\b(ra) asdf"
	p := ogdl.NewBytesParser([]byte(s))

	DocEscape(p)

	g := p.Graph()

	fmt.Println(g.Show())

}
