package jupyter

// Format: https://nbformat.readthedocs.io/en/latest/

import (
	"bytes"
	"encoding/json"
	"log"
	"math"

	"github.com/miekg/mmark"

	"github.com/rveen/ogdl"
)

func FromJupyter(buf []byte) (*ogdl.Graph, error) {

	var v interface{}

	// Use Decoder, since we want to treat integers as integers.
	dec := json.NewDecoder(bytes.NewReader(buf))
	dec.UseNumber()

	// We expect here only one map or list
	err := dec.Decode(&v)

	return toGraph(v), err
}

func ToHTML(g *ogdl.Graph) ([]byte, error) {

	var buf bytes.Buffer

	cells := g.Node("cells")

	if cells == nil {
		log.Println("no cells!")
		return nil, nil
	}

	for _, c := range cells.Out[0].Out {
		typ := c.Node("cell_type")
		switch typ.String() {
		case "code":

			// Source code sections
			ss := c.Node("source")
			if ss == nil || len(ss.Out) == 0 {
				break
			}
			buf.WriteString("<pre class='code'>\n")
			for _, s := range ss.Out[0].Out {
				buf.WriteString(s.ThisString())
			}
			buf.WriteString("</pre>\n")

			// Outputs
			oo := c.Node("outputs")
			if oo == nil || oo.Len() == 0 || oo.Out[0].Len() == 0 {
				break
			}

			d := oo.Out[0].Out[0].Node("data").Out[0]

			for _, e := range d.Out {
				switch e.ThisString() {
				case "image/png":
					buf.WriteString("<img src=\"data:image/png;base64,")
					buf.WriteString(e.String())
					buf.WriteString("\">\n")

				case "text/plain":
					for _, s := range e.Out[0].Out {
						buf.WriteString("<pre class='text'>" + s.ThisString() + "</pre>\n")
					}

				}

			}

		case "markdown":

			ss := c.Node("source")
			if ss == nil || len(ss.Out) == 0 {
				break
			}

			var md bytes.Buffer

			for _, s := range ss.Out[0].Out {
				md.WriteString(s.ThisString())
			}

			buf.Write(xmarkdown(md.String()))

		default:
			buf.WriteString(typ.String())
			buf.WriteRune('\n')
		}
	}

	return buf.Bytes(), nil
}

const extensions int = mmark.EXTENSION_TABLES | mmark.EXTENSION_FENCED_CODE |
	mmark.EXTENSION_AUTOLINK | mmark.EXTENSION_SPACE_HEADERS |
	mmark.EXTENSION_CITATION | mmark.EXTENSION_TITLEBLOCK_TOML |
	mmark.EXTENSION_HEADER_IDS | mmark.EXTENSION_AUTO_HEADER_IDS |
	mmark.EXTENSION_UNIQUE_HEADER_IDS | mmark.EXTENSION_FOOTNOTES |
	mmark.EXTENSION_SHORT_REF | mmark.EXTENSION_INCLUDE | mmark.EXTENSION_PARTS |
	mmark.EXTENSION_ABBREVIATIONS | mmark.EXTENSION_DEFINITION_LISTS

// MDX processes extended markdown
func xmarkdown(s string) []byte {
	htmlFlags := 0
	renderer := mmark.HtmlRenderer(htmlFlags, "", "")
	return mmark.Parse([]byte(s), renderer, extensions).Bytes()
}

func toGraph(v interface{}) *ogdl.Graph {

	var g *ogdl.Graph

	switch v.(type) {

	case []interface{}:
		g = ogdl.New("-")
		for _, i := range v.([]interface{}) {
			n := toGraph(i)
			if n != nil && !(n.This == "-" && n.Len() == 0) {
				g.Add(n)
			}
		}

	case map[string]interface{}:
		g = ogdl.New("{")
		for k, i := range v.(map[string]interface{}) {
			n := toGraph(i)
			if n != nil && !(n.This == "-" && n.Len() == 0) {
				g.Add(k).Add(n)
			} else {
				g.Add(k)
			}
		}

	case json.Number:
		// try first to decode the number as an integer.
		i, err := v.(json.Number).Int64()
		if err != nil {
			f, err := v.(json.Number).Float64()
			if err != nil {
				f = math.NaN()
			}
			g = ogdl.New(f)
		} else {
			g = ogdl.New(i)
		}

	default:
		g = ogdl.New(v)
	}

	return g
}
