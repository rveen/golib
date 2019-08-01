// A markdown processor. Converts markdown to OGDL, that may be rendered later
// as HTML or Latex/Context
package document

import (
	"bytes"

	"github.com/rveen/ogdl"
)

func New(s string) (*ogdl.Graph, error) {
	p := ogdl.NewBytesParser([]byte(s))
	g, _ := Document(p)
	return g, nil
}

func ToHtml(g *ogdl.Graph) string {

	var buf bytes.Buffer

	for _, n := range g.Out {
		switch n.This {
		case "!p":
			renderP(n, &buf)
		case "!q":
			buf.WriteString("<em>")
			buf.WriteString(n.String())
			buf.WriteString("</em>\n")
		case "!h1":
			buf.WriteString("<h1>")
			buf.WriteString(n.String())
			buf.WriteString("</h1>\n")
		case "!h2":
			buf.WriteString("<h2>")
			buf.WriteString(n.String())
			buf.WriteString("</h2>\n")
		}
	}

	return buf.String()
}

func renderP(g *ogdl.Graph, buf *bytes.Buffer) {
	buf.WriteString("<p>")

	for _, n := range g.Out {

		s := n.ThisString()

		switch s {

		case "!esc":
			processEsc(n, buf)

		default:
			buf.WriteString(s)
		}
	}

	buf.WriteString("</p>\n")
}

func processEsc(g *ogdl.Graph, buf *bytes.Buffer) {
	if g == nil || g.Len() == 0 {
		return
	}

	g = g.Out[0]

	f := g.ThisString()

	switch f {

	case "b":
		buf.WriteString("<b>" + g.String() + "</b>")
	}
}
