package fn

// fromXML is gxml.FromXML from github.com/rveen/ogdl v1 (io/gxml), on ogdl v2
// nodes: an XML document as a tree, with attributes as "@name" children.

import (
	"bytes"
	"encoding/xml"
	"strings"
	"unicode"

	"github.com/rveen/ogdl/v2"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

func fromXML(b []byte) *ogdl.Node {

	decoder := xml.NewDecoder(bytes.NewReader(b))

	g := ogdl.New(nil)
	var key string
	level := -1

	att := true

	var stack []*ogdl.Node
	stack = append(stack, g)

	tr := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)

	for {
		// Read tokens from the XML document in a stream.
		t, _ := decoder.Token()
		if t == nil {
			break
		}
		// Inspect the type of the token just read.
		switch se := t.(type) {

		case xml.StartElement:
			level++

			key = se.Name.Local
			// No accents in key
			key, _, _ = transform.String(tr, key)
			// - -> _
			key = strings.Replace(key, "-", "_", -1)

			n := stack[len(stack)-1].Add(key)
			// push
			stack = append(stack, n)
			if att {
				for _, at := range se.Attr {
					n.Add("@" + at.Name.Local).Add(at.Value)
				}
			}

		case xml.CharData:

			val := strings.TrimSpace(string(se))
			if len(val) > 0 {
				stack[len(stack)-1].Add(val)
			}

		case xml.EndElement:
			level--
			// pop

			stack = stack[:len(stack)-1]

		}
	}

	return g
}
