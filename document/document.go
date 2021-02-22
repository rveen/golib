// A markdown processor. Converts markdown to OGDL, that may be rendered later
// as HTML or Latex/Context
package document

import (
	// "bytes"
	"strings"

	"github.com/rveen/golib/parser"
	"github.com/rveen/golib/parser/eventhandler"
	"github.com/rveen/ogdl"
)

type Document struct {
	stream *eventhandler.SimpleEventHandler
	ix     int
}

func New(s string) (*Document, error) {

	doc := &Document{}

	doc.stream = eventhandler.New()
	p := parser.New([]byte(s), doc.stream)

	for Block(p) {
	}

	return doc, nil
}

func (doc *Document) Graph() *ogdl.Graph {
	return doc.stream.Graph()
}

func (doc *Document) Html() string {

	var sb strings.Builder

	doc.ix = 0

	for {
		s, n := doc.stream.Item(doc.ix)

		if n < 0 {
			break
		}

		doc.ix++

		switch s {
		case "!pre":
			doc.codeToHtml(&sb)
		case "!p":
			doc.textToHtml(&sb)
		case "!h":
			doc.headerToHtml(&sb)
		case "!ul":
			doc.listToHtml(&sb, 1)
		case "!tb":
			doc.tableToHtml(&sb)
		}
	}

	return sb.String()
}

func (doc *Document) Data() *ogdl.Graph {

	eh := eventhandler.New()
	doc.ix = 0

	for {
		s, n := doc.stream.Item(doc.ix)

		if n < 0 {
			break
		}

		doc.ix++

		switch s {
		case "!h":
			doc.headerToData(eh)
		case "!p":
			doc.textToData(eh)
		case "!tb":
			doc.tableToData(eh)
		}
	}

	return eh.Graph()
}
